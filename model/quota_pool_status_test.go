package model

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSetQuotaPoolEnabledSupportsSystemPools(t *testing.T) {
	for _, poolType := range []string{QuotaPoolTypeNormal, QuotaPoolTypeDefault, QuotaPoolTypeNewUser} {
		t.Run(poolType, func(t *testing.T) {
			db := setupQuotaPoolFundsTestDB(t)
			pool := QuotaPool{Name: poolType, PoolType: poolType, Enabled: true, IsDefault: poolType == QuotaPoolTypeDefault}
			require.NoError(t, db.Create(&pool).Error)
			for _, enabled := range []bool{false, true} {
				require.NoError(t, SetQuotaPoolEnabled(pool.Id, enabled))
				require.NoError(t, SyncSystemQuotaPools())
				require.NoError(t, db.First(&pool, pool.Id).Error)
				assert.Equal(t, enabled, pool.Enabled)
			}
			if pool.IsSystemPool() {
				require.ErrorIs(t, DeleteQuotaPool(pool.Id), ErrQuotaPoolSystemReadonly)
			}
		})
	}
}
