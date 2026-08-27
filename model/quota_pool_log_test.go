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
	assert.Equal(t, fmt.Sprintf("额度池“平台保障部”自动赠送 %s", logger.LogQuota(amount)), log.Content)
	assert.Contains(t, log.Other, `"amount":5000000`)
	assert.Contains(t, log.Other, `"quota_pool_name":"平台保障部"`)
}
