package model

import (
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestDisabledQuotaPoolAllowsReclaimButRejectsAllocation(t *testing.T) {
	db := setupQuotaPoolFundsTestDB(t)
	pool, user := seedQuotaPoolMember(t, db, 100, 80)
	require.NoError(t, SetQuotaPoolEnabled(pool.Id, false))
	_, err := ReclaimQuotaToPool(pool.Id, user.Id, 30, 6)
	require.NoError(t, err)
	for _, operation := range []string{QuotaPoolTransactionAllocateManual, QuotaPoolTransactionAllocateAuto} {
		_, err = AllocateQuotaFromPool(pool.Id, user.Id, 10, operation, 6)
		require.ErrorIs(t, err, ErrQuotaPoolDisabled)
	}
	_, err = ReclaimQuotaToPool(pool.Id, user.Id, 51, 6)
	require.ErrorIs(t, err, ErrQuotaPoolInsufficientQuota)
	require.NoError(t, db.First(&user, user.Id).Error)
	require.NoError(t, db.First(&pool, pool.Id).Error)
	assert.Equal(t, 50, user.Quota)
	assert.Equal(t, 130, pool.Quota)
	var transactions []QuotaPoolTransaction
	require.NoError(t, db.Find(&transactions).Error)
	require.Len(t, transactions, 1)
	assert.Equal(t, QuotaPoolTransactionReclaimUser, transactions[0].Type)
	assert.Equal(t, 30, transactions[0].Amount)
}
