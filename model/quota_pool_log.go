package model

import (
	"fmt"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
)

func CountAutoRechargeLogs(userId int, sinceTimestamp int64) (int64, error) {
	var count int64
	err := LOG_DB.Model(&Log{}).
		Where("user_id = ? AND type = ? AND created_at >= ?", userId, LogTypeSystem, sinceTimestamp).
		Where("content LIKE ? OR content LIKE ? OR other LIKE ?", "系统自动赠送%", "额度池%自动赠送%", `%"recharge_source":"auto"%`).
		Count(&count).Error
	return count, err
}

func RecordAutoRechargeLog(userId, poolId, amount, operatorId int, poolName string) error {
	content := fmt.Sprintf("系统自动赠送 %s", logger.LogQuota(amount))
	if poolId > 0 {
		content = fmt.Sprintf("额度池“%s”自动赠送 %s", poolName, logger.LogQuota(amount))
	}
	username, _ := GetUsernameById(userId, false)
	other := map[string]any{
		"recharge_source": "auto",
		"quota_pool_id":   poolId,
		"quota_pool_name": poolName,
		"amount":          amount,
		"operator_id":     operatorId,
	}
	return createLog(&Log{
		UserId: userId, Username: username, CreatedAt: common.GetTimestamp(),
		Type: LogTypeSystem, Content: content, Quota: amount,
		Other: common.MapToJsonStr(other),
	})
}
