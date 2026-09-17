package model

import (
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/common"
)

func ListQuotaPoolOperationLogs(poolId int, page *common.PageInfo, action string) ([]Log, int64, error) {
	objectEnd := fmt.Sprintf(`%%{"quota_pool_id":%d}%%`, poolId)
	objectMore := fmt.Sprintf(`%%{"quota_pool_id":%d,%%`, poolId)
	fieldEnd := fmt.Sprintf(`%%,"quota_pool_id":%d}%%`, poolId)
	fieldMore := fmt.Sprintf(`%%,"quota_pool_id":%d,%%`, poolId)
	query := LOG_DB.Model(&Log{}).
		Where("type IN ?", []int{LogTypeManage, LogTypeTopup}).
		Where("(other LIKE ? OR other LIKE ? OR other LIKE ? OR other LIKE ?)", objectEnd, objectMore, fieldEnd, fieldMore)

	if action = strings.TrimSpace(action); action != "" {
		encoded, err := common.Marshal(action)
		if err != nil {
			return nil, 0, err
		}
		// 审计写入使用紧凑 JSON；匹配 op.action 的完整字符串，避免动作前缀或 LIKE 通配符误匹配。
		pattern := strings.NewReplacer("!", "!!", "%", "!%", "_", "!_").Replace(`"op":{"action":` + string(encoded))
		query = query.Where("(other LIKE ? ESCAPE '!' OR other LIKE ? ESCAPE '!')", "%"+pattern+",%", "%"+pattern+"}%")
	}
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
