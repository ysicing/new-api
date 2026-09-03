package model

import (
	"errors"
	"sort"
	"time"
)

const quotaPoolStatsMemberBatchSize = 500

var ErrQuotaPoolStatsTimezoneUnsupported = errors.New("quota pool statistics require a whole-hour timezone offset")

type quotaPoolHourlyUsageRow struct {
	UserId        int
	CreatedAt     int64
	RequestCount  int
	UsedQuota     int
	GptQuota      int
	ClaudeQuota   int
	DeepSeekQuota int
	GeminiQuota   int
	QwenQuota     int
	OtherQuota    int
}

type quotaPoolMemberActivity struct {
	activeDays   int
	lastSeenDate string
	lastActive   int64
}

func GetQuotaPoolStats(poolId int, startTimestamp, endTimestamp int64) (*QuotaPoolStats, error) {
	return GetQuotaPoolStatsInLocation(poolId, startTimestamp, endTimestamp, time.Local)
}

func GetQuotaPoolStatsInLocation(poolId int, startTimestamp, endTimestamp int64, location *time.Location) (*QuotaPoolStats, error) {
	if location == nil {
		location = time.Local
	}
	if !quotaPoolStatsLocationSupported(startTimestamp, endTimestamp, location) {
		return nil, ErrQuotaPoolStatsTimezoneUnsupported
	}
	pool, err := GetQuotaPoolById(poolId)
	if err != nil {
		return nil, err
	}
	memberPoolId := poolId
	if pool.IsDefault || pool.PoolType == QuotaPoolTypeDefault {
		memberPoolId = QuotaPoolDefaultUserPoolId
	}
	var members []User
	if err := DB.Select("id", "username").Where("quota_pool_id = ?", memberPoolId).Find(&members).Error; err != nil {
		return nil, err
	}
	ids := make([]int, 0, len(members))
	for _, member := range members {
		ids = append(ids, member.Id)
	}
	bucketStart := startTimestamp - startTimestamp%3600
	bucketEndExclusive := endTimestamp - endTimestamp%3600 + 3600
	usage := make(map[int]usageBucket)
	startDate := time.Unix(startTimestamp, 0).In(location)
	startDate = time.Date(startDate.Year(), startDate.Month(), startDate.Day(), 0, 0, 0, 0, location)
	endDate := time.Unix(endTimestamp, 0).In(location)
	endDate = time.Date(endDate.Year(), endDate.Month(), endDate.Day(), 0, 0, 0, 0, location)
	stats := &QuotaPoolStats{
		Usage: []QuotaPoolUsageStat{}, Members: []QuotaPoolMemberStat{}, Daily: []QuotaPoolDailyStat{},
		Recharge: []QuotaPoolRechargeStat{}, Summary: QuotaPoolStatsSummary{MemberCount: len(members)}, TimeZone: location.String(),
	}
	dailyIndexByDate := make(map[string]int)
	for date := startDate; !date.After(endDate); date = date.AddDate(0, 0, 1) {
		dateKey := date.Format(time.DateOnly)
		stats.Daily = append(stats.Daily, QuotaPoolDailyStat{Date: dateKey})
		dailyIndexByDate[dateKey] = len(stats.Daily) - 1
	}
	activities := make(map[int]*quotaPoolMemberActivity, len(members))
	for _, member := range members {
		activities[member.Id] = &quotaPoolMemberActivity{}
	}
	err = walkQuotaPoolHourlyUsage(ids, bucketStart, bucketEndExclusive, func(row quotaPoolHourlyUsageRow) {
		dateKey := time.Unix(row.CreatedAt, 0).In(location).Format(time.DateOnly)
		dailyIndex, exists := dailyIndexByDate[dateKey]
		if !exists {
			return
		}
		stats.Daily[dailyIndex].RequestCount += row.RequestCount
		stats.Daily[dailyIndex].UsedQuota += row.UsedQuota
		bucket := usage[row.UserId]
		bucket.RequestCount += row.RequestCount
		bucket.UsedQuota += row.UsedQuota
		bucket.GptQuota += row.GptQuota
		bucket.ClaudeQuota += row.ClaudeQuota
		bucket.DeepSeekQuota += row.DeepSeekQuota
		bucket.GeminiQuota += row.GeminiQuota
		bucket.QwenQuota += row.QwenQuota
		bucket.OtherQuota += row.OtherQuota
		usage[row.UserId] = bucket
		if row.RequestCount <= 0 {
			return
		}
		activity := activities[row.UserId]
		if activity.lastSeenDate != dateKey {
			activity.lastSeenDate = dateKey
			activity.activeDays++
			stats.Daily[dailyIndex].ActiveMembers++
		}
		if row.CreatedAt > activity.lastActive {
			activity.lastActive = row.CreatedAt
		}
	})
	if err != nil {
		return nil, err
	}
	for index := range stats.Daily {
		stats.Daily[index].ActiveRate = quotaPoolPercentage(stats.Daily[index].ActiveMembers, len(members))
	}

	for _, member := range members {
		bucket := usage[member.Id]
		activity := activities[member.Id]
		memberStat := QuotaPoolMemberStat{
			QuotaPoolUsageStat: QuotaPoolUsageStat{
				UserId: member.Id, Username: member.Username, RequestCount: bucket.RequestCount, UsedQuota: bucket.UsedQuota,
				GptQuota: bucket.GptQuota, ClaudeQuota: bucket.ClaudeQuota, DeepSeekQuota: bucket.DeepSeekQuota,
				GeminiQuota: bucket.GeminiQuota, QwenQuota: bucket.QwenQuota, OtherQuota: bucket.OtherQuota,
			},
			Active: activity.activeDays > 0, ActiveDays: activity.activeDays, LastActiveAt: activity.lastActive,
		}
		if memberStat.LastActiveAt > 0 {
			memberStat.LastActiveTime = time.Unix(memberStat.LastActiveAt, 0).In(location).Format("2006-01-02 15:04:05 -07:00 MST")
		}
		if memberStat.ActiveDays > 0 {
			memberStat.AverageDailyUsage = float64(memberStat.UsedQuota) / float64(memberStat.ActiveDays)
			stats.Summary.ActiveMembers++
		}
		stats.Summary.RequestCount += memberStat.RequestCount
		stats.Summary.TotalUsage += memberStat.UsedQuota
		stats.Members = append(stats.Members, memberStat)
	}
	stats.Summary.ActiveRate = quotaPoolPercentage(stats.Summary.ActiveMembers, stats.Summary.MemberCount)
	if stats.Summary.ActiveMembers > 0 {
		stats.Summary.AverageUsagePerActiveMember = float64(stats.Summary.TotalUsage) / float64(stats.Summary.ActiveMembers)
	}
	stats.TotalUsage = stats.Summary.TotalUsage
	for index := range stats.Members {
		if stats.Summary.TotalUsage != 0 {
			stats.Members[index].UsageShare = float64(stats.Members[index].UsedQuota) * 100 / float64(stats.Summary.TotalUsage)
		}
		if stats.Members[index].UsedQuota != 0 {
			stats.Usage = append(stats.Usage, stats.Members[index].QuotaPoolUsageStat)
		}
	}
	sort.Slice(stats.Members, func(i, j int) bool {
		if stats.Members[i].UsedQuota == stats.Members[j].UsedQuota {
			return stats.Members[i].UserId < stats.Members[j].UserId
		}
		return stats.Members[i].UsedQuota > stats.Members[j].UsedQuota
	})
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

func quotaPoolStatsLocationSupported(startTimestamp, endTimestamp int64, location *time.Location) bool {
	if endTimestamp < startTimestamp {
		return false
	}
	firstHour := startTimestamp - startTimestamp%3600
	for timestamp := firstHour; ; timestamp += 3600 {
		_, offset := time.Unix(timestamp, 0).In(location).Zone()
		if offset%3600 != 0 {
			return false
		}
		if timestamp > endTimestamp-3600 {
			break
		}
	}
	_, endOffset := time.Unix(endTimestamp, 0).In(location).Zone()
	if endOffset%3600 != 0 {
		return false
	}
	return true
}

func walkQuotaPoolHourlyUsage(userIds []int, startTimestamp, endTimestampExclusive int64, visit func(quotaPoolHourlyUsageRow)) error {
	if len(userIds) == 0 {
		return nil
	}
	for offset := 0; offset < len(userIds); offset += quotaPoolStatsMemberBatchSize {
		end := min(offset+quotaPoolStatsMemberBatchSize, len(userIds))
		rows, err := DB.Table("quota_data").
			Select("user_id, created_at,\n"+usageAggregateMetricsSelect).
			Where("user_id IN ? AND created_at >= ? AND created_at < ?", userIds[offset:end], startTimestamp, endTimestampExclusive).
			Group("user_id, created_at").Order("created_at ASC, user_id ASC").Rows()
		if err != nil {
			return err
		}
		for rows.Next() {
			var row quotaPoolHourlyUsageRow
			if err := rows.Scan(
				&row.UserId, &row.CreatedAt, &row.RequestCount, &row.UsedQuota,
				&row.GptQuota, &row.ClaudeQuota, &row.DeepSeekQuota, &row.GeminiQuota, &row.QwenQuota, &row.OtherQuota,
			); err != nil {
				_ = rows.Close()
				return err
			}
			visit(row)
		}
		if err := rows.Err(); err != nil {
			_ = rows.Close()
			return err
		}
		if err := rows.Close(); err != nil {
			return err
		}
	}
	return nil
}

func quotaPoolPercentage(value, total int) float64 {
	if total <= 0 {
		return 0
	}
	return float64(value) * 100 / float64(total)
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
		case QuotaPoolTransactionReclaimUser:
			stats.TotalReclaim += item.Amount
		case QuotaPoolTransactionInitialFund, QuotaPoolTransactionManualRefill, QuotaPoolTransactionMonthlyRefill:
			stats.TotalRefill += item.Amount
		}
	}
	return nil
}
