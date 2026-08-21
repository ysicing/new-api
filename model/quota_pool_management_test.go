package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreateQuotaPoolWritesInitialFundAndRejectsDuplicateName(t *testing.T) {
	db := setupQuotaPoolFundsTestDB(t)

	pool, err := CreateQuotaPool(" 研发二组 ", 1000, 7)

	require.NoError(t, err)
	assert.Equal(t, "研发二组", pool.Name)
	assert.Equal(t, 1000, pool.BaseQuota)
	assert.Equal(t, 1000, pool.Quota)
	var transaction QuotaPoolTransaction
	require.NoError(t, db.Where("pool_id = ?", pool.Id).First(&transaction).Error)
	assert.Equal(t, QuotaPoolTransactionInitialFund, transaction.Type)
	assert.Equal(t, 1000, transaction.Amount)
	assert.Equal(t, 7, transaction.OperatorId)

	_, err = CreateQuotaPool("研发二组", 2000, 7)
	require.ErrorIs(t, err, ErrQuotaPoolNameExists)
}

func TestAddQuotaPoolManualRefillEnforcesAmountAndMonthlyLimit(t *testing.T) {
	db := setupQuotaPoolFundsTestDB(t)
	pool, _ := seedQuotaPoolMember(t, db, 1000, 0)

	first, err := AddQuotaPoolManualRefill(pool.Id, 500, 8)
	require.NoError(t, err)
	assert.Equal(t, 1500, first.QuotaAfter)
	_, err = AddQuotaPoolManualRefill(pool.Id, 750, 8)
	require.NoError(t, err, "second refill can use the updated base quota")
	_, err = AddQuotaPoolManualRefill(pool.Id, 100, 8)
	require.ErrorIs(t, err, ErrQuotaPoolRefillLimited)

	var transactions int64
	require.NoError(t, db.Model(&QuotaPoolTransaction{}).
		Where("pool_id = ? AND type = ?", pool.Id, QuotaPoolTransactionManualRefill).
		Count(&transactions).Error)
	assert.EqualValues(t, 2, transactions)
}

func TestGrantQuotaPoolAdminSupportsV1AndV2ForPoolMembers(t *testing.T) {
	db := setupQuotaPoolFundsTestDB(t)
	pool, member := seedQuotaPoolMember(t, db, 1000, 0)
	outsider := User{Username: "outsider", Password: "password", AffCode: "outsider-code", Role: common.RoleCommonUser, Status: common.UserStatusEnabled}
	require.NoError(t, db.Create(&outsider).Error)

	require.NoError(t, GrantQuotaPoolAdmin(pool.Id, member.Id, QuotaPoolAdminLevelV2))
	summary, err := GetQuotaPoolAdminSummary(member.Id)
	require.NoError(t, err)
	require.NotNil(t, summary)
	assert.Equal(t, QuotaPoolAdminLevelV2, summary.Level)
	assert.Equal(t, pool.Id, summary.PoolId)

	err = GrantQuotaPoolAdmin(pool.Id, outsider.Id, QuotaPoolAdminLevelV1)
	require.ErrorIs(t, err, ErrQuotaPoolMemberMismatch)
}

func TestUpdateQuotaPoolConfigAdjustsAvailableQuotaWithBaseQuota(t *testing.T) {
	db := setupQuotaPoolFundsTestDB(t)
	pool, _ := seedQuotaPoolMember(t, db, 1000, 0)
	require.NoError(t, db.Model(&pool).Update("quota", 400).Error)

	change, err := UpdateQuotaPoolConfig(pool.Id, map[string]any{"base_quota": 1200, "weekly_limit": 3}, 7)

	require.NoError(t, err)
	require.NotNil(t, change)
	assert.Equal(t, 600, change.QuotaAfter)
	require.NoError(t, db.First(&pool, pool.Id).Error)
	assert.Equal(t, 1200, pool.BaseQuota)
	assert.Equal(t, 600, pool.Quota)
	assert.Equal(t, 3, pool.WeeklyLimit)
}

func TestDeleteQuotaPoolRejectsSystemPoolsAndPoolsWithMembers(t *testing.T) {
	db := setupQuotaPoolFundsTestDB(t)
	pool, user := seedQuotaPoolMember(t, db, 1000, 0)
	systemPool := QuotaPool{Name: QuotaPoolNewUserName, PoolType: QuotaPoolTypeNewUser, Enabled: true, BaseQuota: -1, Quota: -1}
	require.NoError(t, db.Create(&systemPool).Error)

	require.ErrorIs(t, DeleteQuotaPool(systemPool.Id), ErrQuotaPoolSystemReadonly)
	require.ErrorIs(t, DeleteQuotaPool(pool.Id), ErrQuotaPoolHasMembers)
	require.NoError(t, db.Model(&user).Update("quota_pool_id", 0).Error)
	require.NoError(t, DeleteQuotaPool(pool.Id))
}
