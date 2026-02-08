package service

import (
	"fmt"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"
)

// AutoRechargeTask 自动充值定时任务
func AutoRechargeTask() {
	for {
		cfg := operation_setting.GetAutoRechargeSetting()
		if !cfg.Enabled {
			time.Sleep(time.Duration(cfg.Interval) * time.Minute)
			continue
		}

		thresholdQuota := int(float64(cfg.Threshold) * common.QuotaPerUnit)
		amountQuota := int(float64(cfg.Amount) * common.QuotaPerUnit)

		common.SysLog(fmt.Sprintf(
			"auto recharge task started, interval: %d min, threshold: $%d (%s), amount: $%d (%s), weekly_limit: %d, monthly_limit: %d",
			cfg.Interval,
			cfg.Threshold, logger.LogQuota(thresholdQuota),
			cfg.Amount, logger.LogQuota(amountQuota),
			cfg.WeeklyLimit, cfg.MonthlyLimit))

		checkAndRechargeUsers(cfg, thresholdQuota, amountQuota)
		time.Sleep(time.Duration(cfg.Interval) * time.Minute)
	}
}

// checkAndRechargeUsers 检查并充值用户额度
func checkAndRechargeUsers(
	cfg *operation_setting.AutoRechargeSetting,
	thresholdQuota, amountQuota int,
) {
	defer func() {
		if r := recover(); r != nil {
			common.SysError(fmt.Sprintf("auto recharge task panic: %v", r))
		}
	}()

	common.SysLog("starting auto recharge check")

	var users []model.User
	err := model.DB.Where("status = ?", common.UserStatusEnabled).Find(&users).Error
	if err != nil {
		common.SysError(fmt.Sprintf("failed to get users for auto recharge: %s", err.Error()))
		return
	}

	now := time.Now()
	weekday := int(now.Weekday())
	if weekday == 0 {
		weekday = 7
	}
	weekStart := time.Date(now.Year(), now.Month(), now.Day()-weekday+1, 0, 0, 0, 0, now.Location()).Unix()
	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location()).Unix()

	rechargedCount := 0
	for _, user := range users {
		quota, err := model.GetUserQuota(user.Id, true)
		if err != nil {
			common.SysError(fmt.Sprintf("failed to get quota for user %d: %s", user.Id, err.Error()))
			continue
		}

		if quota > thresholdQuota {
			continue
		}

		// 检查周限制
		if cfg.WeeklyLimit > 0 {
			weekCount, err := model.CountAutoRechargeLogs(user.Id, weekStart)
			if err != nil {
				common.SysError(fmt.Sprintf("failed to count weekly recharge for user %d: %s", user.Id, err.Error()))
				continue
			}
			if weekCount >= int64(cfg.WeeklyLimit) {
				continue
			}
		}

		// 检查月限制
		if cfg.MonthlyLimit > 0 {
			monthCount, err := model.CountAutoRechargeLogs(user.Id, monthStart)
			if err != nil {
				common.SysError(fmt.Sprintf("failed to count monthly recharge for user %d: %s", user.Id, err.Error()))
				continue
			}
			if monthCount >= int64(cfg.MonthlyLimit) {
				continue
			}
		}

		_ = model.IncreaseUserQuota(user.Id, amountQuota, true)
		model.RecordLog(user.Id, model.LogTypeSystem, fmt.Sprintf("系统自动赠送 %s", logger.LogQuota(amountQuota)))

		rechargedCount++
		common.SysLog(fmt.Sprintf("auto recharged user %d (%s), old quota: %d, recharged: %d",
			user.Id, user.Username, quota, amountQuota))
	}

	if rechargedCount > 0 {
		common.SysLog(fmt.Sprintf("auto recharge completed, total recharged: %d users", rechargedCount))
	} else {
		common.SysLog("auto recharge check completed, no users need recharge")
	}
}
