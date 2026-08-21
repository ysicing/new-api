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
