package service

import (
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/alicebob/miniredis/v2"
	"github.com/go-redis/redis/v8"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func useOperationsStatsRedis(t *testing.T) *miniredis.Miniredis {
	t.Helper()
	server := miniredis.RunT(t)
	previousEnabled, previousClient := common.RedisEnabled, common.RDB
	common.RedisEnabled = true
	common.RDB = redis.NewClient(&redis.Options{Addr: server.Addr()})
	t.Cleanup(func() {
		_ = common.RDB.Close()
		common.RedisEnabled = previousEnabled
		common.RDB = previousClient
	})
	return server
}

func TestOperationsTopUsersCachesResultForFiveMinutes(t *testing.T) {
	truncate(t)
	server := useOperationsStatsRedis(t)
	now := time.Date(2026, time.August, 27, 12, 30, 0, 0, time.Local)
	user := model.User{Id: 1, Username: "cached-user", Password: "password", AffCode: "stats-cache-user"}
	require.NoError(t, model.DB.Create(&user).Error)
	require.NoError(t, model.DB.Create(&model.QuotaData{
		UserID: user.Id, Username: user.Username, ModelName: "gpt-5",
		CreatedAt: now.Add(-24 * time.Hour).Truncate(time.Hour).Unix(), Quota: 100,
	}).Error)

	first, err := GetOperationsTopUsers("week", now)
	require.NoError(t, err)
	require.Len(t, first.Items, 1)
	assert.Equal(t, 100, first.Items[0].UsedQuota)

	require.NoError(t, model.DB.Model(&model.QuotaData{}).Where("user_id = ?", user.Id).Update("quota", 200).Error)
	server.FastForward(4*time.Minute + 59*time.Second)
	second, err := GetOperationsTopUsers("week", now.Add(4*time.Minute+59*time.Second))
	require.NoError(t, err)
	assert.Equal(t, 100, second.Items[0].UsedQuota)
	assert.Equal(t, first.GeneratedAt, second.GeneratedAt)

	server.FastForward(2 * time.Second)
	third, err := GetOperationsTopUsers("week", now.Add(5*time.Minute+1*time.Second))
	require.NoError(t, err)
	assert.Equal(t, 200, third.Items[0].UsedQuota)
	assert.Greater(t, third.GeneratedAt, second.GeneratedAt)
}

func TestOperationsTopUsersFallsBackWhenRedisIsUnavailable(t *testing.T) {
	truncate(t)
	server := useOperationsStatsRedis(t)
	server.Close()
	now := time.Date(2026, time.August, 27, 12, 30, 0, 0, time.Local)
	user := model.User{Id: 1, Username: "redis-down", Password: "password", AffCode: "stats-redis-down"}
	require.NoError(t, model.DB.Create(&user).Error)
	require.NoError(t, model.DB.Create(&model.QuotaData{
		UserID: user.Id, Username: user.Username, ModelName: "gpt-5",
		CreatedAt: now.Add(-24 * time.Hour).Truncate(time.Hour).Unix(), Quota: 100,
	}).Error)

	result, err := GetOperationsTopUsers("week", now)

	require.NoError(t, err)
	require.Len(t, result.Items, 1)
	assert.Equal(t, 100, result.Items[0].UsedQuota)
}

func TestOperationsTopUsersFallsBackToDirectQueryWithoutRedis(t *testing.T) {
	truncate(t)
	previousEnabled, previousClient := common.RedisEnabled, common.RDB
	common.RedisEnabled = false
	common.RDB = nil
	t.Cleanup(func() {
		common.RedisEnabled = previousEnabled
		common.RDB = previousClient
	})
	now := time.Date(2026, time.August, 27, 12, 30, 0, 0, time.Local)
	user := model.User{Id: 1, Username: "direct-user", Password: "password", AffCode: "stats-direct-user"}
	require.NoError(t, model.DB.Create(&user).Error)
	require.NoError(t, model.DB.Create(&model.QuotaData{
		UserID: user.Id, Username: user.Username, ModelName: "gpt-5",
		CreatedAt: now.Add(-24 * time.Hour).Truncate(time.Hour).Unix(), Quota: 100,
	}).Error)

	first, err := GetOperationsTopUsers("week", now)
	require.NoError(t, err)
	require.NoError(t, model.DB.Model(&model.QuotaData{}).Where("user_id = ?", user.Id).Update("quota", 200).Error)
	second, err := GetOperationsTopUsers("week", now.Add(time.Second))

	require.NoError(t, err)
	assert.Equal(t, 100, first.Items[0].UsedQuota)
	assert.Equal(t, 200, second.Items[0].UsedQuota)
}

func TestQuotaPoolStatsUsesFiveMinuteRedisCache(t *testing.T) {
	truncate(t)
	server := useOperationsStatsRedis(t)
	now := time.Date(2026, time.August, 27, 12, 30, 0, 0, time.Local)
	pool := model.QuotaPool{Name: "缓存池", PoolType: model.QuotaPoolTypeNormal, Enabled: true}
	require.NoError(t, model.DB.Create(&pool).Error)
	user := model.User{Id: 1, Username: "pool-user", Password: "password", AffCode: "stats-cache-pool", QuotaPoolId: pool.Id}
	require.NoError(t, model.DB.Create(&user).Error)
	require.NoError(t, model.DB.Create(&model.QuotaData{
		UserID: user.Id, Username: user.Username, ModelName: "gpt-5",
		CreatedAt: now.Add(-24 * time.Hour).Truncate(time.Hour).Unix(), Quota: 100,
	}).Error)
	start := now.AddDate(0, 0, -3).Unix()

	first, firstGeneratedAt, err := GetCachedQuotaPoolStats(pool.Id, start, now.Unix(), now)
	require.NoError(t, err)
	assert.Equal(t, 100, first.TotalUsage)

	require.NoError(t, model.DB.Model(&model.QuotaData{}).Where("user_id = ?", user.Id).Update("quota", 200).Error)
	second, secondGeneratedAt, err := GetCachedQuotaPoolStats(pool.Id, start, now.Unix(), now.Add(4*time.Minute+59*time.Second))
	require.NoError(t, err)
	assert.Equal(t, 100, second.TotalUsage)
	assert.Equal(t, firstGeneratedAt, secondGeneratedAt)

	server.FastForward(5*time.Minute + time.Second)
	third, thirdGeneratedAt, err := GetCachedQuotaPoolStats(pool.Id, start, now.Unix(), now.Add(5*time.Minute+1*time.Second))
	require.NoError(t, err)
	assert.Equal(t, 200, third.TotalUsage)
	assert.Greater(t, thirdGeneratedAt, secondGeneratedAt)
}
