package controller

import (
	"strconv"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
)

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
