package model

import (
	"fmt"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupQuotaPoolFundsTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:quota-pool-funds-%s?mode=memory&cache=shared", t.Name())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&User{}, &QuotaPool{}, &QuotaPoolAdmin{}, &QuotaPoolTransaction{}))
	previousDB := DB
	previousRedis := common.RedisEnabled
	previousBatch := common.BatchUpdateEnabled
	common.SetDatabaseTypes(common.DatabaseTypeSQLite, common.DatabaseTypeSQLite)
	DB = db
	common.RedisEnabled = false
	common.BatchUpdateEnabled = false
	t.Cleanup(func() {
		DB = previousDB
		common.RedisEnabled = previousRedis
		common.BatchUpdateEnabled = previousBatch
	})
	return db
}

func seedQuotaPoolMember(t *testing.T, db *gorm.DB, poolQuota, userQuota int) (QuotaPool, User) {
	t.Helper()
	pool := QuotaPool{Name: "研发池", PoolType: QuotaPoolTypeNormal, Enabled: true, BaseQuota: poolQuota, Quota: poolQuota}
	require.NoError(t, db.Create(&pool).Error)
	user := User{Username: "pool-user", Password: "password", AffCode: "pool-user-code", Role: common.RoleCommonUser, Status: common.UserStatusEnabled, Quota: userQuota, QuotaPoolId: pool.Id}
	require.NoError(t, db.Create(&user).Error)
	return pool, user
}

func TestAllocateQuotaFromPoolUpdatesBalancesAndWritesTransactionAtomically(t *testing.T) {
	db := setupQuotaPoolFundsTestDB(t)
	pool, user := seedQuotaPoolMember(t, db, 100, 10)

	change, err := AllocateQuotaFromPool(pool.Id, user.Id, 40, QuotaPoolTransactionAllocateManual, 9)

	require.NoError(t, err)
	assert.Equal(t, QuotaPoolBalanceChange{PoolId: pool.Id, Amount: -40, QuotaBefore: 100, QuotaAfter: 60}, *change)
	require.NoError(t, db.First(&pool, pool.Id).Error)
	require.NoError(t, db.First(&user, user.Id).Error)
	assert.Equal(t, 60, pool.Quota)
	assert.Equal(t, 50, user.Quota)
	var transaction QuotaPoolTransaction
	require.NoError(t, db.First(&transaction).Error)
	assert.Equal(t, -40, transaction.Amount)
	assert.Equal(t, 9, transaction.OperatorId)
	assert.Equal(t, QuotaPoolTransactionAllocateManual, transaction.Type)
}

func TestAllocateQuotaFromPoolRejectsInsufficientBalanceWithoutPartialWrites(t *testing.T) {
	db := setupQuotaPoolFundsTestDB(t)
	pool, user := seedQuotaPoolMember(t, db, 30, 10)

	_, err := AllocateQuotaFromPool(pool.Id, user.Id, 40, QuotaPoolTransactionAllocateAuto, 0)

	require.ErrorIs(t, err, ErrQuotaPoolInsufficientQuota)
	require.NoError(t, db.First(&pool, pool.Id).Error)
	require.NoError(t, db.First(&user, user.Id).Error)
	assert.Equal(t, 30, pool.Quota)
	assert.Equal(t, 10, user.Quota)
	var count int64
	require.NoError(t, db.Model(&QuotaPoolTransaction{}).Count(&count).Error)
	assert.Zero(t, count)
}

func TestMoveUserQuotaPoolReclaimsBalanceAndRevokesOldAdmin(t *testing.T) {
	db := setupQuotaPoolFundsTestDB(t)
	oldPool, user := seedQuotaPoolMember(t, db, 100, 35)
	target := QuotaPool{Name: "目标池", PoolType: QuotaPoolTypeNormal, Enabled: true, BaseQuota: 200, Quota: 200}
	require.NoError(t, db.Create(&target).Error)
	require.NoError(t, db.Create(&QuotaPoolAdmin{PoolId: oldPool.Id, UserId: user.Id, Level: QuotaPoolAdminLevelV1}).Error)

	result, err := MoveUserBetweenQuotaPools(user.Id, target.Id, false, 7)

	require.NoError(t, err)
	assert.Equal(t, oldPool.Id, result.OldPoolId)
	assert.Equal(t, target.Id, result.NewPoolId)
	assert.True(t, result.Reclaimed)
	require.NoError(t, db.First(&oldPool, oldPool.Id).Error)
	require.NoError(t, db.First(&user, user.Id).Error)
	assert.Equal(t, 135, oldPool.Quota)
	assert.Zero(t, user.Quota)
	assert.Equal(t, target.Id, user.QuotaPoolId)
	var adminCount int64
	require.NoError(t, db.Model(&QuotaPoolAdmin{}).Where("user_id = ?", user.Id).Count(&adminCount).Error)
	assert.Zero(t, adminCount)
	var transaction QuotaPoolTransaction
	require.NoError(t, db.Where("type = ?", QuotaPoolTransactionReclaimUser).First(&transaction).Error)
	assert.Equal(t, 35, transaction.Amount)
}

func TestMoveUserQuotaPoolClearsNegativeBalanceWithoutCreditingOldPool(t *testing.T) {
	db := setupQuotaPoolFundsTestDB(t)
	oldPool, user := seedQuotaPoolMember(t, db, 100, -25)

	result, err := MoveUserBetweenQuotaPools(user.Id, QuotaPoolDefaultUserPoolId, true, 7)

	require.NoError(t, err)
	assert.False(t, result.Reclaimed)
	require.NoError(t, db.First(&oldPool, oldPool.Id).Error)
	require.NoError(t, db.First(&user, user.Id).Error)
	assert.Equal(t, 100, oldPool.Quota)
	assert.Zero(t, user.Quota)
	assert.Zero(t, user.QuotaPoolId)
}
