package controller

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestStatsRangeDefaultsToCurrentWeek(t *testing.T) {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodGet, "/api/quota_pool/1/stats", nil)
	now := time.Date(2026, time.August, 19, 15, 30, 0, 0, time.Local)

	start, end := statsRange(c, now)

	assert.Equal(t, time.Date(2026, time.August, 17, 0, 0, 0, 0, time.Local).Unix(), start)
	assert.Equal(t, now.Unix(), end)
}

func TestStatsRangeSupportsMonthAndExplicitTimestamps(t *testing.T) {
	now := time.Date(2026, time.August, 19, 15, 30, 0, 0, time.Local)
	tests := []struct {
		name      string
		query     string
		wantStart int64
		wantEnd   int64
	}{
		{name: "month", query: "period=month", wantStart: now.AddDate(0, -1, 0).Unix(), wantEnd: now.Unix()},
		{name: "timestamps", query: "start_timestamp=100&end_timestamp=200", wantStart: 100, wantEnd: 200},
		{name: "explicit unbounded timestamps", query: "start_timestamp=0&end_timestamp=0", wantStart: 0, wantEnd: 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			c.Request = httptest.NewRequest(http.MethodGet, "/api/quota_pool/1/stats?"+tt.query, nil)

			start, end := statsRange(c, now)

			assert.Equal(t, tt.wantStart, start)
			assert.Equal(t, tt.wantEnd, end)
		})
	}
}

func TestParseQuotaPoolStatsRangeSupportsPresetsAndAutomaticGranularity(t *testing.T) {
	location := time.FixedZone("UTC+8", 8*60*60)
	now := time.Date(2026, time.August, 19, 15, 30, 0, 0, location)
	tests := []struct {
		name            string
		query           string
		wantPreset      string
		wantGranularity string
		wantStart       time.Time
		wantEnd         time.Time
	}{
		{name: "rolling 1 day", query: "preset=rolling_1d", wantPreset: "rolling_1d", wantGranularity: "hour", wantStart: time.Date(2026, time.August, 18, 16, 0, 0, 0, location), wantEnd: now},
		{name: "rolling 7 days", query: "preset=rolling_7d", wantPreset: "rolling_7d", wantGranularity: "day", wantStart: time.Date(2026, time.August, 12, 16, 0, 0, 0, location), wantEnd: now},
		{name: "rolling 14 days", query: "preset=rolling_14d", wantPreset: "rolling_14d", wantGranularity: "day", wantStart: time.Date(2026, time.August, 5, 16, 0, 0, 0, location), wantEnd: now},
		{name: "rolling 29 days", query: "preset=rolling_29d", wantPreset: "rolling_29d", wantGranularity: "day", wantStart: time.Date(2026, time.July, 21, 16, 0, 0, 0, location), wantEnd: now},
		{name: "today", query: "preset=today", wantPreset: "today", wantGranularity: "hour", wantStart: time.Date(2026, time.August, 19, 0, 0, 0, 0, location), wantEnd: now},
		{name: "this week", query: "preset=this_week", wantPreset: "this_week", wantGranularity: "day", wantStart: time.Date(2026, time.August, 17, 0, 0, 0, 0, location), wantEnd: now},
		{name: "this month", query: "preset=this_month", wantPreset: "this_month", wantGranularity: "day", wantStart: time.Date(2026, time.August, 1, 0, 0, 0, 0, location), wantEnd: now},
		{name: "custom hours", query: "preset=custom&start_timestamp=1786982400&end_timestamp=1787068800", wantPreset: "custom", wantGranularity: "hour", wantStart: time.Unix(1786982400, 0).In(location), wantEnd: time.Unix(1787068800, 0).In(location).Add(time.Hour - time.Second)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			c.Request = httptest.NewRequest(http.MethodGet, "/api/quota_pool/1/stats?"+tt.query, nil)

			got, err := parseQuotaPoolStatsRange(c, now)

			assert.NoError(t, err)
			assert.Equal(t, tt.wantPreset, got.Preset)
			assert.Equal(t, tt.wantGranularity, got.Granularity)
			assert.Equal(t, tt.wantStart.Unix(), got.StartTimestamp)
			assert.Equal(t, tt.wantEnd.Unix(), got.EndTimestamp)
		})
	}
}

func TestParseQuotaPoolStatsRangeRejectsInvalidRanges(t *testing.T) {
	location := time.FixedZone("UTC+8", 8*60*60)
	now := time.Date(2026, time.August, 19, 15, 30, 0, 0, location)
	queries := []string{
		"preset=week",
		"preset=custom",
		"preset=custom&start_timestamp=200&end_timestamp=100",
		"preset=custom&start_timestamp=1&end_timestamp=9999999999",
		"preset=rolling_1d&granularity=month",
	}
	for _, query := range queries {
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		c.Request = httptest.NewRequest(http.MethodGet, "/api/quota_pool/1/stats?"+query, nil)

		_, err := parseQuotaPoolStatsRange(c, now)

		assert.Error(t, err, query)
	}
}

func TestParseQuotaPoolStatsRangeAllowsAtMost366Days(t *testing.T) {
	location := time.FixedZone("UTC+8", 8*60*60)
	now := time.Date(2026, time.August, 19, 15, 30, 0, 0, location)
	request := func(start time.Time) error {
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		path := fmt.Sprintf("/api/quota_pool/1/stats?preset=custom&start_timestamp=%d&end_timestamp=%d", start.Unix(), now.Unix())
		c.Request = httptest.NewRequest(http.MethodGet, path, nil)
		_, err := parseQuotaPoolStatsRange(c, now)
		return err
	}

	currentHour := time.Unix(now.Unix()-now.Unix()%3600, 0).In(location)
	assert.NoError(t, request(currentHour.Add(-366*24*time.Hour+time.Hour)))
	assert.Error(t, request(now.Add(-366*24*time.Hour)))
}

func TestParseQuotaPoolStatsRangePreservesSecondDSTFallbackHour(t *testing.T) {
	location, err := time.LoadLocation("America/New_York")
	assert.NoError(t, err)
	now, err := time.Parse(time.RFC3339, "2026-11-01T01:30:00-05:00")
	assert.NoError(t, err)
	now = now.In(location)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodGet, "/api/quota_pool/1/stats?preset=rolling_1d", nil)

	statsRange, err := parseQuotaPoolStatsRange(c, now)

	assert.NoError(t, err)
	assert.Equal(t, "2026-11-01T01:00:00-05:00", time.Unix(statsRange.EndTimestamp-statsRange.EndTimestamp%3600, 0).In(location).Format(time.RFC3339))
}

func TestLoadQuotaPoolStatsRejectsInvalidRangeWithBadRequest(t *testing.T) {
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(
		http.MethodGet,
		"/api/quota_pool/1/stats?preset=custom&start_timestamp=200&end_timestamp=100",
		nil,
	)

	_, ok := loadQuotaPoolStats(c, 1, time.Date(2026, time.August, 19, 15, 30, 0, 0, time.Local))

	assert.False(t, ok)
	assert.Equal(t, http.StatusBadRequest, recorder.Code)
	assert.Contains(t, recorder.Body.String(), `"code":"QUOTA_POOL_STATS_RANGE_INVALID"`)
}
