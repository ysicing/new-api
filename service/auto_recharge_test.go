package service

import (
	"context"
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

func TestGetAutoRechargeEligibilityExplainsLimitWithoutMutatingBalances(t *testing.T) {
	db := setupAutoRechargeTest(t)
	now := time.Date(2026, time.August, 31, 12, 0, 0, 0, time.Local)
	amount := int(common.QuotaPerUnit)
	config := operation_setting.GetAutoRechargeSetting()
	config.MonthlyLimit = 3
	pool := model.QuotaPool{
		Name: "资格诊断池", PoolType: model.QuotaPoolTypeNormal, Enabled: true,
		BaseQuota: amount * 2, Quota: amount * 2,
		AutoRechargeAmount: model.QuotaPoolAutoRechargeInherit,
		WeeklyLimit:        model.QuotaPoolAutoRechargeInherit,
		MonthlyLimit:       model.QuotaPoolAutoRechargeInherit,
	}
	require.NoError(t, db.Create(&pool).Error)
	user := model.User{Username: "eligibility-user", Email: "eligibility@example.com", Password: "password", AffCode: "eligibility-user-aff", Role: common.RoleCommonUser, Status: common.UserStatusEnabled, QuotaPoolId: pool.Id}
	require.NoError(t, db.Create(&user).Error)
	require.NoError(t, db.Create(&model.Log{
		UserId: user.Id, Username: user.Username, Type: model.LogTypeTopup,
		CreatedAt: now.Add(-time.Hour).Unix(), Other: common.MapToJsonStr(map[string]any{"recharge_source": "auto"}),
	}).Error)

	result, err := GetAutoRechargeEligibility(user.Email, now)

	require.NoError(t, err)
	assert.False(t, result.Eligible)
	assert.Equal(t, "weekly_limited", result.Reason)
	assert.Equal(t, user.Id, result.UserId)
	assert.Equal(t, user.Quota, result.UserQuota)
	assert.Equal(t, pool.Id, result.PoolId)
	assert.Equal(t, pool.Name, result.PoolName)
	require.NotNil(t, result.PoolQuota)
	assert.Equal(t, pool.Quota, *result.PoolQuota)
	assert.Equal(t, amount, result.Amount)
	assert.EqualValues(t, 1, result.Weekly.Used)
	assert.Equal(t, 1, result.Weekly.Limit)
	assert.EqualValues(t, 1, result.Monthly.Used)
	assert.Equal(t, 3, result.Monthly.Limit)

	var transactionCount int64
	require.NoError(t, db.Model(&model.QuotaPoolTransaction{}).Count(&transactionCount).Error)
	assert.Zero(t, transactionCount)
	var unchangedUser model.User
	var unchangedPool model.QuotaPool
	require.NoError(t, db.First(&unchangedUser, user.Id).Error)
	require.NoError(t, db.First(&unchangedPool, pool.Id).Error)
	assert.Equal(t, user.Quota, unchangedUser.Quota)
	assert.Equal(t, pool.Quota, unchangedPool.Quota)
}

func TestGetAutoRechargeEligibilityReportsInsufficientPoolBalance(t *testing.T) {
	db := setupAutoRechargeTest(t)
	now := time.Date(2026, time.August, 31, 12, 0, 0, 0, time.Local)
	amount := int(common.QuotaPerUnit)
	config := operation_setting.GetAutoRechargeSetting()
	config.WeeklyLimit = 0
	pool := model.QuotaPool{
		Name: "余额不足池", PoolType: model.QuotaPoolTypeNormal, Enabled: true,
		BaseQuota: amount / 2, Quota: amount / 2,
		AutoRechargeAmount: model.QuotaPoolAutoRechargeInherit,
		WeeklyLimit:        model.QuotaPoolAutoRechargeInherit,
		MonthlyLimit:       model.QuotaPoolAutoRechargeInherit,
	}
	require.NoError(t, db.Create(&pool).Error)
	user := model.User{Username: "insufficient-user", Password: "password", AffCode: "insufficient-user-aff", Role: common.RoleCommonUser, Status: common.UserStatusEnabled, QuotaPoolId: pool.Id}
	require.NoError(t, db.Create(&user).Error)

	result, err := GetAutoRechargeEligibility(user.Username, now)

	require.NoError(t, err)
	assert.False(t, result.Eligible)
	assert.Equal(t, "quota_pool_insufficient", result.Reason)
	assert.Equal(t, amount, result.Amount)
}

func TestGetAutoRechargeEligibilityIncludesDisabledPoolDetails(t *testing.T) {
	db := setupAutoRechargeTest(t)
	pool := model.QuotaPool{
		Name: "已停用池", PoolType: model.QuotaPoolTypeNormal, Enabled: false,
		BaseQuota: 800, Quota: 600,
		AutoRechargeAmount: model.QuotaPoolAutoRechargeInherit,
		WeeklyLimit:        model.QuotaPoolAutoRechargeInherit,
		MonthlyLimit:       model.QuotaPoolAutoRechargeInherit,
	}
	require.NoError(t, db.Create(&pool).Error)
	require.NoError(t, db.Model(&pool).Update("enabled", false).Error)
	pool.Enabled = false
	user := model.User{Username: "disabled-pool-user", Password: "password", AffCode: "disabled-pool-user-aff", Role: common.RoleCommonUser, Status: common.UserStatusEnabled, QuotaPoolId: pool.Id}
	require.NoError(t, db.Create(&user).Error)

	result, err := GetAutoRechargeEligibility(user.Username, time.Now())

	require.NoError(t, err)
	assert.False(t, result.Eligible)
	assert.Equal(t, "quota_pool_disabled", result.Reason)
	assert.Equal(t, pool.Name, result.PoolName)
	require.NotNil(t, result.PoolQuota)
	assert.Equal(t, pool.Quota, *result.PoolQuota)
}

func TestGetAutoRechargeEligibilityLabelsSystemDefaultPool(t *testing.T) {
	db := setupAutoRechargeTest(t)
	config := operation_setting.GetAutoRechargeSetting()
	config.WeeklyLimit = 0
	config.MonthlyLimit = 0
	user := model.User{Username: "default-pool-user", Password: "password", AffCode: "default-pool-user-aff", Role: common.RoleCommonUser, Status: common.UserStatusEnabled, QuotaPoolId: model.QuotaPoolDefaultUserPoolId}
	require.NoError(t, db.Create(&user).Error)

	result, err := GetAutoRechargeEligibility(user.Username, time.Now())

	require.NoError(t, err)
	assert.True(t, result.Eligible)
	assert.Equal(t, model.QuotaPoolDefaultUserPoolId, result.PoolId)
	assert.Equal(t, model.QuotaPoolDefaultName, result.PoolName)
	assert.Nil(t, result.PoolQuota)
}

func TestGetAutoRechargeEligibilityKeepsPolicyDetailsWhenBalanceIsAboveThreshold(t *testing.T) {
	db := setupAutoRechargeTest(t)
	amount := int(common.QuotaPerUnit)
	config := operation_setting.GetAutoRechargeSetting()
	config.WeeklyLimit = 0
	config.MonthlyLimit = 0
	pool := model.QuotaPool{
		Name: "高余额用户池", PoolType: model.QuotaPoolTypeNormal, Enabled: true,
		BaseQuota: amount * 4, Quota: amount * 4,
		AutoRechargeAmount: model.QuotaPoolAutoRechargeInherit,
		WeeklyLimit:        model.QuotaPoolAutoRechargeInherit,
		MonthlyLimit:       model.QuotaPoolAutoRechargeInherit,
	}
	require.NoError(t, db.Create(&pool).Error)
	user := model.User{Username: "above-threshold-details", Password: "password", AffCode: "above-threshold-details-aff", Role: common.RoleCommonUser, Status: common.UserStatusEnabled, Quota: amount * 3, QuotaPoolId: pool.Id}
	require.NoError(t, db.Create(&user).Error)

	result, err := GetAutoRechargeEligibility(user.Username, time.Now())

	require.NoError(t, err)
	assert.False(t, result.Eligible)
	assert.Equal(t, "quota_above_threshold", result.Reason)
	assert.Equal(t, pool.Name, result.PoolName)
	require.NotNil(t, result.PoolQuota)
	assert.Equal(t, pool.Quota, *result.PoolQuota)
	assert.Equal(t, amount, result.Amount)
}

func TestGetAutoRechargeEligibilityCountsUsageWhenLimitsAreUnlimited(t *testing.T) {
	db := setupAutoRechargeTest(t)
	now := time.Date(2026, time.August, 31, 12, 0, 0, 0, time.Local)
	config := operation_setting.GetAutoRechargeSetting()
	config.WeeklyLimit = 0
	config.MonthlyLimit = 0
	user := model.User{Username: "unlimited-count-user", Password: "password", AffCode: "unlimited-count-user-aff", Role: common.RoleCommonUser, Status: common.UserStatusEnabled}
	require.NoError(t, db.Create(&user).Error)
	require.NoError(t, db.Create(&model.Log{
		UserId: user.Id, Username: user.Username, Type: model.LogTypeTopup,
		CreatedAt: now.Add(-time.Hour).Unix(), Other: common.MapToJsonStr(map[string]any{"recharge_source": "auto"}),
	}).Error)

	result, err := GetAutoRechargeEligibility(user.Username, now)

	require.NoError(t, err)
	assert.True(t, result.Eligible)
	assert.EqualValues(t, 1, result.Weekly.Used)
	assert.Zero(t, result.Weekly.Limit)
	assert.EqualValues(t, 1, result.Monthly.Used)
	assert.Zero(t, result.Monthly.Limit)
}

func TestGetAutoRechargeEligibilityIgnoresStalePoolWhenFeatureIsDisabled(t *testing.T) {
	db := setupAutoRechargeTest(t)
	pool := model.QuotaPool{
		Name: "停用功能前的池", PoolType: model.QuotaPoolTypeNormal, Enabled: true,
		BaseQuota: 1000, Quota: 1000,
		AutoRechargeAmount: model.QuotaPoolAutoRechargeInherit,
		WeeklyLimit:        model.QuotaPoolAutoRechargeInherit,
		MonthlyLimit:       model.QuotaPoolAutoRechargeInherit,
	}
	require.NoError(t, db.Create(&pool).Error)
	user := model.User{Username: "stale-pool-user", Password: "password", AffCode: "stale-pool-user-aff", Role: common.RoleCommonUser, Status: common.UserStatusEnabled, QuotaPoolId: pool.Id}
	require.NoError(t, db.Create(&user).Error)
	common.QuotaPoolEnabled = false

	result, err := GetAutoRechargeEligibility(user.Username, time.Now())

	require.NoError(t, err)
	assert.True(t, result.Eligible)
	assert.Equal(t, model.QuotaPoolDefaultUserPoolId, result.PoolId)
	assert.Empty(t, result.PoolName)
	assert.Nil(t, result.PoolQuota)
}

func TestGetAutoRechargeEligibilityBlocksDisabledUserFromMaintenance(t *testing.T) {
	db := setupAutoRechargeTest(t)
	config := operation_setting.GetAutoRechargeSetting()
	config.WeeklyLimit = 0
	config.MonthlyLimit = 0
	user := model.User{Username: "disabled-user", Password: "password", AffCode: "disabled-user-aff", Role: common.RoleCommonUser, Status: common.UserStatusDisabled}
	require.NoError(t, db.Create(&user).Error)

	result, err := GetAutoRechargeEligibility(user.Username, time.Now())

	require.NoError(t, err)
	assert.False(t, result.Eligible)
	assert.Equal(t, "user_disabled", result.Reason)
	assert.Greater(t, result.Amount, 0)
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

func TestQuotaPoolMaintenanceResultIncludesSkipReasons(t *testing.T) {
	db := setupAutoRechargeTest(t)
	require.NoError(t, db.AutoMigrate(&model.SystemTask{}, &model.SystemTaskLock{}))
	amount := int(common.QuotaPerUnit)
	normalPool := model.QuotaPool{
		Name: "维护任务池", PoolType: model.QuotaPoolTypeNormal, Enabled: true,
		BaseQuota: amount * 2, Quota: amount * 2,
		AutoRechargeAmount: model.QuotaPoolAutoRechargeInherit,
		WeeklyLimit:        model.QuotaPoolAutoRechargeInherit, MonthlyLimit: model.QuotaPoolAutoRechargeInherit,
	}
	newUserPool := model.QuotaPool{
		Name: "新用户池", PoolType: model.QuotaPoolTypeNewUser, Enabled: true,
		AutoRechargeAmount: model.QuotaPoolAutoRechargeOff,
	}
	require.NoError(t, db.Create(&normalPool).Error)
	require.NoError(t, db.Create(&newUserPool).Error)
	require.NoError(t, db.Create(&[]model.User{
		{Id: 1, Username: "eligible", Password: "password", AffCode: "maintenance-eligible", Status: common.UserStatusEnabled, QuotaPoolId: normalPool.Id},
		{Id: 2, Username: "above-threshold", Password: "password", AffCode: "maintenance-above", Status: common.UserStatusEnabled, Quota: 3 * amount},
		{Id: 3, Username: "new-user", Password: "password", AffCode: "maintenance-new", Status: common.UserStatusEnabled, QuotaPoolId: newUserPool.Id},
	}).Error)
	task, err := model.CreateSystemTask(model.SystemTaskTypeQuotaPoolMaintenance, struct{}{}, nil)
	require.NoError(t, err)
	claimed, ok, err := model.ClaimSystemTask(task.ID, task.Type, "runner-test", common.GetTimestamp()+60)
	require.NoError(t, err)
	require.True(t, ok)

	quotaPoolMaintenanceHandler{}.Run(context.Background(), claimed, "runner-test")

	completed, err := model.GetSystemTaskByTaskID(task.TaskID)
	require.NoError(t, err)
	require.NotNil(t, completed)
	var result struct {
		UsersChecked   int            `json:"users_checked"`
		UsersRecharged int            `json:"users_recharged"`
		UsersSkipped   int            `json:"users_skipped"`
		SkipReasons    map[string]int `json:"skip_reasons"`
	}
	require.NoError(t, common.UnmarshalJsonStr(completed.Result, &result))
	assert.Equal(t, 3, result.UsersChecked)
	assert.Equal(t, 1, result.UsersRecharged)
	assert.Equal(t, 2, result.UsersSkipped)
	assert.Equal(t, map[string]int{
		"quota_above_threshold":  1,
		"new_user_pool_disabled": 1,
	}, result.SkipReasons)
}
