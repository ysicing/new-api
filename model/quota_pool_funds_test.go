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

func TestMoveRootUserQuotaPoolRejectsNonRootOperatorAndDisabledTarget(t *testing.T) {
	for _, scenario := range []struct {
		name         string
		operatorRole int
		enabled      bool
		wantErr      error
	}{
		{name: "system admin cannot move root", operatorRole: common.RoleAdminUser, enabled: true, wantErr: ErrQuotaPoolPermissionDenied},
		{name: "root cannot enter disabled pool", operatorRole: common.RoleRootUser, enabled: false, wantErr: ErrQuotaPoolDisabled},
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
