package controller

import (
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

func TestParseQuotaPoolStatsRangeSupportsCalendarWeekMonthAndCustom(t *testing.T) {
	location := time.FixedZone("UTC+8", 8*60*60)
	now := time.Date(2026, time.August, 19, 15, 30, 0, 0, location)
	tests := []struct {
		name      string
		query     string
		wantType  string
		wantStart time.Time
		wantEnd   time.Time
	}{
		{
			name: "historical week", query: "range_type=week&anchor=2026-08-12", wantType: "week",
			wantStart: time.Date(2026, time.August, 10, 0, 0, 0, 0, location),
			wantEnd:   time.Date(2026, time.August, 16, 23, 59, 59, 0, location),
		},
		{
			name: "historical month", query: "range_type=month&anchor=2026-02", wantType: "month",
			wantStart: time.Date(2026, time.February, 1, 0, 0, 0, 0, location),
			wantEnd:   time.Date(2026, time.March, 1, 0, 0, 0, 0, location).Add(-time.Second),
		},
		{
			name: "leap year month", query: "range_type=month&anchor=2024-02", wantType: "month",
			wantStart: time.Date(2024, time.February, 1, 0, 0, 0, 0, location),
			wantEnd:   time.Date(2024, time.March, 1, 0, 0, 0, 0, location).Add(-time.Second),
		},
		{
			name: "current week", query: "range_type=week&anchor=2026-08-19", wantType: "week",
			wantStart: time.Date(2026, time.August, 17, 0, 0, 0, 0, location), wantEnd: now,
		},
		{
			name: "custom inclusive", query: "range_type=custom&start_date=2026-08-01&end_date=2026-08-03", wantType: "custom",
			wantStart: time.Date(2026, time.August, 1, 0, 0, 0, 0, location),
			wantEnd:   time.Date(2026, time.August, 4, 0, 0, 0, 0, location).Add(-time.Second),
		},
		{
			name: "custom 366 days", query: "range_type=custom&start_date=2025-08-19&end_date=2026-08-19", wantType: "custom",
			wantStart: time.Date(2025, time.August, 19, 0, 0, 0, 0, location), wantEnd: now,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			c.Request = httptest.NewRequest(http.MethodGet, "/api/quota_pool/1/stats?"+tt.query, nil)

			got, err := parseQuotaPoolStatsRange(c, now)

			assert.NoError(t, err)
			assert.Equal(t, tt.wantType, got.RangeType)
			assert.Equal(t, tt.wantStart.Unix(), got.StartTimestamp)
			assert.Equal(t, tt.wantEnd.Unix(), got.EndTimestamp)
			assert.Equal(t, tt.wantStart.Format(time.DateOnly), got.StartDate)
			assert.Equal(t, tt.wantEnd.Format(time.DateOnly), got.EndDate)
		})
	}
}

func TestParseQuotaPoolStatsRangeRejectsInvalidRanges(t *testing.T) {
	location := time.FixedZone("UTC+8", 8*60*60)
	now := time.Date(2026, time.August, 19, 15, 30, 0, 0, location)
	queries := []string{
		"range_type=custom&start_date=2026-08-03&end_date=2026-08-01",
		"range_type=custom&start_date=2025-08-18&end_date=2026-08-19",
		"range_type=custom&start_date=bad&end_date=2026-08-01",
		"range_type=custom&start_date=2026-08-19&end_date=2026-08-20",
		"range_type=month&anchor=2026-13",
		"range_type=week&anchor=2026-08-21",
		"range_type=week&anchor=2026-09-01",
	}
	for _, query := range queries {
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		c.Request = httptest.NewRequest(http.MethodGet, "/api/quota_pool/1/stats?"+query, nil)

		_, err := parseQuotaPoolStatsRange(c, now)

		assert.Error(t, err, query)
	}
}

func TestLoadQuotaPoolStatsRejectsInvalidRangeWithBadRequest(t *testing.T) {
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(
		http.MethodGet,
		"/api/quota_pool/1/stats?range_type=custom&start_date=2026-08-03&end_date=2026-08-01",
		nil,
	)

	_, ok := loadQuotaPoolStats(c, 1, time.Date(2026, time.August, 19, 15, 30, 0, 0, time.Local))

	assert.False(t, ok)
	assert.Equal(t, http.StatusBadRequest, recorder.Code)
	assert.Contains(t, recorder.Body.String(), `"code":"QUOTA_POOL_STATS_RANGE_INVALID"`)
}
