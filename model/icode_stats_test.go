package model

import (
	"fmt"
	"testing"
	"time"

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

func TestGetQuotaPoolStatsDoesNotAggregateUnrelatedLogsForEmptyPool(t *testing.T) {
	mainDB, logDB := setupICodeStatsTest(t)
	pool := QuotaPool{Name: "空池", PoolType: QuotaPoolTypeNormal, Enabled: true}
	require.NoError(t, mainDB.Create(&pool).Error)
	require.NoError(t, logDB.Create(&Log{
		UserId: 99, Type: LogTypeConsume, CreatedAt: 100, ModelName: "gpt-5", Quota: 70,
	}).Error)
	require.NoError(t, mainDB.Create(&QuotaPoolTransaction{
		PoolId: pool.Id, Type: QuotaPoolTransactionManualRefill, Amount: 300, CreatedAt: 100,
	}).Error)

	stats, err := GetQuotaPoolStats(pool.Id, 90, 120)

	require.NoError(t, err)
	assert.Empty(t, stats.Usage)
	assert.Equal(t, 0, stats.TotalUsage)
	assert.Equal(t, 300, stats.TotalRefill)
}

func TestGetQuotaPoolStatsMapsDefaultPoolMembersToVirtualPoolID(t *testing.T) {
	mainDB, logDB := setupICodeStatsTest(t)
	pool := QuotaPool{Name: QuotaPoolDefaultName, PoolType: QuotaPoolTypeDefault, IsDefault: true, Enabled: true}
	require.NoError(t, mainDB.Create(&pool).Error)
	user := User{Username: "default-pool-user", Password: "password", AffCode: "default-pool-stats", QuotaPoolId: QuotaPoolDefaultUserPoolId}
	require.NoError(t, mainDB.Create(&user).Error)
	require.NoError(t, logDB.Create(&Log{
		UserId: user.Id, Type: LogTypeConsume, CreatedAt: 100, ModelName: "claude-4", Quota: 45,
	}).Error)

	stats, err := GetQuotaPoolStats(pool.Id, 90, 120)

	require.NoError(t, err)
	assert.Equal(t, 45, stats.TotalUsage)
	require.Len(t, stats.Usage, 1)
	assert.Equal(t, user.Id, stats.Usage[0].UserId)
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

func TestGetRechargeLeaderboardAtUsesProvidedWeek(t *testing.T) {
	mainDB, logDB := setupICodeStatsTest(t)
	user := User{Id: 1, Username: "alice", Password: "password", AffCode: "recharge-at"}
	require.NoError(t, mainDB.Create(&user).Error)
	now := time.Date(2026, time.August, 27, 12, 0, 0, 0, time.Local)
	require.NoError(t, logDB.Create(&[]Log{
		{UserId: user.Id, Type: LogTypeSystem, CreatedAt: now.Add(-24 * time.Hour).Unix(), Content: "系统自动赠送 100"},
		{UserId: user.Id, Type: LogTypeSystem, CreatedAt: now.Add(-8 * 24 * time.Hour).Unix(), Content: "系统自动赠送 100"},
	}).Error)

	results, err := GetRechargeLeaderboardAt(10, now)

	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, 1, results[0].TotalCount)
}

func TestListQuotaPoolOperationLogsMatchesExactPoolID(t *testing.T) {
	_, logDB := setupICodeStatsTest(t)
	logs := []Log{
		{
			Type: LogTypeManage, Content: "pool-1", Other: common.MapToJsonStr(map[string]any{
				"admin_info": map[string]any{"admin_id": 7, "admin_username": "root", "quota_pool_id": 1},
			}),
		},
		{
			Type: LogTypeManage, Content: "pool-10", Other: common.MapToJsonStr(map[string]any{
				"admin_info": map[string]any{"admin_id": 7, "admin_username": "root", "quota_pool_id": 10},
			}),
		},
	}
	require.NoError(t, logDB.Create(&logs).Error)

	items, total, err := ListQuotaPoolOperationLogs(1, &common.PageInfo{Page: 1, PageSize: 10})

	require.NoError(t, err)
	assert.EqualValues(t, 1, total)
	require.Len(t, items, 1)
	assert.Equal(t, "pool-1", items[0].Content)
}
