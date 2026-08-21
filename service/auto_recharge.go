package service

import (
	"fmt"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"
)

type QuotaPoolAutoRechargeResult struct {
	Recharged bool   `json:"recharged"`
	Reason    string `json:"reason,omitempty"`
	Amount    int    `json:"amount,omitempty"`
}

type WeeklyAutoRechargeUsage struct {
	Enabled   bool  `json:"enabled"`
	Used      int64 `json:"used"`
	Limit     int   `json:"limit"`
	Remaining int64 `json:"remaining"`
}

type effectiveAutoRechargePolicy struct {
	AmountQuota  int
	WeeklyLimit  int
	MonthlyLimit int
}

func resolveAutoRechargePolicy(config *operation_setting.AutoRechargeSetting, pool *model.QuotaPool) effectiveAutoRechargePolicy {
	policy := effectiveAutoRechargePolicy{
		AmountQuota:  int(float64(config.Amount) * common.QuotaPerUnit),
		WeeklyLimit:  config.WeeklyLimit,
		MonthlyLimit: config.MonthlyLimit,
	}
	if pool == nil || pool.PoolType == model.QuotaPoolTypeDefault {
		return policy
	}
	if pool.AutoRechargeAmount >= 0 {
		policy.AmountQuota = pool.AutoRechargeAmount
	}
	if pool.WeeklyLimit >= 0 {
		policy.WeeklyLimit = pool.WeeklyLimit
	}
	if pool.MonthlyLimit >= 0 {
		policy.MonthlyLimit = pool.MonthlyLimit
	}
	return policy
}

func TryAutoRechargeUserById(userId int) QuotaPoolAutoRechargeResult {
	var user model.User
	if err := model.DB.Where("id = ?", userId).First(&user).Error; err != nil {
		return QuotaPoolAutoRechargeResult{Reason: "user_not_found"}
	}
	return tryAutoRechargeUser(&user, time.Now())
}

func GetWeeklyAutoRechargeUsage(user *model.User, pool *model.QuotaPool, now time.Time) (WeeklyAutoRechargeUsage, error) {
	usage := WeeklyAutoRechargeUsage{}
	config := operation_setting.GetAutoRechargeSetting()
	if user == nil || pool == nil || !config.Enabled || !pool.Enabled || pool.IsNewUserPool() {
		return usage, nil
	}
	policy := resolveAutoRechargePolicy(config, pool)
	if policy.AmountQuota <= 0 {
		return usage, nil
	}
	used, err := model.CountAutoRechargeLogs(user.Id, startOfWeek(now).Unix())
	if err != nil {
		return usage, err
	}
	usage.Enabled = true
	usage.Used = used
	usage.Limit = policy.WeeklyLimit
	if policy.WeeklyLimit > 0 {
		usage.Remaining = max(int64(policy.WeeklyLimit)-used, 0)
	}
	return usage, nil
}

func tryAutoRechargeUser(user *model.User, now time.Time) QuotaPoolAutoRechargeResult {
	config := operation_setting.GetAutoRechargeSetting()
	if !config.Enabled {
		return QuotaPoolAutoRechargeResult{Reason: "disabled"}
	}
	threshold := int(float64(config.Threshold) * common.QuotaPerUnit)
	if user == nil || user.Quota > threshold {
		return QuotaPoolAutoRechargeResult{Reason: "quota_above_threshold"}
	}
	pool, reason := autoRechargePool(user)
	if reason != "" {
		return QuotaPoolAutoRechargeResult{Reason: reason}
	}
	policy := resolveAutoRechargePolicy(config, pool)
	if policy.AmountQuota <= 0 {
		return QuotaPoolAutoRechargeResult{Reason: "amount_not_configured"}
	}
	if reason := autoRechargeLimitReason(user.Id, policy, now); reason != "" {
		return QuotaPoolAutoRechargeResult{Reason: reason}
	}
	if err := creditAutoRecharge(user, pool, policy.AmountQuota); err != nil {
		return QuotaPoolAutoRechargeResult{Reason: err.Error()}
	}
	poolId, poolName := 0, ""
	if pool != nil {
		poolId, poolName = pool.Id, pool.Name
	}
	if err := model.RecordAutoRechargeLog(user.Id, poolId, policy.AmountQuota, 0, poolName); err != nil {
		common.SysLog(fmt.Sprintf("failed to record auto recharge for user %d: %v", user.Id, err))
	}
	return QuotaPoolAutoRechargeResult{Recharged: true, Amount: policy.AmountQuota}
}

func autoRechargePool(user *model.User) (*model.QuotaPool, string) {
	if !common.QuotaPoolEnabled || user.QuotaPoolId == model.QuotaPoolDefaultUserPoolId {
		return nil, ""
	}
	pool, err := model.GetQuotaPoolById(user.QuotaPoolId)
	if err != nil {
		return nil, "quota_pool_not_found"
	}
	if pool.IsNewUserPool() {
		return nil, "new_user_pool_disabled"
	}
	if !pool.Enabled {
		return nil, "quota_pool_disabled"
	}
	return pool, ""
}

func autoRechargeLimitReason(userId int, policy effectiveAutoRechargePolicy, now time.Time) string {
	weekStart := startOfWeek(now).Unix()
	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location()).Unix()
	if policy.WeeklyLimit > 0 {
		count, err := model.CountAutoRechargeLogs(userId, weekStart)
		if err != nil {
			return "weekly_count_failed"
		}
		if count >= int64(policy.WeeklyLimit) {
			return "weekly_limited"
		}
	}
	if policy.MonthlyLimit > 0 {
		count, err := model.CountAutoRechargeLogs(userId, monthStart)
		if err != nil {
			return "monthly_count_failed"
		}
		if count >= int64(policy.MonthlyLimit) {
			return "monthly_limited"
		}
	}
	return ""
}

func startOfWeek(now time.Time) time.Time {
	weekday := int(now.Weekday())
	if weekday == 0 {
		weekday = 7
	}
	return time.Date(now.Year(), now.Month(), now.Day()-weekday+1, 0, 0, 0, 0, now.Location())
}

func creditAutoRecharge(user *model.User, pool *model.QuotaPool, amount int) error {
	if pool == nil {
		return model.IncreaseUserQuota(user.Id, amount, true)
	}
	_, err := model.AllocateQuotaFromPool(pool.Id, user.Id, amount, model.QuotaPoolTransactionAllocateAuto, 0)
	return err
}
