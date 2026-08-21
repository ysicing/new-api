package model

import (
	"fmt"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupICodeStatsTest(t *testing.T) (*gorm.DB, *gorm.DB) {
	t.Helper()
	mainDB, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:stats-main-%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	require.NoError(t, err)
	logDB, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:stats-log-%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, mainDB.AutoMigrate(&User{}, &QuotaPool{}, &QuotaPoolTransaction{}))
	require.NoError(t, logDB.AutoMigrate(&Log{}))
	previousDB, previousLogDB := DB, LOG_DB
	DB, LOG_DB = mainDB, logDB
	common.SetDatabaseTypes(common.DatabaseTypeSQLite, common.DatabaseTypeSQLite)
	t.Cleanup(func() { DB, LOG_DB = previousDB, previousLogDB })
	return mainDB, logDB
}

func TestGetTopUsersAggregatesLogDBAndHydratesMainDB(t *testing.T) {
	mainDB, logDB := setupICodeStatsTest(t)
	users := []User{
		{Id: 1, Username: "alice", Password: "password", AffCode: "stats-a", Quota: 500, UsedQuota: 300},
		{Id: 2, Username: "bob", Password: "password", AffCode: "stats-b", Quota: 200, UsedQuota: 900},
	}
	require.NoError(t, mainDB.Create(&users).Error)
	require.NoError(t, logDB.Create(&[]Log{
		{UserId: 1, Type: LogTypeConsume, CreatedAt: 100, ModelName: "gpt-5", Quota: 30},
		{UserId: 2, Type: LogTypeConsume, CreatedAt: 100, ModelName: "claude-4", Quota: 70},
		{UserId: 2, Type: LogTypeConsume, CreatedAt: 110, ModelName: "gpt-5", Quota: 20},
	}).Error)

	results, err := GetTopUsers(90, 120, "", 10)

	require.NoError(t, err)
	require.Len(t, results, 2)
	assert.Equal(t, 2, results[0].UserId)
	assert.Equal(t, 90, results[0].UsedQuota)
	assert.Equal(t, 70, results[0].ClaudeQuota)
	assert.Equal(t, 20, results[0].GptQuota)
	assert.Equal(t, "bob", results[0].Username)
}

func TestGetQuotaPoolStatsAggregatesMembersAndTransactions(t *testing.T) {
	mainDB, logDB := setupICodeStatsTest(t)
	pool := QuotaPool{Name: "统计池", PoolType: QuotaPoolTypeNormal, Enabled: true, BaseQuota: 1000, Quota: 800}
	require.NoError(t, mainDB.Create(&pool).Error)
	user := User{Username: "pool-stats-user", Password: "password", AffCode: "pool-stats-aff", QuotaPoolId: pool.Id}
	require.NoError(t, mainDB.Create(&user).Error)
	require.NoError(t, logDB.Create(&Log{UserId: user.Id, Type: LogTypeConsume, CreatedAt: 100, ModelName: "deepseek-r1", Quota: 45}).Error)
	require.NoError(t, mainDB.Create(&QuotaPoolTransaction{PoolId: pool.Id, Type: QuotaPoolTransactionAllocateAuto, Amount: -200, CreatedAt: 100}).Error)

	stats, err := GetQuotaPoolStats(pool.Id, 90, 120)

	require.NoError(t, err)
	assert.Equal(t, 45, stats.TotalUsage)
	assert.Equal(t, 200, stats.TotalAllocate)
	require.Len(t, stats.Usage, 1)
	assert.Equal(t, 45, stats.Usage[0].DeepSeekQuota)
}

func TestGetRechargeLeaderboardUsesStableSecondaryUsageSort(t *testing.T) {
	mainDB, logDB := setupICodeStatsTest(t)
	users := []User{
		{Id: 1, Username: "alice", Password: "password", AffCode: "recharge-a"},
		{Id: 2, Username: "bob", Password: "password", AffCode: "recharge-b"},
	}
	require.NoError(t, mainDB.Create(&users).Error)
	now := common.GetTimestamp()
	require.NoError(t, logDB.Create(&[]Log{
		{UserId: 1, Type: LogTypeSystem, CreatedAt: now, Content: "系统自动赠送 100"},
		{UserId: 2, Type: LogTypeSystem, CreatedAt: now, Other: `{"recharge_source":"auto"}`},
		{UserId: 1, Type: LogTypeConsume, CreatedAt: now, ModelName: "gpt-5", Quota: 20},
		{UserId: 2, Type: LogTypeConsume, CreatedAt: now, ModelName: "claude-4", Quota: 50},
	}).Error)

	results, err := GetRechargeLeaderboard(10)

	require.NoError(t, err)
	require.Len(t, results, 2)
	assert.Equal(t, 2, results[0].UserId)
	assert.Equal(t, 1, results[0].AutoRechargeCount)
	assert.Equal(t, 50, results[0].UsedQuota)
}
