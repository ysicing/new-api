package service

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/bytedance/gopkg/util/gopool"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var (
	autoRechargeOnce   sync.Once
	autoRechargeRunner = AutoRechargeTask
)

func StartAutoRechargeTask() {
	autoRechargeOnce.Do(func() {
		if !common.IsMasterNode {
			return
		}
		gopool.Go(func() {
			autoRechargeRunner()
		})
	})
}

func AutoRechargeTask() {
	for {
		cfg := operation_setting.GetAutoRechargeSetting()
		interval := cfg.Interval
		if interval <= 0 {
			interval = 30
		}
		if !cfg.Enabled {
			time.Sleep(time.Duration(interval) * time.Minute)
			continue
		}

		thresholdQuota := int(float64(cfg.Threshold) * common.QuotaPerUnit)
		amountQuota := int(float64(cfg.Amount) * common.QuotaPerUnit)

		common.SysLog(fmt.Sprintf(
			"auto recharge task started, interval: %d min, threshold: $%d (%s), amount: $%d (%s), weekly_limit: %d, monthly_limit: %d",
			interval,
			cfg.Threshold, logger.LogQuota(thresholdQuota),
			cfg.Amount, logger.LogQuota(amountQuota),
			cfg.WeeklyLimit, cfg.MonthlyLimit,
		))

		refillMonthlyQuotaPools()
		checkAndRechargeUsers(cfg, thresholdQuota, amountQuota)
		time.Sleep(time.Duration(interval) * time.Minute)
	}
}

type effectiveAutoRechargePolicy struct {
	AmountQuota  int
	WeeklyLimit  int
	MonthlyLimit int
}

type QuotaPoolAutoRechargeResult struct {
	Recharged bool
	Reason    string
	Amount    int
}

func formatAutoRechargeLog(pool *model.QuotaPool, amountQuota int) string {
	quotaText := logger.LogQuota(amountQuota)
	if pool != nil && !pool.IsDefault {
		return fmt.Sprintf("额度池%s自动赠送 %s", pool.Name, quotaText)
	}
	return fmt.Sprintf("系统自动赠送 %s", quotaText)
}

func TryAutoRechargeUserById(userId int) QuotaPoolAutoRechargeResult {
	cfg := operation_setting.GetAutoRechargeSetting()
	user := &model.User{}
	if err := model.DB.First(user, "id = ?", userId).Error; err != nil {
		return QuotaPoolAutoRechargeResult{Reason: "user_not_found"}
	}
	thresholdQuota := int(float64(cfg.Threshold) * common.QuotaPerUnit)
	systemAmountQuota := int(float64(cfg.Amount) * common.QuotaPerUnit)
	return tryAutoRechargeUser(cfg, user, thresholdQuota, systemAmountQuota)
}

func resolveAutoRechargePolicy(cfg *operation_setting.AutoRechargeSetting, pool *model.QuotaPool, systemAmountQuota int) effectiveAutoRechargePolicy {
	policy := effectiveAutoRechargePolicy{
		AmountQuota:  systemAmountQuota,
		WeeklyLimit:  cfg.WeeklyLimit,
		MonthlyLimit: cfg.MonthlyLimit,
	}
	if pool == nil || pool.IsDefault {
		return policy
	}
	switch {
	case pool.AutoRechargeAmount == model.QuotaPoolAutoRechargeInherit:
		policy.AmountQuota = systemAmountQuota
	case pool.AutoRechargeAmount == model.QuotaPoolAutoRechargeOff:
		policy.AmountQuota = 0
	case pool.AutoRechargeAmount > 0:
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

func refillMonthlyQuotaPools() {
	if !common.QuotaPoolEnabled {
		return
	}
	now := time.Now()
	currentMonth := now.Year()*100 + int(now.Month())
	day := now.Day()

	var pools []model.QuotaPool
	err := model.DB.Where("is_default = ? AND enabled = ? AND monthly_refill_enabled = ? AND monthly_refill_amount > ? AND monthly_refill_day <= ? AND last_refill_month <> ?",
		false, true, true, 0, day, currentMonth).Find(&pools).Error
	if err != nil {
		common.SysError(fmt.Sprintf("failed to get quota pools for monthly refill: %s", err.Error()))
		return
	}

	for _, pool := range pools {
		err := model.DB.Transaction(func(tx *gorm.DB) error {
			var currentPool model.QuotaPool
			if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
				Where("id = ? AND enabled = ? AND monthly_refill_enabled = ? AND monthly_refill_amount > ? AND monthly_refill_day <= ? AND last_refill_month <> ?",
					pool.Id, true, true, 0, day, currentMonth).
				First(&currentPool).Error; err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					return nil
				}
				return err
			}

			actualAmount := currentPool.MonthlyRefillAmount
			if currentPool.MonthlyRefillTopUp {
				actualAmount = currentPool.MonthlyRefillAmount - currentPool.Quota
				if actualAmount < 0 {
					actualAmount = 0
				}
			}

			updates := map[string]interface{}{"last_refill_month": currentMonth}
			if actualAmount > 0 {
				updates["quota"] = gorm.Expr("quota + ?", actualAmount)
				updates["base_quota"] = gorm.Expr("base_quota + ?", actualAmount)
			}
			if err := tx.Model(&model.QuotaPool{}).Where("id = ?", currentPool.Id).Updates(updates).Error; err != nil {
				return err
			}
			if actualAmount == 0 {
				return nil
			}
			return tx.Create(&model.QuotaPoolTransaction{
				PoolId:      currentPool.Id,
				Type:        model.QuotaPoolTransactionMonthlyRefill,
				Amount:      actualAmount,
				QuotaBefore: currentPool.Quota,
				QuotaAfter:  currentPool.Quota + actualAmount,
			}).Error
		})
		if err != nil {
			common.SysError(fmt.Sprintf("failed to refill quota pool %d: %s", pool.Id, err.Error()))
		}
	}
}

func checkAndRechargeUsers(
	cfg *operation_setting.AutoRechargeSetting,
	thresholdQuota, amountQuota int,
) {
	defer func() {
		if r := recover(); r != nil {
			common.SysError(fmt.Sprintf("auto recharge task panic: %v", r))
		}
	}()

	var users []model.User
	err := model.DB.Where("status = ?", common.UserStatusEnabled).Find(&users).Error
	if err != nil {
		common.SysError(fmt.Sprintf("failed to get users for auto recharge: %s", err.Error()))
		return
	}

	rechargedCount := 0
	for _, user := range users {
		result := tryAutoRechargeUser(cfg, &user, thresholdQuota, amountQuota)
		if !result.Recharged {
			continue
		}
		rechargedCount++
	}

	if rechargedCount > 0 {
		common.SysLog(fmt.Sprintf("auto recharge completed, total recharged: %d users", rechargedCount))
	}
}

func tryAutoRechargeUser(
	cfg *operation_setting.AutoRechargeSetting,
	user *model.User,
	thresholdQuota, systemAmountQuota int,
) QuotaPoolAutoRechargeResult {
	if user == nil {
		return QuotaPoolAutoRechargeResult{Reason: "missing_user"}
	}
	if user.Quota > thresholdQuota {
		return QuotaPoolAutoRechargeResult{Reason: "quota_above_threshold"}
	}

	var pool *model.QuotaPool
	if common.QuotaPoolEnabled && user.QuotaPoolId != model.QuotaPoolDefaultUserPoolId {
		var err error
		pool, err = model.GetQuotaPoolById(user.QuotaPoolId)
		if err != nil {
			return QuotaPoolAutoRechargeResult{Reason: "quota_pool_not_found"}
		}
	}
	if pool != nil && pool.IsNewUserPool() {
		return QuotaPoolAutoRechargeResult{Reason: "new_user_quota_pool_auto_recharge_disabled"}
	}
	if pool != nil && !pool.IsDefault && !pool.Enabled {
		return QuotaPoolAutoRechargeResult{Reason: "quota_pool_disabled"}
	}

	policy := resolveAutoRechargePolicy(cfg, pool, systemAmountQuota)
	if policy.AmountQuota <= 0 {
		return QuotaPoolAutoRechargeResult{Reason: "amount_not_configured"}
	}

	now := time.Now()
	weekday := int(now.Weekday())
	if weekday == 0 {
		weekday = 7
	}
	weekStart := time.Date(now.Year(), now.Month(), now.Day()-weekday+1, 0, 0, 0, 0, now.Location()).Unix()
	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location()).Unix()

	if policy.WeeklyLimit > 0 {
		weekCount, err := model.CountAutoRechargeLogs(user.Id, weekStart)
		if err != nil {
			common.SysError(fmt.Sprintf("failed to count weekly recharge for user %d: %s", user.Id, err.Error()))
			return QuotaPoolAutoRechargeResult{Reason: "count_weekly_failed"}
		}
		if weekCount >= int64(policy.WeeklyLimit) {
			return QuotaPoolAutoRechargeResult{Reason: "weekly_limited"}
		}
	}

	if policy.MonthlyLimit > 0 {
		monthCount, err := model.CountAutoRechargeLogs(user.Id, monthStart)
		if err != nil {
			common.SysError(fmt.Sprintf("failed to count monthly recharge for user %d: %s", user.Id, err.Error()))
			return QuotaPoolAutoRechargeResult{Reason: "count_monthly_failed"}
		}
		if monthCount >= int64(policy.MonthlyLimit) {
			return QuotaPoolAutoRechargeResult{Reason: "monthly_limited"}
		}
	}

	if pool == nil || pool.IsDefault || user.QuotaPoolId == model.QuotaPoolDefaultUserPoolId {
		if err := model.IncreaseUserQuota(user.Id, policy.AmountQuota, true); err != nil {
			common.SysError(fmt.Sprintf("failed to increase quota for user %d: %s", user.Id, err.Error()))
			return QuotaPoolAutoRechargeResult{Reason: "increase_user_quota_failed"}
		}
		model.RecordLog(user.Id, model.LogTypeSystem, formatAutoRechargeLog(pool, policy.AmountQuota))
		return QuotaPoolAutoRechargeResult{Recharged: true, Amount: policy.AmountQuota}
	}

	transfer, err := model.TransferQuotaFromPoolToUser(pool.Id, user.Id, policy.AmountQuota)
	if err != nil {
		return QuotaPoolAutoRechargeResult{Reason: err.Error()}
	}
	model.RecordLog(user.Id, model.LogTypeSystem, formatAutoRechargeLog(pool, policy.AmountQuota))
	if transfer != nil && transfer.PoolChanged {
		model.RecordQuotaPoolTransaction(
			pool.Id,
			model.QuotaPoolTransactionAllocateAuto,
			transfer.Change.Amount,
			transfer.Change.QuotaBefore,
			transfer.Change.QuotaAfter,
			user.Id,
			0,
		)
	}
	return QuotaPoolAutoRechargeResult{Recharged: true, Amount: policy.AmountQuota}
}
