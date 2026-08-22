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
