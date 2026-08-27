package controller

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupOperationsStatsCacheTest(t *testing.T) {
	t.Helper()
	db := setupModelListControllerTestDB(t)
	require.NoError(t, db.AutoMigrate(&model.Log{}, &model.SystemTask{}, &model.SystemTaskLock{}))
}

func TestOperationsStatsEndpointsReturnThinkingStateWithoutSnapshot(t *testing.T) {
	setupOperationsStatsCacheTest(t)
	require.NoError(t, model.DB.Create(&model.User{
		Id: 1, Username: "live-query-must-not-run", Password: "password", AffCode: "stats-no-cache",
	}).Error)
	require.NoError(t, model.LOG_DB.Create(&model.Log{
		UserId: 1, Type: model.LogTypeConsume, CreatedAt: common.GetTimestamp(), ModelName: "gpt-5", Quota: 100,
	}).Error)

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/log/top_users?period=week&limit=10", nil)

	GetTopUsers(c)

	assert.Equal(t, http.StatusOK, recorder.Code)
	assert.Contains(t, recorder.Body.String(), `"refreshing":true`)
	assert.Contains(t, recorder.Body.String(), `"data":[]`)
	assert.NotContains(t, recorder.Body.String(), "live-query-must-not-run")
}

func TestOperationsStatsEndpointsReadLatestSuccessfulSnapshot(t *testing.T) {
	setupOperationsStatsCacheTest(t)
	result := `{
  "version":1,
  "weekly_top_users":{"generated_at":100,"items":[{"user_id":1,"username":"cached-week","used_quota":10}]},
  "monthly_top_users":{"generated_at":200,"items":[{"user_id":2,"username":"cached-month","used_quota":20}]},
  "recharge_leaderboard":{"generated_at":300,"items":[{"user_id":3,"username":"cached-recharge","total_count":2}]}
}`
	require.NoError(t, model.DB.Create(&model.SystemTask{
		TaskID: "cached-stats", Type: "operations_stats_refresh",
		Status: model.SystemTaskStatusSucceeded, Result: result,
	}).Error)

	topRecorder := httptest.NewRecorder()
	topContext, _ := gin.CreateTestContext(topRecorder)
	topContext.Request = httptest.NewRequest(http.MethodGet, "/api/log/top_users?period=month&limit=10", nil)
	GetTopUsers(topContext)

	assert.Equal(t, http.StatusOK, topRecorder.Code)
	assert.Contains(t, topRecorder.Body.String(), "cached-month")
	assert.Contains(t, topRecorder.Body.String(), `"generated_at":200`)
	assert.Contains(t, topRecorder.Body.String(), `"refresh_schedule":"daily_after_midnight"`)
	assert.Contains(t, topRecorder.Body.String(), `"refreshing":false`)

	rechargeRecorder := httptest.NewRecorder()
	rechargeContext, _ := gin.CreateTestContext(rechargeRecorder)
	rechargeContext.Request = httptest.NewRequest(http.MethodGet, "/api/log/recharge_leaderboard?limit=10", nil)
	GetRechargeLeaderboard(rechargeContext)

	assert.Equal(t, http.StatusOK, rechargeRecorder.Code)
	assert.Contains(t, rechargeRecorder.Body.String(), "cached-recharge")
	assert.Contains(t, rechargeRecorder.Body.String(), `"generated_at":300`)
	assert.Contains(t, rechargeRecorder.Body.String(), `"refresh_schedule":"every_30_minutes"`)
}

func TestOperationsStatsEndpointsApplyRequestedLimitToSnapshot(t *testing.T) {
	setupOperationsStatsCacheTest(t)
	result := `{
  "version":1,
  "weekly_top_users":{"generated_at":100,"items":[
    {"user_id":1,"username":"first","used_quota":20},
    {"user_id":2,"username":"second","used_quota":10}
  ]},
  "monthly_top_users":{"generated_at":100,"items":[]},
  "recharge_leaderboard":{"generated_at":100,"items":[]}
}`
	require.NoError(t, model.DB.Create(&model.SystemTask{
		TaskID: "limited-stats", Type: model.SystemTaskTypeOperationsStatsRefresh,
		Status: model.SystemTaskStatusSucceeded, Result: result,
	}).Error)

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/log/top_users?period=week&limit=1", nil)
	GetTopUsers(c)

	assert.Contains(t, recorder.Body.String(), "first")
	assert.NotContains(t, recorder.Body.String(), "second")
}
