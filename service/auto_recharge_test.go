package service

import (
	"fmt"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupAutoRechargeTest(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:auto-recharge-%s?mode=memory&cache=shared", t.Name())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.User{}, &model.Log{}, &model.QuotaPool{}, &model.QuotaPoolAdmin{}, &model.QuotaPoolTransaction{}))
	previousDB, previousLogDB := model.DB, model.LOG_DB
	previousEnabled, previousRedis := common.QuotaPoolEnabled, common.RedisEnabled
	previousConfig := *operation_setting.GetAutoRechargeSetting()
	model.DB, model.LOG_DB = db, db
	common.QuotaPoolEnabled = true
	common.RedisEnabled = false
	common.SetDatabaseTypes(common.DatabaseTypeSQLite, common.DatabaseTypeSQLite)
	*operation_setting.GetAutoRechargeSetting() = operation_setting.AutoRechargeSetting{Enabled: true, Interval: 30, Threshold: 2, Amount: 1, WeeklyLimit: 1}
	t.Cleanup(func() {
		model.DB, model.LOG_DB = previousDB, previousLogDB
		common.QuotaPoolEnabled, common.RedisEnabled = previousEnabled, previousRedis
		*operation_setting.GetAutoRechargeSetting() = previousConfig
	})
	return db
}

func TestTryAutoRechargeUserUsesPoolAndHonorsWeeklyLimit(t *testing.T) {
	db := setupAutoRechargeTest(t)
	amount := int(common.QuotaPerUnit)
	pool := model.QuotaPool{Name: "自动充值池", PoolType: model.QuotaPoolTypeNormal, Enabled: true, BaseQuota: amount * 2, Quota: amount * 2, AutoRechargeAmount: model.QuotaPoolAutoRechargeInherit, WeeklyLimit: model.QuotaPoolAutoRechargeInherit, MonthlyLimit: model.QuotaPoolAutoRechargeInherit}
	require.NoError(t, db.Create(&pool).Error)
	user := model.User{Username: "auto-user", Password: "password", AffCode: "auto-user-aff", Role: common.RoleCommonUser, Status: common.UserStatusEnabled, QuotaPoolId: pool.Id}
	require.NoError(t, db.Create(&user).Error)

	first := TryAutoRechargeUserById(user.Id)
	second := TryAutoRechargeUserById(user.Id)

	assert.True(t, first.Recharged)
	assert.Equal(t, amount, first.Amount)
	assert.False(t, second.Recharged)
	assert.Equal(t, "weekly_limited", second.Reason)
	require.NoError(t, db.First(&pool, pool.Id).Error)
	require.NoError(t, db.First(&user, user.Id).Error)
	assert.Equal(t, amount, pool.Quota)
	assert.Equal(t, amount, user.Quota)
	usage, err := GetWeeklyAutoRechargeUsage(&user, &pool, time.Now())
	require.NoError(t, err)
	assert.True(t, usage.Enabled)
	assert.EqualValues(t, 1, usage.Used)
	assert.Zero(t, usage.Remaining)
	var transactionCount int64
	require.NoError(t, db.Model(&model.QuotaPoolTransaction{}).Where("type = ?", model.QuotaPoolTransactionAllocateAuto).Count(&transactionCount).Error)
	assert.EqualValues(t, 1, transactionCount)
}

func TestRefillMonthlyQuotaPoolsTopUpIsIdempotent(t *testing.T) {
	db := setupAutoRechargeTest(t)
	now := time.Date(2026, time.August, 21, 12, 0, 0, 0, time.Local)
	pool := model.QuotaPool{
		Name: "月度补齐池", PoolType: model.QuotaPoolTypeNormal, Enabled: true,
		BaseQuota: 1000, Quota: 300, MonthlyRefillEnabled: true,
		MonthlyRefillTopUp: true, MonthlyRefillAmount: 600, MonthlyRefillDay: 10,
	}
	require.NoError(t, db.Create(&pool).Error)

	first, err := RefillMonthlyQuotaPools(now)
	require.NoError(t, err)
	second, err := RefillMonthlyQuotaPools(now)
	require.NoError(t, err)

	assert.Equal(t, 1, first.Refilled)
	assert.Equal(t, 0, second.Refilled)
	require.NoError(t, db.First(&pool, pool.Id).Error)
	assert.Equal(t, 600, pool.Quota)
	assert.Equal(t, 1300, pool.BaseQuota)
	assert.Equal(t, 202608, pool.LastRefillMonth)
	var count int64
	require.NoError(t, db.Model(&model.QuotaPoolTransaction{}).Where("type = ?", model.QuotaPoolTransactionMonthlyRefill).Count(&count).Error)
	assert.EqualValues(t, 1, count)
}

func TestQuotaPoolMaintenanceHandlerUsesConfiguredInterval(t *testing.T) {
	setupAutoRechargeTest(t)
	handler := quotaPoolMaintenanceHandler{}

	assert.Equal(t, model.SystemTaskTypeQuotaPoolMaintenance, handler.Type())
	assert.True(t, handler.Enabled())
	assert.Equal(t, 30*time.Minute, handler.Interval())
}
