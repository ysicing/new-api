package model

import (
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestQuotaPoolMemberStatsCountsOnlySuccessfulPoolAllocations(t *testing.T) {
	db, logDB := setupICodeStatsTest(t)
	pool := QuotaPool{Id: 7, Name: "member-recharge", PoolType: QuotaPoolTypeNormal, Enabled: true}
	require.NoError(t, db.Create(&pool).Error)
	require.NoError(t, db.Create(&[]User{{Id: 1, Username: "alice", AffCode: "stat-a", QuotaPoolId: 7}, {Id: 2, Username: "inactive", AffCode: "stat-b", QuotaPoolId: 7}}).Error)
	require.NoError(t, db.Create(&[]QuotaPoolTransaction{
		{PoolId: 7, UserId: 1, Type: QuotaPoolTransactionAllocateAuto, Amount: -100, CreatedAt: 100},
		{PoolId: 7, UserId: 1, Type: QuotaPoolTransactionAllocateAuto, Amount: -200, CreatedAt: 200},
		{PoolId: 7, UserId: 1, OperatorId: 2, Type: QuotaPoolTransactionAllocateManual, Amount: -50, CreatedAt: 150},
		{PoolId: 7, UserId: 2, Type: QuotaPoolTransactionAllocateManual, Amount: -30, CreatedAt: 150},
		{PoolId: 7, UserId: 1, Type: QuotaPoolTransactionAllocateAuto, Amount: -900, CreatedAt: 99},
		{PoolId: 7, UserId: 1, Type: QuotaPoolTransactionAllocateAuto, Amount: -900, CreatedAt: 201},
		{PoolId: 8, UserId: 1, Type: QuotaPoolTransactionAllocateAuto, Amount: -900, CreatedAt: 150},
		{PoolId: 7, UserId: 1, Type: QuotaPoolTransactionReclaimUser, Amount: 900, CreatedAt: 150},
		{PoolId: 7, UserId: 1, Type: QuotaPoolTransactionManualRefill, Amount: 900, CreatedAt: 150},
		{PoolId: 7, UserId: 1, Type: QuotaPoolTransactionAllocateAuto, Amount: 900, CreatedAt: 150},
	}).Error)
	require.NoError(t, logDB.Create(&Log{UserId: 1, Type: LogTypeTopup, Quota: 100, CreatedAt: 100, Other: `{"recharge_source":"auto","quota_pool_id":7}`}).Error)
	stats, err := GetQuotaPoolStats(7, 100, 200, QuotaPoolStatsGranularityDay)
	require.NoError(t, err)
	require.Len(t, stats.Members, 2)
	assert.EqualValues(t, 2, stats.Members[0].AutoRechargeCount)
	assert.EqualValues(t, 1, stats.Members[0].ManualRechargeCount)
	assert.EqualValues(t, 350, stats.Members[0].RechargeAmount)
	assert.EqualValues(t, 0, stats.Members[1].AutoRechargeCount)
	assert.EqualValues(t, 1, stats.Members[1].ManualRechargeCount)
	assert.EqualValues(t, 30, stats.Members[1].RechargeAmount)
	assert.False(t, stats.Members[1].Active)
}

func TestDefaultPoolMemberStatsCountsSystemAutoRechargeFromLogDatabase(t *testing.T) {
	db, logDB := setupICodeStatsTest(t)
	require.NoError(t, db.Create(&QuotaPool{Id: 7, Name: "default", PoolType: QuotaPoolTypeDefault, IsDefault: true, Enabled: true}).Error)
	require.NoError(t, db.Create(&User{Id: 1, Username: "default-member", AffCode: "default-stat", QuotaPoolId: 0}).Error)
	require.NoError(t, logDB.Create(&[]Log{
		{UserId: 1, Type: LogTypeTopup, Quota: 10, CreatedAt: 100, Other: `{"recharge_source":"auto","quota_pool_id":0}`},
		{UserId: 1, Type: LogTypeTopup, Quota: 20, CreatedAt: 200, Other: `{"quota_pool_id":0,"recharge_source":"auto"}`},
		{UserId: 1, Type: LogTypeSystem, Quota: 30, CreatedAt: 150, Content: "系统自动赠送 $30"},
		{UserId: 1, Type: LogTypeTopup, Quota: 900, CreatedAt: 99, Content: "系统自动赠送 $900"},
		{UserId: 1, Type: LogTypeTopup, Quota: 900, CreatedAt: 201, Content: "系统自动赠送 $900"},
		{UserId: 1, Type: LogTypeTopup, Quota: 900, CreatedAt: 150, Other: `{"recharge_source":"auto","quota_pool_id":8}`},
		{UserId: 1, Type: LogTypeConsume, Quota: 900, CreatedAt: 150, Content: "系统自动赠送 $900"},
		{UserId: 1, Type: LogTypeTopup, Quota: 900, CreatedAt: 150, Content: "manual recharge"},
		{UserId: 2, Type: LogTypeTopup, Quota: 900, CreatedAt: 150, Content: "系统自动赠送 $900"},
	}).Error)
	stats, err := GetQuotaPoolStats(7, 100, 200, QuotaPoolStatsGranularityDay)
	require.NoError(t, err)
	require.Len(t, stats.Members, 1)
	assert.EqualValues(t, 3, stats.Members[0].AutoRechargeCount)
	assert.Zero(t, stats.Members[0].ManualRechargeCount)
	assert.EqualValues(t, 60, stats.Members[0].RechargeAmount)
}
