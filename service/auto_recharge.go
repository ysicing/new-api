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

type AutoRechargeLimitUsage struct {
	Used  int64 `json:"used"`
	Limit int   `json:"limit"`
}

type AutoRechargeEligibility struct {
	Eligible  bool                   `json:"eligible"`
	Reason    string                 `json:"reason,omitempty"`
	UserId    int                    `json:"user_id"`
	Username  string                 `json:"username"`
	Email     string                 `json:"email"`
	UserQuota int                    `json:"user_quota"`
	Threshold int                    `json:"threshold"`
	PoolId    int                    `json:"pool_id"`
	PoolName  string                 `json:"pool_name"`
	PoolQuota *int                   `json:"pool_quota"`
	Amount    int                    `json:"amount"`
	Weekly    AutoRechargeLimitUsage `json:"weekly"`
	Monthly   AutoRechargeLimitUsage `json:"monthly"`
}

type effectiveAutoRechargePolicy struct {
	AmountQuota  int
	WeeklyLimit  int
	MonthlyLimit int
}

func resolveAutoRechargePolicy(config *operation_setting.AutoRechargeSetting, pool *model.QuotaPool) effectiveAutoRechargePolicy {
	policy := effectiveAutoRechargePolicy{
		AmountQuota:  common.QuotaFromFloat(float64(config.Amount) * common.QuotaPerUnit),
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

// GetAutoRechargeEligibility 返回当前时点的只读诊断快照，不会执行充值。
func GetAutoRechargeEligibility(identifier string, now time.Time) (*AutoRechargeEligibility, error) {
	user, err := model.FindUserByRechargeIdentifier(identifier)
	if err != nil {
		return nil, err
	}
	result, _ := evaluateAutoRechargeUser(user, now, true)
	// 维护任务只扫描启用用户；诊断需包含这层外部约束，避免把任务永远
	// 不会处理的禁用用户误报为可自动充值。
	if user.Status != common.UserStatusEnabled {
		result.Eligible = false
		result.Reason = "user_disabled"
	}
	return &result, nil
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
	eligibility, pool := evaluateAutoRechargeUser(user, now, false)
	if !eligibility.Eligible {
		return QuotaPoolAutoRechargeResult{Reason: eligibility.Reason}
	}
	if err := creditAutoRecharge(user, pool, eligibility.Amount); err != nil {
		return QuotaPoolAutoRechargeResult{Reason: err.Error()}
	}
	poolId, poolName := 0, ""
	if pool != nil {
		poolId, poolName = pool.Id, pool.Name
	}
	if err := model.RecordAutoRechargeLog(user.Id, poolId, eligibility.Amount, 0, poolName); err != nil {
		common.SysLog(fmt.Sprintf("failed to record auto recharge for user %d: %v", user.Id, err))
	}
	return QuotaPoolAutoRechargeResult{Recharged: true, Amount: eligibility.Amount}
}

func evaluateAutoRechargeUser(user *model.User, now time.Time, collectDetails bool) (AutoRechargeEligibility, *model.QuotaPool) {
	config := operation_setting.GetAutoRechargeSetting()
	result := AutoRechargeEligibility{
		Threshold: common.QuotaFromFloat(float64(config.Threshold) * common.QuotaPerUnit),
		Weekly:    AutoRechargeLimitUsage{Limit: config.WeeklyLimit},
		Monthly:   AutoRechargeLimitUsage{Limit: config.MonthlyLimit},
	}
	if user == nil {
		result.Reason = "user_not_found"
		return result, nil
	}
	result.UserId, result.Username, result.Email = user.Id, user.Username, user.Email
	result.UserQuota, result.PoolId = user.Quota, user.QuotaPoolId
	if !common.QuotaPoolEnabled {
		result.PoolId = model.QuotaPoolDefaultUserPoolId
	} else if user.QuotaPoolId == model.QuotaPoolDefaultUserPoolId {
		result.PoolName = model.QuotaPoolDefaultName
	}
	if !config.Enabled {
		result.Reason = "disabled"
		if !collectDetails {
			return result, nil
		}
	}
	if user.Quota > result.Threshold && result.Reason == "" {
		result.Reason = "quota_above_threshold"
		if !collectDetails {
			return result, nil
		}
	}
	pool, poolReason := autoRechargePool(user)
	if pool != nil {
		poolQuota := pool.Quota
		result.PoolId, result.PoolName, result.PoolQuota = pool.Id, pool.Name, &poolQuota
	}
	if poolReason != "" {
		if result.Reason == "" {
			result.Reason = poolReason
		}
		if !collectDetails {
			return result, pool
		}
	}
	policy := resolveAutoRechargePolicy(config, pool)
	result.Amount = policy.AmountQuota
	result.Weekly.Limit, result.Monthly.Limit = policy.WeeklyLimit, policy.MonthlyLimit
	if policy.AmountQuota <= 0 {
		if result.Reason == "" {
			result.Reason = "amount_not_configured"
		}
		if !collectDetails {
			return result, pool
		}
	}
	var limitReason string
	result.Weekly, result.Monthly, limitReason = evaluateAutoRechargeLimits(user.Id, policy, now, collectDetails)
	if limitReason != "" {
		if result.Reason == "" {
			result.Reason = limitReason
		}
		if !collectDetails {
			return result, pool
		}
	}
	// 只读诊断可以依据当前池余额给出快照结论；实际充值必须继续进入
	// AllocateQuotaFromPool 的事务条件更新，避免并发补充额度后仍按旧快照拒绝。
	if collectDetails && pool != nil && pool.Quota < policy.AmountQuota && result.Reason == "" {
		result.Reason = "quota_pool_insufficient"
	}
	result.Eligible = result.Reason == ""
	return result, pool
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
		return pool, "new_user_pool_disabled"
	}
	if !pool.Enabled {
		return pool, "quota_pool_disabled"
	}
	return pool, ""
}

func evaluateAutoRechargeLimits(userId int, policy effectiveAutoRechargePolicy, now time.Time, collectAll bool) (AutoRechargeLimitUsage, AutoRechargeLimitUsage, string) {
	weekStart := startOfWeek(now).Unix()
	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location()).Unix()
	weekly := AutoRechargeLimitUsage{Limit: policy.WeeklyLimit}
	monthly := AutoRechargeLimitUsage{Limit: policy.MonthlyLimit}
	weeklyReason := ""
	if policy.WeeklyLimit > 0 || collectAll {
		count, err := model.CountAutoRechargeLogs(userId, weekStart)
		if err != nil {
			if policy.WeeklyLimit > 0 {
				return weekly, monthly, "weekly_count_failed"
			}
		} else {
			weekly.Used = count
		}
		if policy.WeeklyLimit > 0 && count >= int64(policy.WeeklyLimit) {
			weeklyReason = "weekly_limited"
			if !collectAll {
				return weekly, monthly, weeklyReason
			}
		}
	}
	if policy.MonthlyLimit > 0 || collectAll {
		count, err := model.CountAutoRechargeLogs(userId, monthStart)
		if err != nil {
			if policy.MonthlyLimit > 0 {
				if weeklyReason != "" {
					return weekly, monthly, weeklyReason
				}
				return weekly, monthly, "monthly_count_failed"
			}
		} else {
			monthly.Used = count
		}
		if policy.MonthlyLimit > 0 && count >= int64(policy.MonthlyLimit) {
			if weeklyReason != "" {
				return weekly, monthly, weeklyReason
			}
			return weekly, monthly, "monthly_limited"
		}
	}
	return weekly, monthly, weeklyReason
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
