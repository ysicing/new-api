package controller

import (
	"errors"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

const maxQuotaPoolStatsDays = 366

var errInvalidQuotaPoolStatsRange = errors.New("invalid quota pool statistics range")

type quotaPoolStatsRange struct {
	RangeType      string `json:"range_type"`
	StartTimestamp int64  `json:"start_timestamp"`
	EndTimestamp   int64  `json:"end_timestamp"`
	StartDate      string `json:"start_date"`
	EndDate        string `json:"end_date"`
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
		location = time.Local
	}
	now = now.In(location)
	rangeType := c.DefaultQuery("range_type", "")
	if rangeType == "" {
		rangeType = c.DefaultQuery("period", "week")
	}

	var start, end time.Time
	switch rangeType {
	case "week":
		anchor := now
		if raw := c.Query("anchor"); raw != "" {
			parsed, err := time.ParseInLocation(time.DateOnly, raw, location)
			if err != nil {
				return quotaPoolStatsRange{}, errInvalidQuotaPoolStatsRange
			}
			if parsed.After(now) {
				return quotaPoolStatsRange{}, errInvalidQuotaPoolStatsRange
			}
			anchor = parsed
		}
		weekday := int(anchor.Weekday())
		if weekday == 0 {
			weekday = 7
		}
		start = time.Date(anchor.Year(), anchor.Month(), anchor.Day()-weekday+1, 0, 0, 0, 0, location)
		end = start.AddDate(0, 0, 7).Add(-time.Second)
	case "month":
		anchor := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, location)
		if raw := c.Query("anchor"); raw != "" {
			parsed, err := time.ParseInLocation("2006-01", raw, location)
			if err != nil {
				return quotaPoolStatsRange{}, errInvalidQuotaPoolStatsRange
			}
			if parsed.After(now) {
				return quotaPoolStatsRange{}, errInvalidQuotaPoolStatsRange
			}
			anchor = parsed
		}
		start = time.Date(anchor.Year(), anchor.Month(), 1, 0, 0, 0, 0, location)
		end = start.AddDate(0, 1, 0).Add(-time.Second)
	case "custom":
		var err error
		start, err = time.ParseInLocation(time.DateOnly, c.Query("start_date"), location)
		if err != nil {
			return quotaPoolStatsRange{}, errInvalidQuotaPoolStatsRange
		}
		endDate, err := time.ParseInLocation(time.DateOnly, c.Query("end_date"), location)
		if err != nil {
			return quotaPoolStatsRange{}, errInvalidQuotaPoolStatsRange
		}
		today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, location)
		if endDate.After(today) {
			return quotaPoolStatsRange{}, errInvalidQuotaPoolStatsRange
		}
		end = endDate.AddDate(0, 0, 1).Add(-time.Second)
	default:
		return quotaPoolStatsRange{}, errInvalidQuotaPoolStatsRange
	}

	if start.After(now) || end.Before(start) || quotaPoolStatsDayCount(start, end) > maxQuotaPoolStatsDays {
		return quotaPoolStatsRange{}, errInvalidQuotaPoolStatsRange
	}
	if end.After(now) {
		end = now
	}
	return quotaPoolStatsRange{
		RangeType: rangeType, StartTimestamp: start.Unix(), EndTimestamp: end.Unix(),
		StartDate: start.Format(time.DateOnly), EndDate: end.Format(time.DateOnly),
	}, nil
}

func quotaPoolStatsDayCount(start, end time.Time) int {
	endDate := time.Date(end.Year(), end.Month(), end.Day(), 0, 0, 0, 0, end.Location())
	days := 0
	for cursor := time.Date(start.Year(), start.Month(), start.Day(), 0, 0, 0, 0, start.Location()); !cursor.After(endDate); cursor = cursor.AddDate(0, 0, 1) {
		days++
		if days > maxQuotaPoolStatsDays {
			return days
		}
	}
	return days
}
