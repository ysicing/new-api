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

func TestQuotaPoolWalletOperationsDatabaseMatrix(t *testing.T) {
	for _, dialect := range []string{"sqlite", "mysql", "postgres"} {
		t.Run(dialect, func(t *testing.T) {
			db, logDB := setupOperationsDatabaseMatrix(t, dialect)
			require.NoError(t, db.AutoMigrate(&SubscriptionPlan{}, &SubscriptionOrder{}, &UserSubscription{}))
			previousPoolEnabled, previousQuotaPerUnit := common.QuotaPoolEnabled, common.QuotaPerUnit
			common.QuotaPoolEnabled, common.QuotaPerUnit = true, 100
			t.Cleanup(func() {
				common.QuotaPoolEnabled, common.QuotaPerUnit = previousPoolEnabled, previousQuotaPerUnit
			})

			for _, test := range []struct {
				name   string
				frozen bool
				price  float64
				quota  int
			}{
				{name: "frozen_paid", frozen: true, price: 1, quota: 500},
				{name: "frozen_free", frozen: true, quota: 500},
				{name: "frozen_insufficient_balance", frozen: true, price: 1},
				{name: "active_paid", price: 1, quota: 500},
				{name: "active_free", quota: 500},
			} {
				t.Run("subscription_balance/"+test.name, func(t *testing.T) {
					user := User{
						Username: test.name, Password: "password", AffCode: test.name,
						Role: common.RoleCommonUser, Status: common.UserStatusEnabled,
						Group: "default", Quota: test.quota, QuotaFrozen: test.frozen,
					}
					require.NoError(t, db.Create(&user).Error)
					plan := SubscriptionPlan{
						Title: test.name, Enabled: true, PriceAmount: test.price,
						DurationUnit: SubscriptionDurationMonth, DurationValue: 1,
						TotalAmount: 1000, UpgradeGroup: "pro",
						AllowBalancePay: common.GetPointer(true),
					}
					require.NoError(t, db.Create(&plan).Error)
					InvalidateSubscriptionPlanCache(plan.Id)
					t.Cleanup(func() { InvalidateSubscriptionPlanCache(plan.Id) })

					err := PurchaseSubscriptionWithBalance(user.Id, plan.Id)
					wantCount, wantQuota, wantGroup := int64(0), test.quota, "default"
					if test.frozen {
						require.ErrorIs(t, err, ErrQuotaFrozen)
					} else {
						require.NoError(t, err)
						wantCount, wantGroup = 1, "pro"
						if test.price > 0 {
							wantQuota -= 100
						}
					}
					var stored User
					require.NoError(t, db.First(&stored, user.Id).Error)
					assert.Equal(t, wantQuota, stored.Quota)
					assert.Equal(t, wantGroup, stored.Group)
					assert.Equal(t, test.frozen, stored.QuotaFrozen)
					for _, record := range []any{&UserSubscription{}, &SubscriptionOrder{}} {
						var count int64
						require.NoError(t, db.Model(record).Where("user_id = ?", user.Id).Count(&count).Error)
						assert.Equal(t, wantCount, count)
					}
					var logCount int64
					require.NoError(t, logDB.Model(&Log{}).Where("user_id = ? AND type = ?", user.Id, LogTypeTopup).Count(&logCount).Error)
					assert.Equal(t, wantCount, logCount)
				})
			}

			for _, test := range []struct {
				name      string
				enabled   bool
				frozen    bool
				role      int
				mode      string
				value     int
				wantQuota int
				wantErr   error
			}{
				{name: "enabled_pool_root", enabled: true, role: common.RoleRootUser, mode: "add", value: 25, wantQuota: 500, wantErr: ErrQuotaPoolPermissionDenied},
				{name: "disabled_pool_root_add", role: common.RoleRootUser, mode: "add", value: 25, wantQuota: 525},
				{name: "disabled_pool_root_subtract", role: common.RoleRootUser, mode: "subtract", value: 25, wantQuota: 475},
				{name: "disabled_pool_root_override", role: common.RoleRootUser, mode: "override", value: 525, wantQuota: 525},
				{name: "disabled_pool_admin", role: common.RoleAdminUser, mode: "add", value: 25, wantQuota: 500, wantErr: ErrUserQuotaPermission},
				{name: "disabled_pool_frozen_root", frozen: true, role: common.RoleRootUser, mode: "add", value: 25, wantQuota: 500, wantErr: ErrQuotaFrozen},
			} {
				t.Run("manual_adjustment/"+test.name, func(t *testing.T) {
					pool := QuotaPool{Name: test.name, PoolType: QuotaPoolTypeNormal, Enabled: test.enabled, BaseQuota: 1000, Quota: 800}
					require.NoError(t, db.Create(&pool).Error)
					user := User{
						Username: test.name, Password: "password", AffCode: test.name,
						Role: common.RoleCommonUser, Status: common.UserStatusEnabled,
						Quota: 500, QuotaFrozen: test.frozen, QuotaPoolId: pool.Id,
					}
					require.NoError(t, db.Create(&user).Error)

					adjustment, err := AdjustUserQuota(user.Id, test.role, test.mode, test.value)
					if test.wantErr != nil {
						require.ErrorIs(t, err, test.wantErr)
						assert.Nil(t, adjustment)
					} else {
						require.NoError(t, err)
						require.NotNil(t, adjustment)
						assert.Equal(t, 500, adjustment.Before)
						assert.Equal(t, test.wantQuota, adjustment.After)
					}
					require.NoError(t, db.First(&user, user.Id).Error)
					assert.Equal(t, test.wantQuota, user.Quota)
					assert.Equal(t, test.frozen, user.QuotaFrozen)
					require.NoError(t, db.First(&pool, pool.Id).Error)
					assert.Equal(t, test.enabled, pool.Enabled)
					assert.Equal(t, 1000, pool.BaseQuota)
					assert.Equal(t, 800, pool.Quota)
					var transactionCount int64
					require.NoError(t, db.Model(&QuotaPoolTransaction{}).Where("pool_id = ?", pool.Id).Count(&transactionCount).Error)
					assert.Zero(t, transactionCount)
				})
			}
		})
	}
}

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
			previousMain, previousLog := common.MainDatabaseType(), common.LogDatabaseType()
			common.SetDatabaseTypes(test.dbType, test.dbType)
			t.Cleanup(func() {
				common.SetDatabaseTypes(previousMain, previousLog)
			})
			for _, fresh := range []bool{true, false} {
				name := "rc25_upgrade"
				if fresh {
					name = "fresh"
				}
				t.Run(name, func(t *testing.T) {
					db, err := gorm.Open(test.dialector, &gorm.Config{})
					require.NoError(t, err)
					t.Cleanup(func() {
						sqlDB, err := db.DB()
						require.NoError(t, err)
						defer func() {
							require.NoError(t, sqlDB.Close())
						}()
						dropQuotaPoolMigrationFixture(t, db)
					})
					dropQuotaPoolMigrationFixture(t, db)
					verifyQuotaPoolRestartMatrix(t, test.dialector, fresh)
				})
			}
		})
	}
}

func seedLegacyQuotaPoolFixture(t *testing.T, db *gorm.DB) {
	t.Helper()
	require.NoError(t, db.AutoMigrate(&legacyQuotaPoolUser{}, &legacyQuotaPool{}, &legacyQuotaPoolAdmin{}, &legacyQuotaPoolTransaction{}, &Option{}))
	require.NoError(t, db.Create(&[]legacyQuotaPoolUser{
		{
			Username: "legacy-root", Password: "fixture-root-password-hash", AffCode: "legacy-root-aff",
			Role: common.RoleRootUser, Status: common.UserStatusEnabled, Quota: 500, UsedQuota: 120,
		},
		{
			Username: "legacy-alice", Password: "fixture-alice-password-hash", AffCode: "legacy-alice-aff",
			Role: common.RoleCommonUser, Status: common.UserStatusEnabled, Quota: 80, UsedQuota: 900,
			QuotaFrozen: true, QuotaPoolId: 2, LDAPId: "alice@example.com", Department: "R&D",
		},
	}).Error)
	require.NoError(t, db.Create(&[]legacyQuotaPool{{Name: QuotaPoolDefaultName, PoolType: QuotaPoolTypeDefault, BaseQuota: -1, Quota: -1, Enabled: true}, {Name: "研发一组", PoolType: QuotaPoolTypeNormal, BaseQuota: 10000, Quota: 4200, Enabled: true}}).Error)
	require.NoError(t, db.Model(&legacyQuotaPool{}).Where("id = ?", 2).Update("enabled", false).Error)
	require.NoError(t, db.Create(&legacyQuotaPoolAdmin{PoolId: 2, UserId: 2, Level: 1}).Error)
	require.NoError(t, db.Create(&[]legacyQuotaPoolTransaction{{PoolId: 2, Amount: 10000}, {PoolId: 2, Amount: -5800}}).Error)
}

func dropQuotaPoolMigrationFixture(t *testing.T, db *gorm.DB) {
	t.Helper()
	require.NoError(t, db.Migrator().DropTable("quota_pool_transactions", "quota_pool_admins", "quota_pools", "quota_pool_budget_tags", "users", "options"))
}
