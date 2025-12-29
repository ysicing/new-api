package service

import (
	"fmt"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
)

// AutoRechargeTask 自动充值定时任务
func AutoRechargeTask() {
	// 计算转换后的quota值用于日志显示
	thresholdQuota := int(float64(common.AutoRechargeThreshold) * common.QuotaPerUnit)
	amountQuota := int(float64(common.AutoRechargeAmount) * common.QuotaPerUnit)

	common.SysLog(fmt.Sprintf("auto recharge task started, interval: %d minutes, threshold: $%d (%s), amount: $%d (%s)",
		common.AutoRechargeInterval,
		common.AutoRechargeThreshold, logger.LogQuota(thresholdQuota),
		common.AutoRechargeAmount, logger.LogQuota(amountQuota)))

	for {
		checkAndRechargeUsers()
		time.Sleep(time.Duration(common.AutoRechargeInterval) * time.Minute)
	}
}

// checkAndRechargeUsers 检查并充值用户额度
func checkAndRechargeUsers() {
	defer func() {
		if r := recover(); r != nil {
			common.SysError(fmt.Sprintf("auto recharge task panic: %v", r))
		}
	}()

	common.SysLog("starting auto recharge check")

	// 查询所有启用的用户
	var users []model.User
	err := model.DB.Where("status = ?", common.UserStatusEnabled).Find(&users).Error
	if err != nil {
		common.SysError(fmt.Sprintf("failed to get users for auto recharge: %s", err.Error()))
		return
	}

	// 将配置的美元金额转换为内部 quota 单位
	rechargeThreshold := int(float64(common.AutoRechargeThreshold) * common.QuotaPerUnit)
	rechargeQuota := int(float64(common.AutoRechargeAmount) * common.QuotaPerUnit)

	rechargedCount := 0
	for _, user := range users {
		// 获取用户当前额度
		quota, err := model.GetUserQuota(user.Id, true)
		if err != nil {
			common.SysError(fmt.Sprintf("failed to get quota for user %d: %s", user.Id, err.Error()))
			continue
		}

		// 检查是否需要充值
		if quota <= rechargeThreshold {
			// 执行充值
			_ = model.IncreaseUserQuota(user.Id, rechargeQuota, true)
			model.RecordLog(user.Id, model.LogTypeSystem, fmt.Sprintf("系统自动赠送 %s", logger.LogQuota(rechargeQuota)))

			rechargedCount++
			common.SysLog(fmt.Sprintf("auto recharged user %d (%s), old quota: %d, recharged: %d, new quota: %d",
				user.Id, user.Username, quota, rechargeQuota, quota+rechargeQuota))
		}
	}

	if rechargedCount > 0 {
		common.SysLog(fmt.Sprintf("auto recharge completed, total recharged: %d users", rechargedCount))
	} else {
		common.SysLog("auto recharge check completed, no users need recharge")
	}
}
