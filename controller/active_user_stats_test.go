package controller

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetActiveUserStatsReturnsIndependentDailyResponse(t *testing.T) {
	db := setupModelListControllerTestDB(t)
	require.NoError(t, db.AutoMigrate(&model.QuotaData{}))
	now := time.Now().Truncate(time.Second)
	require.NoError(t, db.Create(&model.QuotaData{
		UserID: 1, Username: "alice", CreatedAt: now.Truncate(time.Hour).Unix(), Count: 1,
	}).Error)
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	path := "/api/data/active-users?start_timestamp=" + strconv.FormatInt(now.Add(-time.Hour).Unix(), 10) + "&end_timestamp=" + strconv.FormatInt(now.Unix(), 10)
	context.Request = httptest.NewRequest(http.MethodGet, path, nil)

	GetActiveUserStats(context)

	assert.Equal(t, http.StatusOK, recorder.Code)
	var response struct {
		Success bool                  `json:"success"`
		Data    model.ActiveUserStats `json:"data"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	assert.True(t, response.Success)
	assert.Equal(t, 1, response.Data.TotalActiveUsers)
	require.NotEmpty(t, response.Data.Daily)
}

func TestGetActiveUserStatsRejectsRangesLongerThanThirtyDays(t *testing.T) {
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodGet, "/api/data/active-users?start_timestamp=1&end_timestamp=2678402", nil)

	GetActiveUserStats(context)

	assert.Equal(t, http.StatusBadRequest, recorder.Code)
	assert.Contains(t, recorder.Body.String(), `"code":"ACTIVE_USERS_INVALID_RANGE"`)
}

func TestValidActiveUserStatsRangeCountsLocalCalendarDaysAcrossDST(t *testing.T) {
	location, err := time.LoadLocation("America/New_York")
	require.NoError(t, err)
	start := time.Date(2026, time.March, 1, 0, 0, 0, 0, location)
	end := time.Date(2026, time.March, 30, 23, 59, 59, 0, location)

	assert.True(t, validActiveUserStatsRange(start.Unix(), end.Unix(), location))
	assert.False(t, validActiveUserStatsRange(start.Unix(), end.AddDate(0, 0, 1).Unix(), location))
}
