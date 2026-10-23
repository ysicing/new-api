package model

import (
	"fmt"
	"testing"

	"github.com/QuantumNous/new-api/logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRecordAutoRechargeLogFormatsQuotaAndSnapshotsPoolName(t *testing.T) {
	mainDB, logDB := setupICodeStatsTest(t)
	user := User{Id: 1, Username: "alice", Password: "password", AffCode: "auto-recharge-log"}
	require.NoError(t, mainDB.Create(&user).Error)
	const amount = 5_000_000

	require.NoError(t, RecordAutoRechargeLog(user.Id, 7, amount, 0, "平台保障部"))

	var log Log
	require.NoError(t, logDB.Order("id DESC").First(&log).Error)
	assert.Equal(t, LogTypeTopup, log.Type)
	assert.Equal(t, fmt.Sprintf("额度池“平台保障部”自动赠送 %s", logger.LogQuota(amount)), log.Content)
	assert.Contains(t, log.Other, `"amount":5000000`)
	assert.Contains(t, log.Other, `"quota_pool_name":"平台保障部"`)
}

func TestCountAutoRechargeLogsSupportsLegacyAndTopupTypes(t *testing.T) {
	_, logDB := setupICodeStatsTest(t)
	const userId = 7
	const since = 100
	require.NoError(t, logDB.Create(&[]Log{
		{UserId: userId, Type: LogTypeSystem, CreatedAt: 110, Content: "系统自动赠送 100"},
		{UserId: userId, Type: LogTypeTopup, CreatedAt: 120, Other: `{"recharge_source":"auto"}`},
		{UserId: userId, Type: LogTypeTopup, CreatedAt: 130, Other: `{"op":{"action":"quota_pool.member_recharge"}}`},
		{UserId: 8, Type: LogTypeTopup, CreatedAt: 140, Other: `{"recharge_source":"auto"}`},
	}).Error)

	count, err := CountAutoRechargeLogs(userId, since)

	require.NoError(t, err)
	assert.EqualValues(t, 2, count)
}
