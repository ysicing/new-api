package model

import (
	"os"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func TestQuotaPoolLegacyMigrationAcrossExternalDatabases(t *testing.T) {
	if os.Getenv("TEST_QUOTA_POOL_DESTRUCTIVE") != "1" {
		t.Skip("set TEST_QUOTA_POOL_DESTRUCTIVE=1 with dedicated test databases")
	}
	tests := []struct {
		name      string
		dsn       string
		dialector gorm.Dialector
		dbType    common.DatabaseType
	}{
		{name: "mysql", dsn: os.Getenv("TEST_MYSQL_DSN"), dialector: mysql.Open(os.Getenv("TEST_MYSQL_DSN")), dbType: common.DatabaseTypeMySQL},
		{name: "postgres", dsn: os.Getenv("TEST_POSTGRES_DSN"), dialector: postgres.Open(os.Getenv("TEST_POSTGRES_DSN")), dbType: common.DatabaseTypePostgreSQL},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if test.dsn == "" {
				t.Skip("dedicated DSN not configured")
			}
			db, err := gorm.Open(test.dialector, &gorm.Config{})
			require.NoError(t, err)
			common.SetDatabaseTypes(test.dbType, test.dbType)
			dropQuotaPoolMigrationFixture(t, db)
			t.Cleanup(func() { dropQuotaPoolMigrationFixture(t, db) })
			seedLegacyQuotaPoolFixture(t, db)
			before := captureLegacyQuotaPoolSnapshot(t, db)

			require.NoError(t, migrateQuotaPoolSchema(db))
			require.NoError(t, migrateQuotaPoolSchema(db))

			after := captureLegacyQuotaPoolSnapshot(t, db)
			assert.Equal(t, before, after)
			assert.True(t, db.Migrator().HasColumn(&QuotaPool{}, "monthly_refill_top_up"))
		})
	}
}

func seedLegacyQuotaPoolFixture(t *testing.T, db *gorm.DB) {
	t.Helper()
	require.NoError(t, db.AutoMigrate(&legacyQuotaPoolUser{}, &legacyQuotaPool{}, &legacyQuotaPoolAdmin{}, &legacyQuotaPoolTransaction{}, &Option{}))
	require.NoError(t, db.Create(&[]legacyQuotaPoolUser{{Id: 1, Quota: 500, UsedQuota: 120}, {Id: 2, Quota: 80, UsedQuota: 900, QuotaPoolId: 2, LDAPId: "alice@example.com", Department: "R&D"}}).Error)
	require.NoError(t, db.Create(&[]legacyQuotaPool{{Id: 1, Name: QuotaPoolDefaultName, PoolType: QuotaPoolTypeDefault, BaseQuota: -1, Quota: -1}, {Id: 2, Name: "研发一组", PoolType: QuotaPoolTypeNormal, BaseQuota: 10000, Quota: 4200}}).Error)
	require.NoError(t, db.Create(&legacyQuotaPoolAdmin{Id: 1, PoolId: 2, UserId: 2, Level: 1}).Error)
	require.NoError(t, db.Create(&[]legacyQuotaPoolTransaction{{Id: 1, PoolId: 2, Amount: 10000}, {Id: 2, PoolId: 2, Amount: -5800}}).Error)
}

func dropQuotaPoolMigrationFixture(t *testing.T, db *gorm.DB) {
	t.Helper()
	require.NoError(t, db.Migrator().DropTable("quota_pool_transactions", "quota_pool_admins", "quota_pools", "users", "options"))
}
