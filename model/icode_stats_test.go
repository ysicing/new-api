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
	require.NoError(t, mainDB.AutoMigrate(&User{}, &QuotaData{}, &QuotaPool{}, &QuotaPoolTransaction{}))
	require.NoError(t, logDB.AutoMigrate(&Log{}))
	previousDB, previousLogDB := DB, LOG_DB
	DB, LOG_DB = mainDB, logDB
	common.SetDatabaseTypes(common.DatabaseTypeSQLite, common.DatabaseTypeSQLite)
	t.Cleanup(func() { DB, LOG_DB = previousDB, previousLogDB })
	return mainDB, logDB
}

func TestGetTopUsersAggregatesQuotaDataAndHydratesMainDB(t *testing.T) {
	mainDB, _ := setupICodeStatsTest(t)
	users := []User{
		{Id: 1, Username: "alice", Password: "password", AffCode: "stats-a", Quota: 500, UsedQuota: 300},
		{Id: 2, Username: "bob", Password: "password", AffCode: "stats-b", Quota: 200, UsedQuota: 900},
	}
	require.NoError(t, mainDB.Create(&users).Error)
	require.NoError(t, mainDB.Create(&[]QuotaData{
		{UserID: 1, Username: "alice", CreatedAt: 100, ModelName: "gpt-5", Quota: 30},
		{UserID: 2, Username: "bob", CreatedAt: 100, ModelName: "claude-4", Quota: 70},
		{UserID: 2, Username: "bob", CreatedAt: 110, ModelName: "gpt-5", Quota: 20},
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

func TestGetTopUsersUsesQuotaDataWithoutLogTail(t *testing.T) {
	mainDB, logDB := setupICodeStatsTest(t)
	now := time.Date(2026, time.August, 27, 12, 30, 0, 0, time.Local)
	start := now.AddDate(0, 0, -3)
	users := []User{
		{Id: 1, Username: "stored", Password: "password", AffCode: "stats-stored"},
		{Id: 2, Username: "old-log", Password: "password", AffCode: "stats-old-log"},
	}
	require.NoError(t, mainDB.Create(&users).Error)
	require.NoError(t, mainDB.Create(&QuotaData{
		UserID: 1, Username: "stored", ModelName: "gpt-5",
		CreatedAt: now.Add(-24 * time.Hour).Truncate(time.Hour).Unix(), Quota: 100,
	}).Error)
	require.NoError(t, logDB.Create(&[]Log{
		{UserId: 2, Type: LogTypeConsume, CreatedAt: now.Add(-24 * time.Hour).Unix(), ModelName: "gpt-5", Quota: 1000},
		{UserId: 1, Type: LogTypeConsume, CreatedAt: now.Add(-30 * time.Minute).Unix(), ModelName: "claude-4", Quota: 50},
	}).Error)

	results, err := GetTopUsers(start.Unix(), now.Unix(), "", 10)

	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, 1, results[0].UserId)
	assert.Equal(t, 100, results[0].UsedQuota)
	assert.Equal(t, 100, results[0].GptQuota)
	assert.Zero(t, results[0].ClaudeQuota)
}

func TestGetQuotaPoolStatsAggregatesMembersAndTransactions(t *testing.T) {
	mainDB, _ := setupICodeStatsTest(t)
	pool := QuotaPool{Name: "统计池", PoolType: QuotaPoolTypeNormal, Enabled: true, BaseQuota: 1000, Quota: 800}
	require.NoError(t, mainDB.Create(&pool).Error)
	user := User{Username: "pool-stats-user", Password: "password", AffCode: "pool-stats-aff", QuotaPoolId: pool.Id}
	require.NoError(t, mainDB.Create(&user).Error)
	require.NoError(t, mainDB.Create(&QuotaData{UserID: user.Id, Username: user.Username, CreatedAt: 100, ModelName: "deepseek-r1", Quota: 45}).Error)
	require.NoError(t, mainDB.Create(&[]QuotaPoolTransaction{
		{PoolId: pool.Id, Type: QuotaPoolTransactionAllocateAuto, Amount: -200, CreatedAt: 100},
		{PoolId: pool.Id, Type: QuotaPoolTransactionReclaimUser, Amount: 50, CreatedAt: 110},
	}).Error)

	stats, err := GetQuotaPoolStats(pool.Id, 90, 120)

	require.NoError(t, err)
	assert.Equal(t, 45, stats.TotalUsage)
	assert.Equal(t, 200, stats.TotalAllocate)
	assert.Equal(t, 50, stats.TotalReclaim)
	require.Len(t, stats.Usage, 1)
	assert.Equal(t, 45, stats.Usage[0].DeepSeekQuota)
}

func TestGetQuotaPoolStatsUsesQuotaDataWithoutLogTail(t *testing.T) {
	mainDB, logDB := setupICodeStatsTest(t)
	now := time.Date(2026, time.August, 27, 12, 30, 0, 0, time.Local)
	start := now.AddDate(0, 0, -3)
	pool := QuotaPool{Name: "聚合统计池", PoolType: QuotaPoolTypeNormal, Enabled: true}
	require.NoError(t, mainDB.Create(&pool).Error)
	user := User{Username: "pool-aggregate-user", Password: "password", AffCode: "pool-aggregate", QuotaPoolId: pool.Id}
	require.NoError(t, mainDB.Create(&user).Error)
	require.NoError(t, mainDB.Create(&QuotaData{
		UserID: user.Id, Username: user.Username, ModelName: "gpt-5",
		CreatedAt: now.Add(-24 * time.Hour).Truncate(time.Hour).Unix(), Quota: 100,
	}).Error)
	require.NoError(t, logDB.Create(&[]Log{
		{UserId: user.Id, Type: LogTypeConsume, CreatedAt: now.Add(-24 * time.Hour).Unix(), ModelName: "gpt-5", Quota: 1000},
		{UserId: user.Id, Type: LogTypeConsume, CreatedAt: now.Add(-30 * time.Minute).Unix(), ModelName: "claude-4", Quota: 50},
	}).Error)

	stats, err := GetQuotaPoolStats(pool.Id, start.Unix(), now.Unix())

	require.NoError(t, err)
	assert.Equal(t, 100, stats.TotalUsage)
	require.Len(t, stats.Usage, 1)
	assert.Equal(t, 100, stats.Usage[0].GptQuota)
	assert.Zero(t, stats.Usage[0].ClaudeQuota)
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
	mainDB, _ := setupICodeStatsTest(t)
	pool := QuotaPool{Name: QuotaPoolDefaultName, PoolType: QuotaPoolTypeDefault, IsDefault: true, Enabled: true}
	require.NoError(t, mainDB.Create(&pool).Error)
	user := User{Username: "default-pool-user", Password: "password", AffCode: "default-pool-stats", QuotaPoolId: QuotaPoolDefaultUserPoolId}
	require.NoError(t, mainDB.Create(&user).Error)
	require.NoError(t, mainDB.Create(&QuotaData{
		UserID: user.Id, Username: user.Username, CreatedAt: 100, ModelName: "claude-4", Quota: 45,
	}).Error)

	stats, err := GetQuotaPoolStats(pool.Id, 90, 120)

	require.NoError(t, err)
	assert.Equal(t, 45, stats.TotalUsage)
	require.Len(t, stats.Usage, 1)
	assert.Equal(t, user.Id, stats.Usage[0].UserId)
}

func TestGetQuotaPoolStatsInLocationIncludesActivityTrendAndInactiveMembers(t *testing.T) {
	mainDB, _ := setupICodeStatsTest(t)
	location := time.FixedZone("UTC+8", 8*60*60)
	pool := QuotaPool{Name: "成员活跃统计池", PoolType: QuotaPoolTypeNormal, Enabled: true}
	require.NoError(t, mainDB.Create(&pool).Error)
	members := []User{
		{Id: 1, Username: "alice", Password: "password", AffCode: "pool-active-a", QuotaPoolId: pool.Id},
		{Id: 2, Username: "bob", Password: "password", AffCode: "pool-active-b", QuotaPoolId: pool.Id},
		{Id: 3, Username: "outsider", Password: "password", AffCode: "pool-active-c", QuotaPoolId: 0},
	}
	require.NoError(t, mainDB.Create(&members).Error)
	require.NoError(t, mainDB.Create(&[]QuotaData{
		{UserID: 1, Username: "alice", CreatedAt: time.Date(2026, time.August, 17, 9, 0, 0, 0, location).Unix(), ModelName: "gpt-5", Count: 2, Quota: 30},
		{UserID: 1, Username: "alice", CreatedAt: time.Date(2026, time.August, 17, 10, 0, 0, 0, location).Unix(), ModelName: "claude-4", Count: 1, Quota: 20},
		{UserID: 1, Username: "alice", CreatedAt: time.Date(2026, time.August, 18, 11, 0, 0, 0, location).Unix(), ModelName: "gpt-5", Count: 3, Quota: 50},
		{UserID: 3, Username: "outsider", CreatedAt: time.Date(2026, time.August, 17, 9, 0, 0, 0, location).Unix(), ModelName: "gpt-5", Count: 10, Quota: 1000},
	}).Error)
	start := time.Date(2026, time.August, 17, 0, 0, 0, 0, location)
	end := time.Date(2026, time.August, 19, 15, 30, 0, 0, location)

	stats, err := GetQuotaPoolStatsInLocation(pool.Id, start.Unix(), end.Unix(), location)

	require.NoError(t, err)
	assert.Equal(t, 2, stats.Summary.MemberCount)
	assert.Equal(t, 1, stats.Summary.ActiveMembers)
	assert.Equal(t, 50.0, stats.Summary.ActiveRate)
	assert.Equal(t, 6, stats.Summary.RequestCount)
	assert.Equal(t, 100, stats.Summary.TotalUsage)
	assert.Equal(t, 100.0, stats.Summary.AverageUsagePerActiveMember)
	assert.Equal(t, "UTC+8", stats.TimeZone)
	require.Len(t, stats.Daily, 3)
	assert.Equal(t, QuotaPoolDailyStat{Date: "2026-08-17", ActiveMembers: 1, ActiveRate: 50, RequestCount: 3, UsedQuota: 50}, stats.Daily[0])
	assert.Equal(t, QuotaPoolDailyStat{Date: "2026-08-19"}, stats.Daily[2])
	require.Len(t, stats.Members, 2)
	assert.Equal(t, "alice", stats.Members[0].Username)
	assert.True(t, stats.Members[0].Active)
	assert.Equal(t, 2, stats.Members[0].ActiveDays)
	assert.Equal(t, 6, stats.Members[0].RequestCount)
	assert.Equal(t, 100, stats.Members[0].UsedQuota)
	assert.Equal(t, 50.0, stats.Members[0].AverageDailyUsage)
	assert.Equal(t, 100.0, stats.Members[0].UsageShare)
	assert.Equal(t, time.Date(2026, time.August, 18, 11, 0, 0, 0, location).Unix(), stats.Members[0].LastActiveAt)
	assert.Equal(t, "2026-08-18 11:00:00 +08:00 UTC+8", stats.Members[0].LastActiveTime)
	assert.Equal(t, "bob", stats.Members[1].Username)
	assert.False(t, stats.Members[1].Active)
	assert.Zero(t, stats.Members[1].UsedQuota)
}

func TestGetQuotaPoolStatsInLocationRejectsHalfHourTimezone(t *testing.T) {
	location := time.FixedZone("UTC+5:30", 5*60*60+30*60)
	start := time.Date(2026, time.August, 17, 0, 0, 0, 0, location)
	end := time.Date(2026, time.August, 17, 23, 59, 59, 0, location)

	_, err := GetQuotaPoolStatsInLocation(1, start.Unix(), end.Unix(), location)

	assert.ErrorIs(t, err, ErrQuotaPoolStatsTimezoneUnsupported)
}

func TestGetQuotaPoolStatsInLocationRejectsHalfHourDSTTransition(t *testing.T) {
	location, err := time.LoadLocation("Australia/Lord_Howe")
	require.NoError(t, err)
	start := time.Date(2026, time.April, 5, 0, 0, 0, 0, location)
	end := time.Date(2026, time.April, 5, 23, 59, 59, 0, location)

	_, err = GetQuotaPoolStatsInLocation(1, start.Unix(), end.Unix(), location)

	assert.ErrorIs(t, err, ErrQuotaPoolStatsTimezoneUnsupported)
}

func TestGetRechargeLeaderboardUsesStableSecondaryUsageSort(t *testing.T) {
	mainDB, _ := setupICodeStatsTest(t)
	users := []User{
		{Id: 1, Username: "alice", Password: "password", AffCode: "recharge-a"},
		{Id: 2, Username: "bob", Password: "password", AffCode: "recharge-b"},
	}
	require.NoError(t, mainDB.Create(&users).Error)
	now := common.GetTimestamp()
	require.NoError(t, mainDB.Create(&[]QuotaPoolTransaction{
		{PoolId: 1, UserId: 1, Type: QuotaPoolTransactionAllocateAuto, Amount: -100, CreatedAt: now},
		{PoolId: 1, UserId: 2, Type: QuotaPoolTransactionAllocateAuto, Amount: -100, CreatedAt: now},
	}).Error)
	require.NoError(t, mainDB.Create(&[]QuotaData{
		{UserID: 1, Username: "alice", ModelName: "gpt-5", CreatedAt: now - now%3600 - 7200, Quota: 20},
		{UserID: 2, Username: "bob", ModelName: "claude-4", CreatedAt: now - now%3600 - 7200, Quota: 50},
	}).Error)
	results, err := GetRechargeLeaderboard(10)

	require.NoError(t, err)
	require.Len(t, results, 2)
	assert.Equal(t, 2, results[0].UserId)
	assert.Equal(t, 1, results[0].AutoRechargeCount)
	assert.Equal(t, 50, results[0].UsedQuota)
}

func TestGetRechargeLeaderboardAtUsesProvidedWeek(t *testing.T) {
	mainDB, _ := setupICodeStatsTest(t)
	user := User{Id: 1, Username: "alice", Password: "password", AffCode: "recharge-at"}
	require.NoError(t, mainDB.Create(&user).Error)
	now := time.Date(2026, time.August, 27, 12, 0, 0, 0, time.Local)
	require.NoError(t, mainDB.Create(&[]QuotaPoolTransaction{
		{PoolId: 1, UserId: user.Id, Type: QuotaPoolTransactionAllocateAuto, Amount: -100, CreatedAt: now.Add(-24 * time.Hour).Unix()},
		{PoolId: 1, UserId: user.Id, Type: QuotaPoolTransactionAllocateAuto, Amount: -100, CreatedAt: now.Add(-8 * 24 * time.Hour).Unix()},
	}).Error)

	results, err := GetRechargeLeaderboardAt(10, now)

	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, 1, results[0].TotalCount)
}

func TestGetRechargeLeaderboardUsesTransactionsWithoutLogs(t *testing.T) {
	mainDB, logDB := setupICodeStatsTest(t)
	now := time.Date(2026, time.August, 27, 12, 30, 0, 0, time.Local)
	users := []User{
		{Id: 1, Username: "pool-user", Password: "password", AffCode: "recharge-pool"},
		{Id: 2, Username: "default-recent", Password: "password", AffCode: "recharge-default-recent"},
		{Id: 3, Username: "default-old", Password: "password", AffCode: "recharge-default-old"},
	}
	require.NoError(t, mainDB.Create(&users).Error)
	require.NoError(t, mainDB.Create(&QuotaPoolTransaction{
		PoolId: 1, Type: QuotaPoolTransactionAllocateAuto, Amount: -100,
		UserId: 1, CreatedAt: now.Add(-24 * time.Hour).Unix(),
	}).Error)
	require.NoError(t, mainDB.Create(&[]QuotaData{
		{UserID: 1, Username: "pool-user", ModelName: "gpt-5", CreatedAt: now.Add(-24 * time.Hour).Truncate(time.Hour).Unix(), Quota: 50},
		{UserID: 2, Username: "default-recent", ModelName: "claude-4", CreatedAt: now.Add(-24 * time.Hour).Truncate(time.Hour).Unix(), Quota: 20},
	}).Error)
	require.NoError(t, logDB.Create(&[]Log{
		{UserId: 2, Type: LogTypeSystem, CreatedAt: now.Add(-30 * time.Minute).Unix(), Content: "系统自动赠送 100", Other: `{"recharge_source":"auto","quota_pool_id":0}`},
		{UserId: 3, Type: LogTypeSystem, CreatedAt: now.Add(-24 * time.Hour).Unix(), Content: "系统自动赠送 100", Other: `{"recharge_source":"auto","quota_pool_id":0}`},
	}).Error)

	results, err := GetRechargeLeaderboardAt(10, now)

	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, 1, results[0].UserId)
	assert.Equal(t, 1, results[0].AutoRechargeCount)
	assert.Equal(t, 50, results[0].UsedQuota)
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
			Type: LogTypeTopup, Content: "pool-1-recharge", Other: common.MapToJsonStr(map[string]any{
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
	assert.EqualValues(t, 2, total)
	require.Len(t, items, 2)
	assert.Equal(t, "pool-1-recharge", items[0].Content)
	assert.Equal(t, "pool-1", items[1].Content)
}
