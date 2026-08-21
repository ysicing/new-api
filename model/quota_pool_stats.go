package model

import "sort"

func GetQuotaPoolStats(poolId int, startTimestamp, endTimestamp int64) (*QuotaPoolStats, error) {
	var members []User
	if err := DB.Select("id", "username").Where("quota_pool_id = ?", poolId).Find(&members).Error; err != nil {
		return nil, err
	}
	ids := make([]int, 0, len(members))
	names := make(map[int]string, len(members))
	for _, member := range members {
		ids = append(ids, member.Id)
		names[member.Id] = member.Username
	}
	usage, err := aggregateUsage(startTimestamp, endTimestamp, "", ids)
	if err != nil {
		return nil, err
	}
	stats := &QuotaPoolStats{Usage: []QuotaPoolUsageStat{}, Recharge: []QuotaPoolRechargeStat{}}
	for userId, bucket := range usage {
		if bucket.UsedQuota == 0 {
			continue
		}
		stats.Usage = append(stats.Usage, QuotaPoolUsageStat{
			UserId: userId, Username: names[userId], UsedQuota: bucket.UsedQuota,
			GptQuota: bucket.GptQuota, ClaudeQuota: bucket.ClaudeQuota,
			DeepSeekQuota: bucket.DeepSeekQuota, GeminiQuota: bucket.GeminiQuota,
			QwenQuota: bucket.QwenQuota, OtherQuota: bucket.OtherQuota,
		})
		stats.TotalUsage += bucket.UsedQuota
	}
	sort.Slice(stats.Usage, func(i, j int) bool {
		if stats.Usage[i].UsedQuota == stats.Usage[j].UsedQuota {
			return stats.Usage[i].UserId < stats.Usage[j].UserId
		}
		return stats.Usage[i].UsedQuota > stats.Usage[j].UsedQuota
	})
	if err := loadQuotaPoolRechargeStats(poolId, startTimestamp, endTimestamp, stats); err != nil {
		return nil, err
	}
	return stats, nil
}

func loadQuotaPoolRechargeStats(poolId int, startTimestamp, endTimestamp int64, stats *QuotaPoolStats) error {
	query := DB.Model(&QuotaPoolTransaction{}).
		Select("type, COUNT(*) AS count, COALESCE(SUM(amount), 0) AS amount").
		Where("pool_id = ?", poolId)
	if startTimestamp > 0 {
		query = query.Where("created_at >= ?", startTimestamp)
	}
	if endTimestamp > 0 {
		query = query.Where("created_at <= ?", endTimestamp)
	}
	if err := query.Group("type").Order("type ASC").Scan(&stats.Recharge).Error; err != nil {
		return err
	}
	for _, item := range stats.Recharge {
		switch item.Type {
		case QuotaPoolTransactionAllocateAuto, QuotaPoolTransactionAllocateManual:
			stats.TotalAllocate += -item.Amount
		case QuotaPoolTransactionInitialFund, QuotaPoolTransactionManualRefill, QuotaPoolTransactionMonthlyRefill:
			stats.TotalRefill += item.Amount
		}
	}
	return nil
}
