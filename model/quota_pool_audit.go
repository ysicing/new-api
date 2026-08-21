package model

import (
	"fmt"

	"github.com/QuantumNous/new-api/common"
)

func ListQuotaPoolOperationLogs(poolId int, page *common.PageInfo) ([]Log, int64, error) {
	pattern := fmt.Sprintf(`%%"quota_pool_id":%d%%`, poolId)
	query := LOG_DB.Model(&Log{}).Where("type = ? AND other LIKE ?", LogTypeManage, pattern)
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var logs []Log
	if err := query.Order("id DESC").Offset(page.GetStartIdx()).Limit(page.GetPageSize()).Find(&logs).Error; err != nil {
		return nil, 0, err
	}
	return logs, total, nil
}
