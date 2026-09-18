package model

// quotaPoolMemberRechargeStat 的金额使用内部额度单位；充值不会改变成员活跃状态。
type quotaPoolMemberRechargeStat struct {
	AutoCount   int64
	ManualCount int64
	Amount      int64
}

// 普通池只统计成功拨付流水，不再叠加对应充值日志；存量默认池没有池拨付流水，改读日志库。
// 分批按当前成员 ID 聚合，避免跨库关联和超出数据库参数上限。
func loadQuotaPoolMemberRechargeStats(poolID int, memberIDs []int, start, end int64, systemDefault bool) (map[int]quotaPoolMemberRechargeStat, error) {
	result := make(map[int]quotaPoolMemberRechargeStat, len(memberIDs))
	for offset := 0; offset < len(memberIDs); offset += quotaPoolStatsMemberBatchSize {
		ids := memberIDs[offset:min(offset+quotaPoolStatsMemberBatchSize, len(memberIDs))]
		if systemDefault {
			var rows []struct {
				UserId int
				Count  int64
				Amount int64
			}
			err := LOG_DB.Model(&Log{}).
				Select("user_id, COUNT(*) AS count, COALESCE(SUM(quota), 0) AS amount").
				Where("user_id IN ? AND type IN ? AND created_at >= ? AND created_at <= ?", ids, []int{LogTypeSystem, LogTypeTopup}, start, end).
				Where("quota >= ?", 0).
				Where("content LIKE ? OR (other LIKE ? AND (other LIKE ? OR other LIKE ?))", "系统自动赠送%", `%"recharge_source":"auto"%`, `%"quota_pool_id":0,%`, `%"quota_pool_id":0}%`).
				Group("user_id").Scan(&rows).Error
			if err != nil {
				return nil, err
			}
			for _, row := range rows {
				result[row.UserId] = quotaPoolMemberRechargeStat{AutoCount: row.Count, Amount: row.Amount}
			}
			continue
		}
		var rows []struct {
			UserId int
			Type   string
			Count  int64
			Amount int64
		}
		err := DB.Model(&QuotaPoolTransaction{}).
			Select("user_id, type, COUNT(*) AS count, COALESCE(SUM(amount), 0) AS amount").
			Where("pool_id = ? AND user_id IN ? AND type IN ? AND amount < 0 AND created_at >= ? AND created_at <= ?", poolID, ids, []string{QuotaPoolTransactionAllocateAuto, QuotaPoolTransactionAllocateManual}, start, end).
			Group("user_id, type").Scan(&rows).Error
		if err != nil {
			return nil, err
		}
		for _, row := range rows {
			stat := result[row.UserId]
			if row.Type == QuotaPoolTransactionAllocateAuto {
				stat.AutoCount += row.Count
			} else {
				stat.ManualCount += row.Count
			}
			stat.Amount -= row.Amount
			result[row.UserId] = stat
		}
	}
	return result, nil
}
