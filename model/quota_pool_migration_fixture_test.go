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

type legacyQuotaPoolUser struct {
	Id          int `gorm:"primaryKey"`
	Quota       int
	UsedQuota   int
	QuotaPoolId int
	LDAPId      string
	Department  string
}

func (legacyQuotaPoolUser) TableName() string { return "users" }

type legacyQuotaPool struct {
	Id        int `gorm:"primaryKey"`
	Name      string
	PoolType  string
	BaseQuota int
	Quota     int
}

func (legacyQuotaPool) TableName() string { return "quota_pools" }

type legacyQuotaPoolAdmin struct {
	Id     int `gorm:"primaryKey"`
	PoolId int
	UserId int
	Level  int
}

func (legacyQuotaPoolAdmin) TableName() string { return "quota_pool_admins" }

type legacyQuotaPoolTransaction struct {
	Id     int `gorm:"primaryKey"`
	PoolId int
	Amount int
}

func (legacyQuotaPoolTransaction) TableName() string { return "quota_pool_transactions" }

type legacyQuotaPoolSnapshot struct {
	UserCount        int64
	UserQuota        int64
	UserUsedQuota    int64
	PoolCount        int64
	PoolQuota        int64
	PoolBaseQuota    int64
	AdminCount       int64
	TransactionCount int64
	TransactionSum   int64
}

func openLegacyQuotaPoolFixture(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:legacy-quota-pool-%s?mode=memory&cache=shared", t.Name())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&legacyQuotaPoolUser{},
		&legacyQuotaPool{},
		&legacyQuotaPoolAdmin{},
		&legacyQuotaPoolTransaction{},
	))
	require.NoError(t, db.Create(&[]legacyQuotaPoolUser{
		{Id: 1, Quota: 500, UsedQuota: 120, QuotaPoolId: 0},
		{Id: 2, Quota: 80, UsedQuota: 900, QuotaPoolId: 2, LDAPId: "alice@example.com", Department: "R&D"},
	}).Error)
	require.NoError(t, db.Create(&[]legacyQuotaPool{
		{Id: 1, Name: "产研中心默认额度池(存量)", PoolType: "default", BaseQuota: -1, Quota: -1},
		{Id: 2, Name: "研发一组", PoolType: "normal", BaseQuota: 10000, Quota: 4200},
	}).Error)
	require.NoError(t, db.Create(&legacyQuotaPoolAdmin{Id: 1, PoolId: 2, UserId: 2, Level: 1}).Error)
	require.NoError(t, db.Create(&[]legacyQuotaPoolTransaction{
		{Id: 1, PoolId: 2, Amount: 10000},
		{Id: 2, PoolId: 2, Amount: -5800},
	}).Error)
	return db
}

func captureLegacyQuotaPoolSnapshot(t *testing.T, db *gorm.DB) legacyQuotaPoolSnapshot {
	t.Helper()
	var snapshot legacyQuotaPoolSnapshot
	require.NoError(t, db.Table("users").Count(&snapshot.UserCount).Error)
	require.NoError(t, db.Table("users").Select("COALESCE(SUM(quota), 0)").Scan(&snapshot.UserQuota).Error)
	require.NoError(t, db.Table("users").Select("COALESCE(SUM(used_quota), 0)").Scan(&snapshot.UserUsedQuota).Error)
	require.NoError(t, db.Table("quota_pools").Count(&snapshot.PoolCount).Error)
	require.NoError(t, db.Table("quota_pools").Select("COALESCE(SUM(quota), 0)").Scan(&snapshot.PoolQuota).Error)
	require.NoError(t, db.Table("quota_pools").Select("COALESCE(SUM(base_quota), 0)").Scan(&snapshot.PoolBaseQuota).Error)
	require.NoError(t, db.Table("quota_pool_admins").Count(&snapshot.AdminCount).Error)
	require.NoError(t, db.Table("quota_pool_transactions").Count(&snapshot.TransactionCount).Error)
	require.NoError(t, db.Table("quota_pool_transactions").Select("COALESCE(SUM(amount), 0)").Scan(&snapshot.TransactionSum).Error)
	return snapshot
}

func TestLegacyQuotaPoolFixtureSnapshot(t *testing.T) {
	db := openLegacyQuotaPoolFixture(t)

	got := captureLegacyQuotaPoolSnapshot(t, db)

	assert.Equal(t, legacyQuotaPoolSnapshot{
		UserCount:        2,
		UserQuota:        580,
		UserUsedQuota:    1020,
		PoolCount:        2,
		PoolQuota:        4199,
		PoolBaseQuota:    9999,
		AdminCount:       1,
		TransactionCount: 2,
		TransactionSum:   4200,
	}, got)
}

func TestMigrateQuotaPoolSchemaPreservesLegacyDataAndIsIdempotent(t *testing.T) {
	db := openLegacyQuotaPoolFixture(t)
	before := captureLegacyQuotaPoolSnapshot(t, db)

	require.NoError(t, migrateQuotaPoolSchema(db))
	require.NoError(t, migrateQuotaPoolSchema(db))

	after := captureLegacyQuotaPoolSnapshot(t, db)
	assert.Equal(t, before, after)
	assert.True(t, db.Migrator().HasColumn(&QuotaPool{}, "monthly_refill_top_up"))
	assert.True(t, db.Migrator().HasColumn(&QuotaPool{}, "last_refill_month"))
	assert.True(t, db.Migrator().HasColumn(&QuotaPoolTransaction{}, "operator_id"))
}

func TestMigrateQuotaPoolSchemaNormalizesAdminLevelsIdempotently(t *testing.T) {
	db := openLegacyQuotaPoolFixture(t)
	require.NoError(t, db.Model(&legacyQuotaPoolAdmin{}).
		Where("id = ?", 1).Update("level", 2).Error)

	require.NoError(t, migrateQuotaPoolSchema(db))
	require.NoError(t, migrateQuotaPoolSchema(db))

	var admin QuotaPoolAdmin
	require.NoError(t, db.Where("id = ?", 1).First(&admin).Error)
	assert.Equal(t, 1, admin.Level)
}

func TestSyncSystemQuotaPoolsMigratesLegacyPoolWithoutChangingMembersQuota(t *testing.T) {
	db := openLegacyQuotaPoolFixture(t)
	require.NoError(t, migrateQuotaPoolSchema(db))

	require.NoError(t, syncSystemQuotaPools(db))
	require.NoError(t, syncSystemQuotaPools(db))

	var defaultPool QuotaPool
	require.NoError(t, db.Where("id = ?", 1).First(&defaultPool).Error)
	assert.Equal(t, 1, defaultPool.Id)
	assert.Zero(t, defaultPool.BaseQuota)
	assert.Zero(t, defaultPool.Quota)
	assert.Equal(t, QuotaPoolTypeNormal, defaultPool.PoolType)
	assert.False(t, defaultPool.IsDefault)
	assert.True(t, defaultPool.LegacyDefault)
	var member legacyQuotaPoolUser
	require.NoError(t, db.First(&member, 1).Error)
	assert.Equal(t, defaultPool.Id, member.QuotaPoolId)
	assert.Equal(t, 500, member.Quota)
	assert.Equal(t, 120, member.UsedQuota)
	// 管理员配置资金后，再次同步不得归零，也不改动原普通池。
	require.NoError(t, db.Model(&defaultPool).Updates(map[string]any{"base_quota": 1000, "quota": 800}).Error)
	require.NoError(t, syncSystemQuotaPools(db))
	require.NoError(t, db.First(&defaultPool, defaultPool.Id).Error)
	assert.Equal(t, 1000, defaultPool.BaseQuota)
	assert.Equal(t, 800, defaultPool.Quota)

	var newUserPools int64
	require.NoError(t, db.Model(&QuotaPool{}).Where("pool_type = ?", QuotaPoolTypeNewUser).Count(&newUserPools).Error)
	assert.EqualValues(t, 1, newUserPools)

	var normalPool QuotaPool
	require.NoError(t, db.Where("id = ?", 2).First(&normalPool).Error)
	assert.Equal(t, "研发一组", normalPool.Name)
	assert.Equal(t, 4200, normalPool.Quota)
}

func TestSyncSystemQuotaPoolsRenamesLegacyNewUserPool(t *testing.T) {
	dsn := fmt.Sprintf("file:rename-new-user-pool-%s?mode=memory&cache=shared", t.Name())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&QuotaPool{}, &quotaPoolUserCompatibilityColumns{}))
	legacy := QuotaPool{
		Name: "默认额度池", PoolType: QuotaPoolTypeNewUser, Enabled: true,
		BaseQuota: QuotaPoolUnlimitedQuota, Quota: QuotaPoolUnlimitedQuota,
	}
	require.NoError(t, db.Create(&legacy).Error)

	require.NoError(t, syncSystemQuotaPools(db))

	var stored QuotaPool
	require.NoError(t, db.First(&stored, legacy.Id).Error)
	assert.Equal(t, "新用户额度池", stored.Name)
}

func TestMigrateQuotaPoolSchemaAddsUserCompatibilityColumns(t *testing.T) {
	dsn := fmt.Sprintf("file:quota-pool-user-columns-%s?mode=memory&cache=shared", t.Name())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.Exec("CREATE TABLE users (id integer PRIMARY KEY)").Error)

	require.NoError(t, migrateQuotaPoolSchema(db))

	assert.True(t, db.Migrator().HasColumn("users", "quota_pool_id"))
	assert.True(t, db.Migrator().HasColumn("users", "ldap_id"))
	assert.True(t, db.Migrator().HasColumn("users", "department"))
}

func TestMigrateQuotaPoolSchemaPersistsLegacyAutoRechargeDefaults(t *testing.T) {
	db := openLegacyQuotaPoolFixture(t)
	require.NoError(t, db.AutoMigrate(&Option{}))

	require.NoError(t, migrateQuotaPoolSchema(db))

	var enabled Option
	require.NoError(t, db.Where("key = ?", "auto_recharge_setting.enabled").First(&enabled).Error)
	assert.Equal(t, "true", enabled.Value)
	var amount Option
	require.NoError(t, db.Where("key = ?", "auto_recharge_setting.amount").First(&amount).Error)
	assert.Equal(t, "200", amount.Value)
}

func TestMigrateQuotaPoolSchemaLeavesFreshInstallAutoRechargeDisabled(t *testing.T) {
	dsn := fmt.Sprintf("file:fresh-quota-pool-%s?mode=memory&cache=shared", t.Name())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&Option{}))

	require.NoError(t, migrateQuotaPoolSchema(db))

	var count int64
	require.NoError(t, db.Model(&Option{}).Where("key LIKE ?", "auto_recharge_setting.%").Count(&count).Error)
	assert.Zero(t, count)
}

func TestMigrateQuotaPoolSchemaPreservesStoredAutoRechargeSetting(t *testing.T) {
	db := openLegacyQuotaPoolFixture(t)
	require.NoError(t, db.AutoMigrate(&Option{}))
	require.NoError(t, db.Create(&Option{Key: "auto_recharge_setting.enabled", Value: "false"}).Error)

	require.NoError(t, migrateQuotaPoolSchema(db))

	var enabled Option
	require.NoError(t, db.Where("key = ?", "auto_recharge_setting.enabled").First(&enabled).Error)
	assert.Equal(t, "false", enabled.Value)
}

func TestSyncSystemQuotaPoolsCreatesLegacyPoolOnlyForUnassignedMembers(t *testing.T) {
	db := setupQuotaPoolFundsTestDB(t)
	require.NoError(t, syncSystemQuotaPools(db))
	var pools []QuotaPool
	require.NoError(t, db.Find(&pools).Error)
	require.Len(t, pools, 1)
	assert.True(t, pools[0].IsNewUserPool())

	// 迁移已注销的成员，避免恢复账号后仍使用虚拟 ID；账户状态和余额不受影响。
	user := User{Username: "legacy-deleted", AffCode: "legacy-deleted", Quota: 40, UsedQuota: 80, QuotaFrozen: true}
	require.NoError(t, db.Create(&user).Error)
	require.NoError(t, db.Delete(&user).Error)
	require.NoError(t, syncSystemQuotaPools(db))
	require.NoError(t, db.Unscoped().First(&user, user.Id).Error)
	assert.Positive(t, user.QuotaPoolId)
	assert.Equal(t, 40, user.Quota)
	assert.Equal(t, 80, user.UsedQuota)
	assert.True(t, user.QuotaFrozen)
	assert.True(t, user.DeletedAt.Valid)
	var pool QuotaPool
	require.NoError(t, db.First(&pool, user.QuotaPoolId).Error)
	assert.Equal(t, QuotaPoolTypeNormal, pool.PoolType)
	assert.Zero(t, pool.BaseQuota)
	assert.Zero(t, pool.Quota)
}

func TestLegacyPoolMigrationRollsBackPoolConversionWhenMembershipUpdateFails(t *testing.T) {
	db := setupQuotaPoolFundsTestDB(t)
	pool := QuotaPool{Name: QuotaPoolDefaultName, PoolType: QuotaPoolTypeDefault, Enabled: true, IsDefault: true, BaseQuota: -1, Quota: -1}
	require.NoError(t, db.Create(&pool).Error)
	user := User{Username: "legacy-rollback", AffCode: "migration-rollback", Quota: 40}
	require.NoError(t, db.Create(&user).Error)
	require.NoError(t, db.Callback().Update().Before("gorm:update").Register("fail_membership_migration", func(tx *gorm.DB) {
		if tx.Statement.Table == "users" {
			tx.AddError(ErrQuotaPoolMemberMismatch)
		}
	}))
	t.Cleanup(func() { db.Callback().Update().Remove("fail_membership_migration") })

	require.ErrorIs(t, syncSystemQuotaPools(db), ErrQuotaPoolMemberMismatch)
	require.NoError(t, db.First(&pool, pool.Id).Error)
	require.NoError(t, db.First(&user, user.Id).Error)
	assert.Equal(t, QuotaPoolTypeDefault, pool.PoolType)
	assert.True(t, pool.IsDefault)
	assert.False(t, pool.LegacyDefault)
	assert.Equal(t, -1, pool.Quota)
	assert.Equal(t, -1, pool.BaseQuota)
	assert.Zero(t, user.QuotaPoolId)
	assert.Equal(t, 40, user.Quota)
}

func TestMigratedLegacyPoolSupportsOrdinaryAccountingAndDeletion(t *testing.T) {
	db := setupQuotaPoolFundsTestDB(t)
	pool := QuotaPool{Name: QuotaPoolDefaultName, PoolType: QuotaPoolTypeDefault, Enabled: true, IsDefault: true, BaseQuota: -1, Quota: -1}
	require.NoError(t, db.Create(&pool).Error)
	user := User{Username: "legacy-member", AffCode: "migration-accounting", Role: common.RoleCommonUser, Status: common.UserStatusEnabled, Quota: 20}
	require.NoError(t, db.Create(&user).Error)
	require.NoError(t, syncSystemQuotaPools(db))
	require.NoError(t, db.First(&pool, pool.Id).Error)
	require.NoError(t, db.First(&user, user.Id).Error)
	assert.Equal(t, pool.Id, user.QuotaPoolId)
	assert.Equal(t, 20, user.Quota)
	_, err := AllocateQuotaFromPool(pool.Id, user.Id, 10, QuotaPoolTransactionAllocateManual, 9)
	require.ErrorIs(t, err, ErrQuotaPoolInsufficientQuota)
	_, err = AllocateQuotaFromPool(0, user.Id, 10, QuotaPoolTransactionAllocateManual, 9)
	require.ErrorIs(t, err, ErrQuotaPoolNotFound)

	_, _, err = UpdateQuotaPoolConfig(pool.Id, map[string]any{"base_quota": 100, "monthly_refill_enabled": true, "monthly_refill_amount": 50}, 9)
	require.NoError(t, err)
	require.NoError(t, GrantQuotaPoolAdmin(pool.Id, user.Id))
	_, err = AllocateQuotaFromPool(pool.Id, user.Id, 10, QuotaPoolTransactionAllocateManual, 9)
	require.NoError(t, err)
	require.ErrorIs(t, DeleteQuotaPool(pool.Id), ErrQuotaPoolHasMembers)
	result, err := RemoveQuotaPoolMember(QuotaPoolMemberRemoval{SourcePoolId: pool.Id, UserId: user.Id, OperatorId: 9, AllowAdminRemoval: true})
	require.NoError(t, err)
	assert.True(t, result.AdminRevoked)
	assert.Equal(t, 30, result.Change.Amount)
	require.NoError(t, db.First(&pool, pool.Id).Error)
	assert.Equal(t, 120, pool.Quota)
	assert.True(t, pool.MonthlyRefillEnabled)
	assert.Equal(t, 50, pool.MonthlyRefillAmount)
	require.NoError(t, DeleteQuotaPool(pool.Id))
	require.NoError(t, syncSystemQuotaPools(db))
	var remaining int64
	require.NoError(t, db.Model(&QuotaPool{}).Where("pool_type <> ?", QuotaPoolTypeNewUser).Count(&remaining).Error)
	assert.Zero(t, remaining, "同步不得重新创建已删除的普通存量池")
}

func TestLegacyPoolMigrationPreservesFiniteFundsPolicyAndAdministrator(t *testing.T) {
	db := setupQuotaPoolFundsTestDB(t)
	pool := QuotaPool{
		Name: "configured-legacy", PoolType: QuotaPoolTypeDefault, IsDefault: true,
		BaseQuota: 100, Quota: 70, AutoRechargeAmount: 10, WeeklyLimit: 2, MonthlyLimit: 4,
	}
	require.NoError(t, db.Create(&pool).Error)
	require.NoError(t, db.Model(&pool).Update("enabled", false).Error)
	user := User{Username: "legacy-admin", AffCode: "migration-admin", Quota: 30}
	require.NoError(t, db.Create(&user).Error)
	admin := QuotaPoolAdmin{PoolId: pool.Id, UserId: user.Id, Level: QuotaPoolAdminLevel}
	require.NoError(t, db.Create(&admin).Error)

	require.NoError(t, syncSystemQuotaPools(db))
	require.NoError(t, db.First(&pool, pool.Id).Error)
	assert.Equal(t, QuotaPoolTypeNormal, pool.PoolType)
	assert.False(t, pool.Enabled)
	assert.Equal(t, 100, pool.BaseQuota)
	assert.Equal(t, 70, pool.Quota)
	assert.Equal(t, 10, pool.AutoRechargeAmount)
	assert.Equal(t, 2, pool.WeeklyLimit)
	assert.Equal(t, 4, pool.MonthlyLimit)
	require.NoError(t, db.First(&admin, admin.Id).Error)
	assert.Equal(t, pool.Id, admin.PoolId)
	assert.Equal(t, user.Id, admin.UserId)
}
