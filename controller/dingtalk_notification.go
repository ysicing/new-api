package controller

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
)

var sendDingTalkTestMessage = service.SendDingTalkTestMessage
var sendDingTalkAnnouncementGroupTestMessage = service.SendDingTalkAnnouncementGroupTestMessage

// ListDingTalkNotifications 返回后台运维使用的钉钉消息投递记录。
func ListDingTalkNotifications(c *gin.Context) {
	page := common.GetPageQuery(c)
	startTimestamp, _ := strconv.ParseInt(c.Query("start_timestamp"), 10, 64)
	endTimestamp, _ := strconv.ParseInt(c.Query("end_timestamp"), 10, 64)
	items, total, err := model.ListDingTalkNotifications(model.DingTalkNotificationQuery{
		EventType: c.Query("event_type"), Status: c.Query("status"), Keyword: c.Query("keyword"),
		StartTimestamp: startTimestamp, EndTimestamp: endTimestamp,
		StartIdx: page.GetStartIdx(), PageSize: page.GetPageSize(),
	})
	if err != nil {
		common.ApiError(c, err)
		return
	}
	page.SetItems(items)
	page.SetTotal(int(total))
	common.ApiSuccess(c, page)
}

func ListDingTalkTestUsers(c *gin.Context) {
	page := common.GetPageQuery(c)
	items, total, err := model.ListDingTalkTestUsers(
		c.Query("keyword"), page.GetStartIdx(), page.GetPageSize(),
	)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	page.SetItems(items)
	page.SetTotal(int(total))
	common.ApiSuccess(c, page)
}

func SendDingTalkTestMessage(c *gin.Context) {
	var request struct {
		UserId int `json:"user_id"`
	}
	if err := common.DecodeJson(c.Request.Body, &request); err != nil || request.UserId <= 0 {
		common.ApiErrorStatus(c, http.StatusBadRequest, common.NewMessage("Select a valid platform user"), gin.H{"code": "DINGTALK_INVALID_USER"})
		c.Abort()
		return
	}
	result, err := sendDingTalkTestMessage(c.Request.Context(), request.UserId)
	if err != nil {
		writeDingTalkTestError(c, err)
		return
	}
	common.ApiSuccess(c, result)
}

func SendDingTalkAnnouncementGroupTestMessage(c *gin.Context) {
	if err := sendDingTalkAnnouncementGroupTestMessage(c.Request.Context()); err != nil {
		writeDingTalkTestError(c, err)
		return
	}
	common.ApiSuccess(c, nil)
}

func writeDingTalkTestError(c *gin.Context, err error) {
	status, code, message := http.StatusBadGateway, "DINGTALK_SEND_FAILED", "Failed to send DingTalk test message"
	switch {
	case errors.Is(err, service.ErrDingTalkNotConfigured):
		status, code, message = http.StatusConflict, "DINGTALK_NOT_CONFIGURED", "Save complete DingTalk application credentials first"
	case errors.Is(err, service.ErrDingTalkGroupNotConfigured):
		status, code, message = http.StatusConflict, "DINGTALK_GROUP_NOT_CONFIGURED", "Configure the announcement group openConversationId first"
	case errors.Is(err, service.ErrDingTalkInvalidEmail):
		status, code, message = http.StatusBadRequest, "DINGTALK_INVALID_EMAIL", "Invalid user email"
	case errors.Is(err, service.ErrDingTalkIdentityMismatch):
		status, code, message = http.StatusConflict, "DINGTALK_IDENTITY_MISMATCH", "DingTalk directory identity does not match the platform user"
	case errors.Is(err, model.ErrDingTalkIdentityConflict):
		status, code, message = http.StatusConflict, "DINGTALK_IDENTITY_CONFLICT", "This DingTalk account is already linked to another platform user"
	case errors.Is(err, service.ErrDingTalkDirectoryFailed):
		status, code, message = http.StatusBadGateway, "DINGTALK_DIRECTORY_FAILED", "Failed to query the DingTalk directory; check application permissions"
	}
	common.ApiErrorStatus(c, status, common.NewMessage(message), gin.H{"code": code})
	c.Abort()
}
