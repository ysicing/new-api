package controller

import (
	"errors"
	"strconv"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/gin-gonic/gin"
)

const maxQuotaPoolStatsDays = 366

var errInvalidQuotaPoolStatsRange = errors.New("invalid quota pool statistics range")

type quotaPoolStatsRange struct {
	Preset         string `json:"preset"`
	Granularity    string `json:"granularity"`
	StartTimestamp int64  `json:"start_timestamp"`
	EndTimestamp   int64  `json:"end_timestamp"`
}

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

func parseQuotaPoolStatsRange(c *gin.Context, now time.Time) (quotaPoolStatsRange, error) {
	location := now.Location()
	if location == nil {
		location = common.BeijingTimeLocation
	}
	now = now.In(location)
	preset := c.DefaultQuery("preset", "rolling_7d")
	var start, end time.Time
	autoGranularity := "day"
	currentHour := time.Unix(now.Unix()-now.Unix()%3600, 0).In(location)
	switch preset {
	case "rolling_1d", "rolling_7d", "rolling_14d", "rolling_29d":
		days := map[string]int{"rolling_1d": 1, "rolling_7d": 7, "rolling_14d": 14, "rolling_29d": 29}[preset]
		start = currentHour.Add(-time.Duration(days*24-1) * time.Hour)
		end = now
		if days == 1 {
			autoGranularity = "hour"
		}
	case "today":
		start = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, location)
		end = now
		autoGranularity = "hour"
	case "this_week":
		weekday := int(now.Weekday())
		if weekday == 0 {
			weekday = 7
		}
		start = time.Date(now.Year(), now.Month(), now.Day()-weekday+1, 0, 0, 0, 0, location)
		end = now
	case "this_month":
		start = time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, location)
		end = now
	case "custom":
		startTimestamp, startErr := strconv.ParseInt(c.Query("start_timestamp"), 10, 64)
		endTimestamp, endErr := strconv.ParseInt(c.Query("end_timestamp"), 10, 64)
		if startErr != nil || endErr != nil || startTimestamp <= 0 || endTimestamp <= 0 {
			return quotaPoolStatsRange{}, errInvalidQuotaPoolStatsRange
		}
		requestedStart := time.Unix(startTimestamp, 0).In(location)
		requestedEnd := time.Unix(endTimestamp, 0).In(location)
		if requestedStart.After(now) || requestedEnd.After(now) || requestedEnd.Before(requestedStart) || requestedEnd.Sub(requestedStart) > maxQuotaPoolStatsDays*24*time.Hour {
			return quotaPoolStatsRange{}, errInvalidQuotaPoolStatsRange
		}
		start = time.Unix(requestedStart.Unix()-requestedStart.Unix()%3600, 0).In(location)
		end = time.Unix(requestedEnd.Unix()-requestedEnd.Unix()%3600+3600-1, 0).In(location)
		if end.After(now) {
			end = now
		}
		if end.Sub(start) > maxQuotaPoolStatsDays*24*time.Hour {
			return quotaPoolStatsRange{}, errInvalidQuotaPoolStatsRange
		}
		if requestedEnd.Sub(requestedStart) <= 48*time.Hour {
			autoGranularity = "hour"
		}
	default:
		return quotaPoolStatsRange{}, errInvalidQuotaPoolStatsRange
	}
	if start.After(now) || end.After(now) || end.Before(start) {
		return quotaPoolStatsRange{}, errInvalidQuotaPoolStatsRange
	}
	granularity := c.DefaultQuery("granularity", autoGranularity)
	if granularity != "hour" && granularity != "day" && granularity != "week" {
		return quotaPoolStatsRange{}, errInvalidQuotaPoolStatsRange
	}
	return quotaPoolStatsRange{
		Preset: preset, Granularity: granularity, StartTimestamp: start.Unix(), EndTimestamp: end.Unix(),
	}, nil
}
