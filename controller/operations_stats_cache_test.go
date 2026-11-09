package controller

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/alicebob/miniredis/v2"
	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupOperationsStatsCacheTest(t *testing.T) {
	t.Helper()
	db := setupModelListControllerTestDB(t)
	require.NoError(t, db.AutoMigrate(&model.Log{}, &model.QuotaData{}, &model.QuotaPoolTransaction{}))
	previousEnabled, previousClient := common.RedisEnabled, common.RDB
	common.RedisEnabled = false
	common.RDB = nil
	t.Cleanup(func() {
		common.RedisEnabled = previousEnabled
		common.RDB = previousClient
	})
}

func TestOperationsStatsEndpointsReturnEmptyDirectResultWithoutSnapshot(t *testing.T) {
	setupOperationsStatsCacheTest(t)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/log/top_users?period=week&limit=10", nil)

	GetTopUsers(c)

	assert.Equal(t, http.StatusOK, recorder.Code)
	assert.Contains(t, recorder.Body.String(), `"refreshing":false`)
	assert.Contains(t, recorder.Body.String(), `"refresh_schedule":"every_5_minutes"`)
	assert.Contains(t, recorder.Body.String(), `"data":[]`)
	assert.NotContains(t, recorder.Body.String(), `"generated_at":0`)
}

func TestOperationsStatsEndpointsQueryAggregatedDataDirectly(t *testing.T) {
	setupOperationsStatsCacheTest(t)
	now := time.Now().Truncate(time.Second)
	user := model.User{Id: 1, Username: "direct-stats", Password: "password", AffCode: "stats-direct"}
	require.NoError(t, model.DB.Create(&user).Error)
	require.NoError(t, model.DB.Create(&model.QuotaData{
		UserID: user.Id, Username: user.Username, ModelName: "gpt-5",
		CreatedAt: now.Add(-24 * time.Hour).Truncate(time.Hour).Unix(), Quota: 20,
	}).Error)
	require.NoError(t, model.DB.Create(&model.QuotaPoolTransaction{
		PoolId: 1, UserId: user.Id, Type: model.QuotaPoolTransactionAllocateAuto,
		Amount: -100, CreatedAt: now.Unix(),
	}).Error)

	topRecorder := httptest.NewRecorder()
	topContext, _ := gin.CreateTestContext(topRecorder)
	topContext.Request = httptest.NewRequest(http.MethodGet, "/api/log/top_users?period=month&limit=10", nil)
	GetTopUsers(topContext)

	assert.Equal(t, http.StatusOK, topRecorder.Code)
	assert.Contains(t, topRecorder.Body.String(), "direct-stats")
	assert.Contains(t, topRecorder.Body.String(), `"refresh_schedule":"every_5_minutes"`)
	assert.Contains(t, topRecorder.Body.String(), `"refreshing":false`)

	rechargeRecorder := httptest.NewRecorder()
	rechargeContext, _ := gin.CreateTestContext(rechargeRecorder)
	rechargeContext.Request = httptest.NewRequest(http.MethodGet, "/api/log/recharge_leaderboard?limit=10", nil)
	GetRechargeLeaderboard(rechargeContext)

	assert.Equal(t, http.StatusOK, rechargeRecorder.Code)
	assert.Contains(t, rechargeRecorder.Body.String(), "direct-stats")
	assert.Contains(t, rechargeRecorder.Body.String(), `"total_count":1`)
	assert.Contains(t, rechargeRecorder.Body.String(), `"refresh_schedule":"every_5_minutes"`)
}

func TestRechargeLeaderboardSeparatesWeekAndMonthCaches(t *testing.T) {
	setupOperationsStatsCacheTest(t)
	server := miniredis.RunT(t)
	common.RedisEnabled = true
	common.RDB = redis.NewClient(&redis.Options{Addr: server.Addr()})
	t.Cleanup(func() { _ = common.RDB.Close() })

	now := time.Now()
	user := model.User{Id: 1, Username: "month-recharge", Password: "password", AffCode: "stats-month-recharge"}
	require.NoError(t, model.DB.Create(&user).Error)
	require.NoError(t, model.DB.Create(&model.QuotaPoolTransaction{
		PoolId: 1, UserId: user.Id, Type: model.QuotaPoolTransactionAllocateAuto,
		Amount: -100, CreatedAt: now.AddDate(0, 0, -20).Unix(),
	}).Error)

	weekRecorder := httptest.NewRecorder()
	weekContext, _ := gin.CreateTestContext(weekRecorder)
	weekContext.Request = httptest.NewRequest(http.MethodGet, "/api/log/recharge_leaderboard?period=week&limit=10", nil)
	GetRechargeLeaderboard(weekContext)
	assert.NotContains(t, weekRecorder.Body.String(), "month-recharge")

	monthRecorder := httptest.NewRecorder()
	monthContext, _ := gin.CreateTestContext(monthRecorder)
	monthContext.Request = httptest.NewRequest(http.MethodGet, "/api/log/recharge_leaderboard?period=month&limit=10", nil)
	GetRechargeLeaderboard(monthContext)
	assert.Contains(t, monthRecorder.Body.String(), "month-recharge")
}

func TestOperationsStatsEndpointsApplyRequestedLimitToDirectResult(t *testing.T) {
	setupOperationsStatsCacheTest(t)
	now := time.Now()
	users := []model.User{
		{Id: 1, Username: "first", Password: "password", AffCode: "stats-first"},
		{Id: 2, Username: "second", Password: "password", AffCode: "stats-second"},
	}
	require.NoError(t, model.DB.Create(&users).Error)
	require.NoError(t, model.DB.Create(&[]model.QuotaData{
		{UserID: 1, Username: "first", ModelName: "gpt-5", CreatedAt: now.Unix(), Quota: 20},
		{UserID: 2, Username: "second", ModelName: "gpt-5", CreatedAt: now.Unix(), Quota: 10},
	}).Error)

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/log/top_users?period=week&limit=1", nil)
	GetTopUsers(c)

	assert.Contains(t, recorder.Body.String(), "first")
	assert.NotContains(t, recorder.Body.String(), "second")
}
