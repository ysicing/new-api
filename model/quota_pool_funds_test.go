package model

import (
	"fmt"
	"math"
	"strconv"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/operation_setting"
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
	previousMain, previousLog := common.MainDatabaseType(), common.LogDatabaseType()
	common.SetDatabaseTypes(common.DatabaseTypeSQLite, common.DatabaseTypeSQLite)
	DB = db
	common.RedisEnabled = false
	common.BatchUpdateEnabled = false
	t.Cleanup(func() {
		DB = previousDB
		common.RedisEnabled = previousRedis
		common.BatchUpdateEnabled = previousBatch
		common.SetDatabaseTypes(previousMain, previousLog)
		sqlDB, err := db.DB()
		require.NoError(t, err)
		require.NoError(t, sqlDB.Close())
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

func TestLegacyAllocationRollsBackWhenItsTransactionCannotBeRecorded(t *testing.T) {
	db := setupQuotaPoolFundsTestDB(t)
	previous := common.QuotaPoolEnabled
	common.QuotaPoolEnabled = true
	t.Cleanup(func() { common.QuotaPoolEnabled = previous })
	pool := QuotaPool{Name: QuotaPoolDefaultName, PoolType: QuotaPoolTypeNormal, LegacyDefault: true, Enabled: true, Quota: 100, BaseQuota: 100}
	require.NoError(t, db.Create(&pool).Error)
	user := User{Username: "legacy-rollback", AffCode: "legacy-rollback", Quota: 10, QuotaPoolId: pool.Id}
	require.NoError(t, db.Create(&user).Error)
	require.NoError(t, db.Migrator().DropTable(&QuotaPoolTransaction{}))
	_, err := AllocateQuotaFromPool(pool.Id, user.Id, 40, QuotaPoolTransactionAllocateAuto, 0)
	require.Error(t, err)
	require.NoError(t, db.First(&user, user.Id).Error)
	assert.Equal(t, 10, user.Quota)
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

func TestAllocateQuotaFromPoolRejectsFrozenUserWithoutDebitingPool(t *testing.T) {
	db := setupQuotaPoolFundsTestDB(t)
	pool, user := seedQuotaPoolMember(t, db, 100, 10)
	require.NoError(t, db.Model(&User{}).Where("id = ?", user.Id).Update("quota_frozen", true).Error)

	_, err := AllocateQuotaFromPool(pool.Id, user.Id, 40, QuotaPoolTransactionAllocateManual, 9)
	require.ErrorContains(t, err, QuotaFrozenMessage)
	require.NoError(t, db.First(&pool, pool.Id).Error)
	require.NoError(t, db.First(&user, user.Id).Error)
	assert.Equal(t, 100, pool.Quota)
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
	require.NoError(t, db.Create(&QuotaPoolAdmin{PoolId: oldPool.Id, UserId: user.Id, Level: QuotaPoolAdminLevel}).Error)

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

	target := QuotaPool{Name: "target-negative", PoolType: QuotaPoolTypeNormal, Enabled: true}
	require.NoError(t, db.Create(&target).Error)
	result, err := MoveUserBetweenQuotaPools(user.Id, target.Id, true, 7)

	require.NoError(t, err)
	assert.False(t, result.Reclaimed)
	require.NoError(t, db.First(&oldPool, oldPool.Id).Error)
	require.NoError(t, db.First(&user, user.Id).Error)
	assert.Equal(t, 100, oldPool.Quota)
	assert.Zero(t, user.Quota)
	assert.Equal(t, target.Id, user.QuotaPoolId)
}

func TestMoveRootUserQuotaPoolPreservesRoleAndReclaimsDisabledPoolBalance(t *testing.T) {
	db := setupQuotaPoolFundsTestDB(t)
	source := QuotaPool{Name: "停用来源池", PoolType: QuotaPoolTypeNormal, Enabled: false, BaseQuota: 100, Quota: 100}
	target := QuotaPool{Name: "目标池", PoolType: QuotaPoolTypeNormal, Enabled: true, BaseQuota: 100, Quota: 100}
	require.NoError(t, db.Create(&source).Error)
	require.NoError(t, db.Model(&source).Update("enabled", false).Error)
	require.NoError(t, db.Create(&target).Error)
	root := User{
		Username: "root-target", Password: "password", AffCode: "root-target",
		Role: common.RoleRootUser, Status: common.UserStatusEnabled, Quota: 20, QuotaPoolId: source.Id,
	}
	require.NoError(t, db.Create(&root).Error)

	result, err := MoveUserBetweenQuotaPools(root.Id, target.Id, true, root.Id)

	require.NoError(t, err)
	assert.True(t, result.Reclaimed)
	require.NoError(t, db.First(&root, root.Id).Error)
	assert.Equal(t, common.RoleRootUser, root.Role)
	assert.Equal(t, target.Id, root.QuotaPoolId)
	assert.Zero(t, root.Quota)
	require.NoError(t, db.First(&source, source.Id).Error)
	assert.Equal(t, 120, source.Quota)
	require.NoError(t, db.First(&target, target.Id).Error)
	assert.Equal(t, 100, target.Quota)
	var transaction QuotaPoolTransaction
	require.NoError(t, db.Where("type = ?", QuotaPoolTransactionReclaimUser).First(&transaction).Error)
	assert.Equal(t, 20, transaction.Amount)
	assert.Equal(t, root.Id, transaction.UserId)
	assert.Equal(t, root.Id, transaction.OperatorId)
}

func TestMoveRootUserQuotaPoolRejectsNonRootOperator(t *testing.T) {
	for _, scenario := range []struct {
		name         string
		operatorRole int
		enabled      bool
		wantErr      error
	}{
		{name: "system admin cannot move root", operatorRole: common.RoleAdminUser, enabled: true, wantErr: ErrQuotaPoolPermissionDenied},
		{name: "system admin cannot move root into disabled pool", operatorRole: common.RoleAdminUser, enabled: false, wantErr: ErrQuotaPoolPermissionDenied},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			db := setupQuotaPoolFundsTestDB(t)
			source, root := seedQuotaPoolMember(t, db, 100, 20)
			require.NoError(t, db.Model(&root).Update("role", common.RoleRootUser).Error)
			target := QuotaPool{Name: "目标池", PoolType: QuotaPoolTypeNormal, Enabled: scenario.enabled, Quota: 100}
			require.NoError(t, db.Create(&target).Error)
			require.NoError(t, db.Model(&target).Update("enabled", scenario.enabled).Error)
			operator := User{Username: "operator", AffCode: "operator", Role: scenario.operatorRole, Status: common.UserStatusEnabled}
			require.NoError(t, db.Create(&operator).Error)

			_, err := MoveUserBetweenQuotaPools(root.Id, target.Id, true, operator.Id)

			require.ErrorIs(t, err, scenario.wantErr)
			require.NoError(t, db.First(&root, root.Id).Error)
			assert.Equal(t, source.Id, root.QuotaPoolId)
			assert.Equal(t, 20, root.Quota)
			assert.Equal(t, common.RoleRootUser, root.Role)
			require.NoError(t, db.First(&source, source.Id).Error)
			assert.Equal(t, 100, source.Quota)
			var count int64
			require.NoError(t, db.Model(&QuotaPoolTransaction{}).Count(&count).Error)
			assert.Zero(t, count)
		})
	}
}

func TestMoveUserQuotaPoolAllowsDisabledTargetWithoutGrantingQuota(t *testing.T) {
	for _, role := range []int{common.RoleCommonUser, common.RoleQuotaPoolSuperAdmin, common.RoleAdminUser, common.RoleRootUser} {
		t.Run(fmt.Sprint(role), func(t *testing.T) {
			db := setupQuotaPoolFundsTestDB(t)
			source, user := seedQuotaPoolMember(t, db, 100, 20)
			require.NoError(t, db.Model(&source).Update("enabled", false).Error)
			require.NoError(t, db.Model(&user).Update("role", role).Error)
			target := QuotaPool{Name: "禁用目标池", PoolType: QuotaPoolTypeNormal, Quota: 200}
			require.NoError(t, db.Create(&target).Error)
			require.NoError(t, db.Model(&target).Update("enabled", false).Error)

			result, err := MoveUserBetweenQuotaPools(user.Id, target.Id, true, user.Id)

			require.NoError(t, err)
			assert.True(t, result.Reclaimed)
			require.NoError(t, db.First(&user, user.Id).Error)
			assert.Equal(t, target.Id, user.QuotaPoolId)
			assert.Equal(t, role, user.Role)
			assert.Zero(t, user.Quota)
			require.NoError(t, db.First(&source, source.Id).Error)
			assert.Equal(t, 120, source.Quota)
			require.NoError(t, db.First(&target, target.Id).Error)
			assert.False(t, target.Enabled)
			assert.Equal(t, 200, target.Quota)
			for _, kind := range []string{QuotaPoolTransactionAllocateManual, QuotaPoolTransactionAllocateAuto} {
				_, err = AllocateQuotaFromPool(target.Id, user.Id, 10, kind, user.Id)
				require.ErrorIs(t, err, ErrQuotaPoolDisabled)
			}
			var transactions []QuotaPoolTransaction
			require.NoError(t, db.Find(&transactions).Error)
			require.Len(t, transactions, 1)
			assert.Equal(t, QuotaPoolTransactionReclaimUser, transactions[0].Type)
			assert.Equal(t, 20, transactions[0].Amount)
		})
	}
}

func TestRootOperatorMovesRootWithoutOriginalPool(t *testing.T) {
	db := setupQuotaPoolFundsTestDB(t)
	target := QuotaPool{Name: "root-target-pool", PoolType: QuotaPoolTypeNormal, Enabled: true, Quota: 100}
	require.NoError(t, db.Create(&target).Error)
	operator := User{Username: "root-operator", AffCode: "root-operator", Role: common.RoleRootUser, Status: common.UserStatusEnabled}
	root := User{Username: "root-member", AffCode: "root-member", Role: common.RoleRootUser, Status: common.UserStatusEnabled, Quota: 20}
	require.NoError(t, db.Create(&operator).Error)
	require.NoError(t, db.Create(&root).Error)

	result, err := MoveUserBetweenQuotaPools(root.Id, target.Id, true, operator.Id)

	require.NoError(t, err)
	assert.False(t, result.Reclaimed)
	require.NoError(t, db.First(&root, root.Id).Error)
	assert.Equal(t, common.RoleRootUser, root.Role)
	assert.Equal(t, target.Id, root.QuotaPoolId)
	assert.Zero(t, root.Quota)
	require.NoError(t, db.First(&target, target.Id).Error)
	assert.Equal(t, 100, target.Quota)
	var count int64
	require.NoError(t, db.Model(&QuotaPoolTransaction{}).Count(&count).Error)
	assert.Zero(t, count)
}

func TestAllocateQuotaFromDisabledPoolRejectsRootUser(t *testing.T) {
	db := setupQuotaPoolFundsTestDB(t)
	pool, root := seedQuotaPoolMember(t, db, 100, 20)
	require.NoError(t, db.Model(&root).Update("role", common.RoleRootUser).Error)
	require.NoError(t, db.Model(&pool).Update("enabled", false).Error)

	_, err := AllocateQuotaFromPool(pool.Id, root.Id, 10, QuotaPoolTransactionAllocateManual, root.Id)

	require.ErrorIs(t, err, ErrQuotaPoolDisabled)
	require.NoError(t, db.First(&pool, pool.Id).Error)
	require.NoError(t, db.First(&root, root.Id).Error)
	assert.Equal(t, 100, pool.Quota)
	assert.Equal(t, 20, root.Quota)
	var count int64
	require.NoError(t, db.Model(&QuotaPoolTransaction{}).Count(&count).Error)
	assert.Zero(t, count)
}

func TestAddUserToQuotaPoolRejectsIneligibleCandidates(t *testing.T) {
	db := setupQuotaPoolFundsTestDB(t)
	target := QuotaPool{Name: "目标池", PoolType: QuotaPoolTypeNormal, Enabled: true, BaseQuota: 100, Quota: 100}
	otherPool := QuotaPool{Name: "其他池", PoolType: QuotaPoolTypeNormal, Enabled: true, BaseQuota: 100, Quota: 100}
	require.NoError(t, db.Create(&target).Error)
	require.NoError(t, db.Create(&otherPool).Error)
	users := []User{
		{Username: "disabled-candidate", Password: "password", AffCode: "disabled-candidate", Role: common.RoleCommonUser, Status: common.UserStatusDisabled, Quota: 20},
		{Username: "root-candidate", Password: "password", AffCode: "root-candidate", Role: common.RoleRootUser, Status: common.UserStatusEnabled, Quota: 20},
		{Username: "other-pool-candidate", Password: "password", AffCode: "other-pool-candidate", Role: common.RoleCommonUser, Status: common.UserStatusEnabled, Quota: 20, QuotaPoolId: otherPool.Id},
	}
	require.NoError(t, db.Create(&users).Error)

	for index := range users[:2] {
		_, err := AddUserToQuotaPool(users[index].Id, target.Id, 7)
		assert.ErrorIs(t, err, ErrQuotaPoolCandidateInvalid)
	}
	_, err := AddUserToQuotaPool(users[2].Id, target.Id, 7)
	assert.ErrorIs(t, err, ErrQuotaPoolCandidateInvalid)

	for index := range users {
		var reloaded User
		require.NoError(t, db.First(&reloaded, users[index].Id).Error)
		assert.Equal(t, users[index].QuotaPoolId, reloaded.QuotaPoolId)
		assert.Equal(t, 20, reloaded.Quota)
	}
}

func TestAddUserToQuotaPoolOnlyAcceptsNewUserPoolSources(t *testing.T) {
	db := setupQuotaPoolFundsTestDB(t)
	target := QuotaPool{Name: "目标池", PoolType: QuotaPoolTypeNormal, Enabled: true, BaseQuota: 100, Quota: 100}
	newUserPool := QuotaPool{Name: QuotaPoolNewUserName, PoolType: QuotaPoolTypeNewUser, Enabled: true, BaseQuota: -1, Quota: -1}
	otherPool := QuotaPool{Name: "其他池", PoolType: QuotaPoolTypeNormal, Enabled: true, BaseQuota: 100, Quota: 100}
	require.NoError(t, db.Create(&target).Error)
	require.NoError(t, db.Create(&newUserPool).Error)
	require.NoError(t, db.Create(&otherPool).Error)
	users := []User{
		{Username: "new-user-source", Password: "password", AffCode: "new-user-source", Role: common.RoleCommonUser, Status: common.UserStatusEnabled, Quota: 20, QuotaPoolId: newUserPool.Id},
		{Username: "legacy-default-source", Password: "password", AffCode: "legacy-default-source", Role: common.RoleCommonUser, Status: common.UserStatusEnabled, Quota: 20},
		{Username: "normal-pool-source", Password: "password", AffCode: "normal-pool-source", Role: common.RoleCommonUser, Status: common.UserStatusEnabled, Quota: 20, QuotaPoolId: otherPool.Id},
	}
	require.NoError(t, db.Create(&users).Error)

	_, err := AddUserToQuotaPool(users[1].Id, target.Id, 7)
	assert.ErrorIs(t, err, ErrQuotaPoolCandidateInvalid)
	_, err = AddUserToQuotaPool(users[2].Id, target.Id, 7)
	assert.ErrorIs(t, err, ErrQuotaPoolCandidateInvalid)
	result, err := AddUserToQuotaPool(users[0].Id, target.Id, 7)

	require.NoError(t, err)
	assert.Equal(t, newUserPool.Id, result.OldPoolId)
	assert.Equal(t, target.Id, result.NewPoolId)
}

func TestReclaimQuotaToPoolUpdatesBalancesAndTransaction(t *testing.T) {
	db := setupQuotaPoolFundsTestDB(t)
	pool, user := seedQuotaPoolMember(t, db, 100, 80)

	change, err := ReclaimQuotaToPool(pool.Id, user.Id, 30, 6)

	require.NoError(t, err)
	assert.Equal(t, 130, change.QuotaAfter)
	require.NoError(t, db.First(&pool, pool.Id).Error)
	require.NoError(t, db.First(&user, user.Id).Error)
	assert.Equal(t, 130, pool.Quota)
	assert.Equal(t, 50, user.Quota)
	var transaction QuotaPoolTransaction
	require.NoError(t, db.Where("type = ?", QuotaPoolTransactionReclaimUser).First(&transaction).Error)
	assert.Equal(t, 30, transaction.Amount)
}

func TestQuotaPoolWalletBoundaries(t *testing.T) {
	db := setupQuotaPoolFundsTestDB(t)
	verifyQuotaPoolWalletBoundaries(t, db)
}

func verifyQuotaPoolWalletBoundaries(t *testing.T, db *gorm.DB) {
	t.Helper()
	previousDB, previousRedis := DB, common.RedisEnabled
	DB, common.RedisEnabled = db, false
	t.Cleanup(func() { DB, common.RedisEnabled = previousDB, previousRedis })
	pool, err := CreateQuotaPool("wallet-boundary", common.MaxWalletQuota, 9)
	require.NoError(t, err)
	user := User{
		Username: "wallet-boundary", AffCode: "wallet-boundary",
		Role: common.RoleQuotaPoolSuperAdmin, Status: common.UserStatusEnabled, QuotaPoolId: pool.Id,
	}
	require.NoError(t, db.Create(&user).Error)

	allocation, err := AllocateQuotaFromPool(pool.Id, user.Id, common.MaxWalletQuota, QuotaPoolTransactionAllocateManual, 9)
	require.NoError(t, err)
	assert.Zero(t, allocation.QuotaAfter)
	reclaim, err := ReclaimQuotaToPool(pool.Id, user.Id, common.MaxWalletQuota, 9)
	require.NoError(t, err)
	assert.Equal(t, common.MaxWalletQuota, reclaim.QuotaAfter)
	require.NoError(t, db.Model(&User{}).Where("id = ?", user.Id).Update("quota", 1).Error)
	var beforeCount int64
	require.NoError(t, db.Model(&QuotaPoolTransaction{}).Count(&beforeCount).Error)
	_, err = ReclaimQuotaToPool(pool.Id, user.Id, 1, 9)
	require.ErrorIs(t, err, ErrWalletQuotaLimitExceeded)
	target, err := CreateQuotaPool("wallet-boundary-target", 100, 9)
	require.NoError(t, err)
	_, err = MoveUserBetweenQuotaPools(user.Id, target.Id, false, 9)
	require.ErrorIs(t, err, ErrWalletQuotaLimitExceeded)
	require.NoError(t, db.First(&user, user.Id).Error)
	assert.Equal(t, pool.Id, user.QuotaPoolId)
	assert.Equal(t, 1, user.Quota)
	require.NoError(t, db.First(pool, pool.Id).Error)
	assert.Equal(t, common.MaxWalletQuota, pool.Quota)
	var afterCount int64
	require.NoError(t, db.Model(&QuotaPoolTransaction{}).Count(&afterCount).Error)
	assert.Equal(t, beforeCount+1, afterCount, "只有创建目标池写入流水")

	require.NoError(t, db.Model(&User{}).Where("id = ?", user.Id).Update("quota", common.MaxWalletQuota).Error)
	_, err = AllocateQuotaFromPool(pool.Id, user.Id, 1, QuotaPoolTransactionAllocateAuto, 0)
	require.ErrorIs(t, err, ErrWalletQuotaLimitExceeded)
	require.NoError(t, db.First(pool, pool.Id).Error)
	assert.Equal(t, common.MaxWalletQuota, pool.Quota)
	require.NoError(t, db.First(&user, user.Id).Error)
	assert.Equal(t, common.MaxWalletQuota, user.Quota)
	require.NoError(t, db.Model(&QuotaPoolTransaction{}).Count(&afterCount).Error)
	assert.Equal(t, beforeCount+1, afterCount)

	require.NoError(t, db.Model(pool).Updates(map[string]any{
		"quota": common.MaxWalletQuota - 1, "base_quota": common.MaxWalletQuota - 1,
		"monthly_refill_enabled": true, "monthly_refill_top_up": true,
		"monthly_refill_amount": common.MaxWalletQuota, "monthly_refill_day": 1,
	}).Error)
	refilled, err := RefillMonthlyQuotaPool(pool.Id, 202611, 9)
	require.NoError(t, err)
	assert.True(t, refilled)
	refilled, err = RefillMonthlyQuotaPool(pool.Id, 202611, 9)
	require.NoError(t, err)
	assert.False(t, refilled)
	require.NoError(t, db.First(pool, pool.Id).Error)
	assert.Equal(t, common.MaxWalletQuota, pool.Quota)
	assert.Equal(t, common.MaxWalletQuota, pool.BaseQuota)
	require.NoError(t, db.Model(&QuotaPoolTransaction{}).Count(&afterCount).Error)
	assert.Equal(t, beforeCount+2, afterCount)
}

func TestQuotaPoolRejectsInvalidIngressWithoutWriting(t *testing.T) {
	for _, amount := range []int{math.MinInt, -1, 0, common.MaxWalletQuota + 1, math.MaxInt} {
		t.Run(fmt.Sprint(amount), func(t *testing.T) {
			db := setupQuotaPoolFundsTestDB(t)
			pool, user := seedQuotaPoolMember(t, db, 100, 80)
			_, err := CreateQuotaPool("invalid", amount, 9)
			require.ErrorIs(t, err, ErrQuotaPoolInvalidAmount)
			_, err = AllocateQuotaFromPool(pool.Id, user.Id, amount, QuotaPoolTransactionAllocateManual, 9)
			require.ErrorIs(t, err, ErrQuotaPoolInvalidAmount)
			_, err = ReclaimQuotaToPool(pool.Id, user.Id, amount, 9)
			require.ErrorIs(t, err, ErrQuotaPoolInvalidAmount)
			_, err = AddQuotaPoolManualRefill(pool.Id, amount, 9)
			require.ErrorIs(t, err, ErrQuotaPoolInvalidAmount)
			_, err = DeductQuotaPoolBalance(pool.Id, amount, 9)
			require.ErrorIs(t, err, ErrQuotaPoolInvalidAmount)
			require.ErrorIs(t, CreditAutoRechargeUserQuota(user.Id, amount), ErrQuotaPoolInvalidAmount)
			require.NoError(t, db.First(&pool, pool.Id).Error)
			require.NoError(t, db.First(&user, user.Id).Error)
			assert.Equal(t, 100, pool.Quota)
			assert.Equal(t, 80, user.Quota)
			var count int64
			require.NoError(t, db.Model(&QuotaPoolTransaction{}).Count(&count).Error)
			assert.Zero(t, count)
		})
	}
}

func TestQuotaPoolRejectsPolicyAndRefillOverflowAtomically(t *testing.T) {
	db := setupQuotaPoolFundsTestDB(t)
	pool, _ := seedQuotaPoolMember(t, db, common.MaxWalletQuota, 0)
	for _, field := range []string{"base_quota", "auto_recharge_amount", "weekly_limit", "monthly_limit", "monthly_refill_amount"} {
		_, _, err := UpdateQuotaPoolConfig(pool.Id, map[string]any{field: common.MaxWalletQuota + 1}, 9)
		require.ErrorIs(t, err, ErrQuotaPoolInvalidAmount, field)
	}
	_, err := AddQuotaPoolManualRefill(pool.Id, 1, 9)
	require.ErrorIs(t, err, ErrWalletQuotaLimitExceeded)
	require.NoError(t, db.Model(&pool).Updates(map[string]any{
		"base_quota": 100, "monthly_refill_enabled": true, "monthly_refill_amount": 1, "monthly_refill_day": 1,
	}).Error)
	_, _, err = UpdateQuotaPoolConfig(pool.Id, map[string]any{"base_quota": 101}, 9)
	require.ErrorIs(t, err, ErrWalletQuotaLimitExceeded)
	refilled, err := RefillMonthlyQuotaPool(pool.Id, 202611, 9)
	require.ErrorIs(t, err, ErrWalletQuotaLimitExceeded)
	assert.False(t, refilled)
	require.NoError(t, db.First(&pool, pool.Id).Error)
	assert.Equal(t, common.MaxWalletQuota, pool.Quota)
	assert.Equal(t, 100, pool.BaseQuota)
	assert.Zero(t, pool.LastRefillMonth)
	require.NoError(t, db.Model(&pool).Updates(map[string]any{"quota": 100, "base_quota": common.MaxWalletQuota}).Error)
	refilled, err = RefillMonthlyQuotaPool(pool.Id, 202611, 9)
	require.ErrorIs(t, err, ErrWalletQuotaLimitExceeded, "累计基础资金也必须检查上界")
	assert.False(t, refilled)
	var count int64
	require.NoError(t, db.Model(&QuotaPoolTransaction{}).Count(&count).Error)
	assert.Zero(t, count)
}

func TestQuotaPoolRejectsInvalidStoredBalancesBeforeArithmetic(t *testing.T) {
	for _, quota := range []int{math.MinInt, -1, common.MaxWalletQuota + 1, math.MaxInt} {
		t.Run(fmt.Sprint(quota), func(t *testing.T) {
			db := setupQuotaPoolFundsTestDB(t)
			pool, user := seedQuotaPoolMember(t, db, 100, 80)
			require.NoError(t, db.Model(&pool).Update("quota", quota).Error)
			_, err := AllocateQuotaFromPool(pool.Id, user.Id, 1, QuotaPoolTransactionAllocateManual, 9)
			require.ErrorIs(t, err, ErrQuotaPoolInvalidAmount)
			_, err = ReclaimQuotaToPool(pool.Id, user.Id, 1, 9)
			require.ErrorIs(t, err, ErrQuotaPoolInvalidAmount)
			_, err = AddQuotaPoolManualRefill(pool.Id, 1, 9)
			require.ErrorIs(t, err, ErrQuotaPoolInvalidAmount)
			_, err = DeductQuotaPoolBalance(pool.Id, 1, 9)
			require.ErrorIs(t, err, ErrQuotaPoolInvalidAmount)
			_, _, err = UpdateQuotaPoolConfig(pool.Id, map[string]any{"base_quota": 101}, 9)
			require.ErrorIs(t, err, ErrQuotaPoolInvalidAmount)
			require.NoError(t, db.First(&pool, pool.Id).Error)
			require.NoError(t, db.First(&user, user.Id).Error)
			assert.Equal(t, quota, pool.Quota)
			assert.Equal(t, 80, user.Quota)
			var count int64
			require.NoError(t, db.Model(&QuotaPoolTransaction{}).Count(&count).Error)
			assert.Zero(t, count)
		})
	}
}

func TestQuotaPoolFrozenReclaimAndUpstreamRefundRemainAvailable(t *testing.T) {
	db := setupQuotaPoolFundsTestDB(t)
	pool, user := seedQuotaPoolMember(t, db, 100, 80)
	require.NoError(t, db.Model(&user).Update("quota_frozen", true).Error)
	require.NoError(t, SetQuotaPoolEnabled(pool.Id, false))
	_, err := ReclaimQuotaToPool(pool.Id, user.Id, 30, 9)
	require.NoError(t, err)
	require.ErrorIs(t, CreditAutoRechargeUserQuota(user.Id, 10), ErrQuotaFrozen)
	// 已结算退款仍沿上游钱包路径；冻结不能把它误当作新增拨款拒绝。
	require.NoError(t, IncreaseUserQuota(user.Id, 10, true))
	require.NoError(t, db.First(&user, user.Id).Error)
	assert.Equal(t, 60, user.Quota)
	assert.True(t, user.QuotaFrozen)
	target := QuotaPool{Name: "frozen-move-target", PoolType: QuotaPoolTypeNormal, Enabled: false}
	require.NoError(t, db.Create(&target).Error)
	result, err := MoveUserBetweenQuotaPools(user.Id, target.Id, false, 9)
	require.NoError(t, err)
	assert.True(t, result.Reclaimed)
	require.NoError(t, db.First(&user, user.Id).Error)
	require.NoError(t, db.First(&pool, pool.Id).Error)
	assert.Zero(t, user.Quota)
	assert.True(t, user.QuotaFrozen)
	assert.Equal(t, 190, pool.Quota)
}

func TestAutoRechargeWalletConversionRejectsInvalidSettings(t *testing.T) {
	previousUnit := common.QuotaPerUnit
	t.Cleanup(func() { common.QuotaPerUnit = previousUnit })
	for _, scenario := range []struct {
		name      string
		amount    int
		unit      float64
		want      int
		wantError bool
	}{
		{name: "above int32 stays exact", amount: math.MaxInt32 + 1, unit: 1, want: math.MaxInt32 + 1},
		{name: "wallet ceiling", amount: common.MaxWalletQuota, unit: 1, want: common.MaxWalletQuota},
		{name: "retain truncation", amount: 1, unit: 1.9, want: 1},
		{name: "wallet overflow", amount: common.MaxWalletQuota, unit: 2, wantError: true},
		{name: "invalid negative", amount: -1, unit: 1, wantError: true},
		{name: "invalid integer", amount: math.MaxInt, unit: 1, wantError: true},
		{name: "nan unit", amount: 1, unit: math.NaN(), wantError: true},
		{name: "infinite unit", amount: 1, unit: math.Inf(1), wantError: true},
		{name: "zero unit", amount: 1, unit: 0, wantError: true},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			common.QuotaPerUnit = scenario.unit
			config := operation_setting.AutoRechargeSetting{Interval: 30, Threshold: scenario.amount, Amount: scenario.amount}
			threshold, amount, err := config.QuotaAmounts()
			if scenario.wantError {
				require.Error(t, err)
				assert.Zero(t, threshold)
				assert.Zero(t, amount)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, scenario.want, threshold)
			assert.Equal(t, scenario.want, amount)
		})
	}
}

func TestQuotaPoolListUsesStrictWalletAmounts(t *testing.T) {
	setupQuotaPoolFundsTestDB(t)
	pool, err := CreateQuotaPool("wallet-display", 100, 9)
	require.NoError(t, err)
	config := operation_setting.GetAutoRechargeSetting()
	previousConfig, previousUnit := *config, common.QuotaPerUnit
	t.Cleanup(func() {
		*config, common.QuotaPerUnit = previousConfig, previousUnit
	})
	common.QuotaPerUnit = 1
	for _, amount := range []int{math.MaxInt32 + 1, common.MaxWalletQuota, common.MaxWalletQuota + 1} {
		t.Run(strconv.Itoa(amount), func(t *testing.T) {
			*config = operation_setting.AutoRechargeSetting{Interval: 30, Threshold: amount, Amount: amount}
			item, err := GetQuotaPoolListItemById(pool.Id)
			if amount > common.MaxWalletQuota {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, amount, item.SystemAutoRecharge.Threshold)
			assert.Equal(t, amount, item.SystemAutoRecharge.Amount)
		})
	}
}

func TestQuotaPoolAutoRechargeOptionValidationRejectsUnsafeWrites(t *testing.T) {
	previousUnit := common.QuotaPerUnit
	t.Cleanup(func() { common.QuotaPerUnit = previousUnit })
	for _, scenario := range []struct {
		name      string
		key       string
		value     string
		unit      float64
		wantError bool
	}{
		{name: "valid enable", key: "enabled", value: "true", unit: 1},
		{name: "invalid boolean", key: "enabled", value: "maybe", unit: 1, wantError: true},
		{name: "negative amount", key: "amount", value: "-1", unit: 1, wantError: true},
		{name: "fractional amount", key: "amount", value: "1.5", unit: 1, wantError: true},
		{name: "nan amount", key: "amount", value: "NaN", unit: 1, wantError: true},
		{name: "integer overflow", key: "amount", value: "9223372036854775808", unit: 1, wantError: true},
		{name: "exact wallet ceiling", key: "amount", value: strconv.Itoa(common.MaxWalletQuota), unit: 1},
		{name: "converted wallet overflow", key: "amount", value: strconv.Itoa(common.MaxWalletQuota), unit: 2, wantError: true},
		{name: "threshold overflow", key: "threshold", value: strconv.Itoa(common.MaxWalletQuota + 1), unit: 1, wantError: true},
		{name: "negative weekly limit", key: "weekly_limit", value: "-1", unit: 1, wantError: true},
		{name: "monthly limit overflow", key: "monthly_limit", value: strconv.Itoa(common.MaxWalletQuota + 1), unit: 1, wantError: true},
		{name: "zero interval", key: "interval", value: "0", unit: 1, wantError: true},
		{name: "largest interval", key: "interval", value: strconv.FormatInt(math.MaxInt64/int64(time.Minute), 10), unit: 1},
		{name: "duration overflow", key: "interval", value: strconv.FormatInt(math.MaxInt64/int64(time.Minute)+1, 10), unit: 1, wantError: true},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			common.QuotaPerUnit = scenario.unit
			config := operation_setting.AutoRechargeSetting{Interval: 30, Threshold: 50, Amount: 200}
			before := config
			err := config.ValidateOption("auto_recharge_setting."+scenario.key, scenario.value)
			if scenario.wantError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			assert.Equal(t, before, config)
		})
	}
	common.QuotaPerUnit = 1
	invalid := operation_setting.AutoRechargeSetting{Enabled: true, Interval: -1, Amount: -1}
	require.NoError(t, invalid.ValidateOption("auto_recharge_setting.enabled", "false"))
	require.Error(t, invalid.ValidateOption("auto_recharge_setting.enabled", "true"))
}
