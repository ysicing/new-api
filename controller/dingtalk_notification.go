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
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
			"success": false, "code": "DINGTALK_INVALID_USER", "message": "请选择有效的平台用户",
		})
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
	status, code, message := http.StatusBadGateway, "DINGTALK_SEND_FAILED", "钉钉测试消息发送失败"
	switch {
	case errors.Is(err, service.ErrDingTalkNotConfigured):
		status, code, message = http.StatusConflict, "DINGTALK_NOT_CONFIGURED", "请先保存完整的钉钉应用凭证"
	case errors.Is(err, service.ErrDingTalkGroupNotConfigured):
		status, code, message = http.StatusConflict, "DINGTALK_GROUP_NOT_CONFIGURED", "请先配置公告通知群 openConversationId"
	case errors.Is(err, service.ErrDingTalkInvalidEmail):
		status, code, message = http.StatusBadRequest, "DINGTALK_INVALID_EMAIL", "用户邮箱无效"
	case errors.Is(err, service.ErrDingTalkIdentityMismatch):
		status, code, message = http.StatusConflict, "DINGTALK_IDENTITY_MISMATCH", "钉钉通讯录身份与平台用户不匹配"
	case errors.Is(err, model.ErrDingTalkIdentityConflict):
		status, code, message = http.StatusConflict, "DINGTALK_IDENTITY_CONFLICT", "该钉钉账号已绑定其他平台用户"
	case errors.Is(err, service.ErrDingTalkDirectoryFailed):
		status, code, message = http.StatusBadGateway, "DINGTALK_DIRECTORY_FAILED", "钉钉通讯录查询失败，请检查权限"
	}
	c.AbortWithStatusJSON(status, gin.H{"success": false, "code": code, "message": message})
}
