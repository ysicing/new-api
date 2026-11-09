package model

import (
	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
)

func CountAutoRechargeLogs(userId int, sinceTimestamp int64) (int64, error) {
	var count int64
	err := LOG_DB.Model(&Log{}).
		Where("user_id = ? AND type IN ? AND created_at >= ?", userId, []int{LogTypeSystem, LogTypeTopup}, sinceTimestamp).
		Where("content LIKE ? OR content LIKE ? OR other LIKE ?", "系统自动赠送%", "额度池%自动赠送%", `%"recharge_source":"auto"%`).
		Count(&count).Error
	return count, err
}

func RecordAutoRechargeLog(userId, poolId, amount, operatorId int, poolName string) error {
	content := common.NewMessage("System automatically granted {{quota}}", map[string]any{"quota": logger.FormatQuota(amount)})
	if poolId > 0 {
		content = common.NewMessage("Quota pool {{pool}} automatically granted {{quota}}", map[string]any{"pool": poolName, "quota": logger.FormatQuota(amount)})
	}
	username, _ := GetUsernameById(userId, false)
	other := NewLogOther()
	other.MergePublic(map[string]any{
		"recharge_source": "auto",
		"quota_pool_id":   poolId,
		"quota_pool_name": poolName,
		"amount":          amount,
		"operator_id":     operatorId,
	})
	return createLog(&Log{
		UserId: userId, Username: username, CreatedAt: common.GetTimestamp(),
		Type: LogTypeTopup, Content: other.setContent([]*common.Message{content}), Quota: amount,
		Other: other.JSONString(),
	})
}
