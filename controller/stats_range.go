package controller

import (
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

func statsRange(c *gin.Context, now time.Time) (int64, int64) {
	startValue, hasStart := c.GetQuery("start_timestamp")
	endValue, hasEnd := c.GetQuery("end_timestamp")
	start, _ := strconv.ParseInt(startValue, 10, 64)
	end, _ := strconv.ParseInt(endValue, 10, 64)
	if hasStart || hasEnd {
		return start, end
	}
	if c.Query("period") == "month" {
		return now.AddDate(0, -1, 0).Unix(), now.Unix()
	}
	weekday := int(now.Weekday())
	if weekday == 0 {
		weekday = 7
	}
	weekStart := time.Date(now.Year(), now.Month(), now.Day()-weekday+1, 0, 0, 0, 0, now.Location())
	return weekStart.Unix(), now.Unix()
}
