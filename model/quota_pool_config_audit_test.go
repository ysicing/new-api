package model

import (
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestUpdateQuotaPoolConfigSnapshotsOnlyActualChanges(t *testing.T) {
	db := setupQuotaPoolFundsTestDB(t)
	pool, _ := seedQuotaPoolMember(t, db, 1000, 0)
	require.NoError(t, db.Model(&pool).Updates(map[string]any{"auto_recharge_amount": -1, "weekly_limit": 2, "monthly_limit": 0}).Error)
	updates := map[string]any{"name": " 研发池 ", "auto_recharge_amount": 500, "weekly_limit": 2, "monthly_limit": 5}
	_, changes, err := UpdateQuotaPoolConfig(pool.Id, updates, 7)
	require.NoError(t, err)
	assert.Equal(t, []QuotaPoolConfigChange{
		{Field: "auto_recharge_amount", Before: -1, After: 500},
		{Field: "monthly_limit", Before: 0, After: 5},
	}, changes)
	_, changes, err = UpdateQuotaPoolConfig(pool.Id, updates, 7)
	require.NoError(t, err)
	assert.Empty(t, changes)
	_, changes, err = UpdateQuotaPoolConfig(pool.Id, map[string]any{"base_quota": 0, "weekly_limit": 4}, 7)
	require.Error(t, err)
	assert.Nil(t, changes)
	require.NoError(t, db.First(&pool, pool.Id).Error)
	assert.Equal(t, 2, pool.WeeklyLimit)
}
