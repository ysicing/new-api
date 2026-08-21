package model

import (
	"fmt"
	"testing"

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

func TestSyncSystemQuotaPoolsCreatesOnlyMissingRows(t *testing.T) {
	db := openLegacyQuotaPoolFixture(t)
	require.NoError(t, migrateQuotaPoolSchema(db))

	require.NoError(t, syncSystemQuotaPools(db))
	require.NoError(t, syncSystemQuotaPools(db))

	var defaultPool QuotaPool
	require.NoError(t, db.Where("pool_type = ?", QuotaPoolTypeDefault).First(&defaultPool).Error)
	assert.Equal(t, 1, defaultPool.Id)
	assert.Equal(t, -1, defaultPool.BaseQuota, "existing system-pool data must not be normalized")
	assert.Equal(t, -1, defaultPool.Quota)

	var newUserPools int64
	require.NoError(t, db.Model(&QuotaPool{}).Where("pool_type = ?", QuotaPoolTypeNewUser).Count(&newUserPools).Error)
	assert.EqualValues(t, 1, newUserPools)

	var normalPool QuotaPool
	require.NoError(t, db.Where("id = ?", 2).First(&normalPool).Error)
	assert.Equal(t, "研发一组", normalPool.Name)
	assert.Equal(t, 4200, normalPool.Quota)
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
