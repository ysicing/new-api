package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRemoveQuotaPoolMemberReclaimsBalanceToNewUserPool(t *testing.T) {
	db := setupQuotaPoolFundsTestDB(t)
	source, user := seedQuotaPoolMember(t, db, 100, 35)
	target := QuotaPool{Name: QuotaPoolNewUserName, PoolType: QuotaPoolTypeNewUser, Enabled: true, BaseQuota: -1, Quota: -1}
	require.NoError(t, db.Create(&target).Error)

	result, err := RemoveQuotaPoolMember(QuotaPoolMemberRemoval{SourcePoolId: source.Id, UserId: user.Id, OperatorId: 7})

	require.NoError(t, err)
	assert.Equal(t, source.Id, result.OldPoolId)
	assert.Equal(t, target.Id, result.NewPoolId)
	assert.Equal(t, 35, result.Change.Amount)
	assert.False(t, result.AdminRevoked)
	require.NoError(t, db.First(&source, source.Id).Error)
	require.NoError(t, db.First(&user, user.Id).Error)
	assert.Equal(t, 135, source.Quota)
	assert.Zero(t, user.Quota)
	assert.Equal(t, target.Id, user.QuotaPoolId)
}

func TestRemoveQuotaPoolMemberRejectsPoolAdministrator(t *testing.T) {
	db := setupQuotaPoolFundsTestDB(t)
	source, user := seedQuotaPoolMember(t, db, 100, 35)
	require.NoError(t, db.Create(&QuotaPool{Name: QuotaPoolNewUserName, PoolType: QuotaPoolTypeNewUser, Enabled: true, BaseQuota: -1, Quota: -1}).Error)
	require.NoError(t, db.Create(&QuotaPoolAdmin{PoolId: source.Id, UserId: user.Id, Level: QuotaPoolAdminLevel}).Error)

	_, err := RemoveQuotaPoolMember(QuotaPoolMemberRemoval{SourcePoolId: source.Id, UserId: user.Id, OperatorId: 7})

	require.ErrorIs(t, err, ErrQuotaPoolPermissionDenied)
	require.NoError(t, db.First(&source, source.Id).Error)
	require.NoError(t, db.First(&user, user.Id).Error)
	assert.Equal(t, 100, source.Quota)
	assert.Equal(t, 35, user.Quota)
	assert.Equal(t, source.Id, user.QuotaPoolId)
}

func TestRemoveQuotaPoolMemberRejectsPrivilegedUserForPoolAdministrator(t *testing.T) {
	db := setupQuotaPoolFundsTestDB(t)
	source, user := seedQuotaPoolMember(t, db, 100, 35)
	require.NoError(t, db.Model(&user).Update("role", common.RoleAdminUser).Error)
	require.NoError(t, db.Create(&QuotaPool{Name: QuotaPoolNewUserName, PoolType: QuotaPoolTypeNewUser, Enabled: true, BaseQuota: -1, Quota: -1}).Error)

	_, err := RemoveQuotaPoolMember(QuotaPoolMemberRemoval{SourcePoolId: source.Id, UserId: user.Id, OperatorId: 7})

	require.ErrorIs(t, err, ErrQuotaPoolPermissionDenied)
	require.NoError(t, db.First(&user, user.Id).Error)
	assert.Equal(t, source.Id, user.QuotaPoolId)
	assert.Equal(t, 35, user.Quota)
}

func TestRemoveQuotaPoolMemberAllowsGlobalAdministratorToRemovePoolAdministrator(t *testing.T) {
	db := setupQuotaPoolFundsTestDB(t)
	source, user := seedQuotaPoolMember(t, db, 100, 35)
	target := QuotaPool{Name: QuotaPoolNewUserName, PoolType: QuotaPoolTypeNewUser, Enabled: true, BaseQuota: -1, Quota: -1}
	require.NoError(t, db.Create(&target).Error)
	require.NoError(t, db.Create(&QuotaPoolAdmin{PoolId: source.Id, UserId: user.Id, Level: QuotaPoolAdminLevel}).Error)

	result, err := RemoveQuotaPoolMember(QuotaPoolMemberRemoval{SourcePoolId: source.Id, UserId: user.Id, OperatorId: 7, AllowAdminRemoval: true})

	require.NoError(t, err)
	assert.True(t, result.AdminRevoked)
	var adminCount int64
	require.NoError(t, db.Model(&QuotaPoolAdmin{}).Where("user_id = ?", user.Id).Count(&adminCount).Error)
	assert.Zero(t, adminCount)
	require.NoError(t, db.First(&user, user.Id).Error)
	assert.Equal(t, target.Id, user.QuotaPoolId)
}

func TestRemoveQuotaPoolMemberClearsNegativeBalanceWithoutTransaction(t *testing.T) {
	db := setupQuotaPoolFundsTestDB(t)
	source, user := seedQuotaPoolMember(t, db, 100, -25)
	target := QuotaPool{Name: QuotaPoolNewUserName, PoolType: QuotaPoolTypeNewUser, Enabled: true, BaseQuota: -1, Quota: -1}
	require.NoError(t, db.Create(&target).Error)

	result, err := RemoveQuotaPoolMember(QuotaPoolMemberRemoval{SourcePoolId: source.Id, UserId: user.Id, OperatorId: 7})

	require.NoError(t, err)
	assert.False(t, result.Reclaimed)
	require.NoError(t, db.First(&source, source.Id).Error)
	require.NoError(t, db.First(&user, user.Id).Error)
	assert.Equal(t, 100, source.Quota)
	assert.Zero(t, user.Quota)
	assert.Equal(t, target.Id, user.QuotaPoolId)
	var transactionCount int64
	require.NoError(t, db.Model(&QuotaPoolTransaction{}).Count(&transactionCount).Error)
	assert.Zero(t, transactionCount)
}

func TestRemoveQuotaPoolMemberRejectsWrongSourceWithoutWrites(t *testing.T) {
	db := setupQuotaPoolFundsTestDB(t)
	source, user := seedQuotaPoolMember(t, db, 100, 35)
	other := QuotaPool{Name: "其他池", PoolType: QuotaPoolTypeNormal, Enabled: true, BaseQuota: 50, Quota: 50}
	require.NoError(t, db.Create(&other).Error)
	require.NoError(t, db.Create(&QuotaPool{Name: QuotaPoolNewUserName, PoolType: QuotaPoolTypeNewUser, Enabled: true, BaseQuota: -1, Quota: -1}).Error)

	_, err := RemoveQuotaPoolMember(QuotaPoolMemberRemoval{SourcePoolId: other.Id, UserId: user.Id, OperatorId: 7})

	require.ErrorIs(t, err, ErrQuotaPoolMemberMismatch)
	require.NoError(t, db.First(&source, source.Id).Error)
	require.NoError(t, db.First(&user, user.Id).Error)
	assert.Equal(t, 100, source.Quota)
	assert.Equal(t, 35, user.Quota)
	assert.Equal(t, source.Id, user.QuotaPoolId)
}

func TestRemoveQuotaPoolMemberRequiresEnabledNewUserPool(t *testing.T) {
	t.Run("missing", func(t *testing.T) {
		db := setupQuotaPoolFundsTestDB(t)
		source, user := seedQuotaPoolMember(t, db, 100, 35)
		_, err := RemoveQuotaPoolMember(QuotaPoolMemberRemoval{SourcePoolId: source.Id, UserId: user.Id, OperatorId: 7})
		require.ErrorIs(t, err, ErrQuotaPoolNotFound)
	})
	t.Run("disabled", func(t *testing.T) {
		db := setupQuotaPoolFundsTestDB(t)
		source, user := seedQuotaPoolMember(t, db, 100, 35)
		target := QuotaPool{Name: QuotaPoolNewUserName, PoolType: QuotaPoolTypeNewUser, Enabled: true, BaseQuota: -1, Quota: -1}
		require.NoError(t, db.Create(&target).Error)
		require.NoError(t, db.Model(&target).Update("enabled", false).Error)
		_, err := RemoveQuotaPoolMember(QuotaPoolMemberRemoval{SourcePoolId: source.Id, UserId: user.Id, OperatorId: 7})
		require.ErrorIs(t, err, ErrQuotaPoolDisabled)
	})
}
