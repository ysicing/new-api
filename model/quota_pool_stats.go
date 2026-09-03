package model

import (
	"errors"
	"sort"
	"strconv"
	"time"

	"github.com/QuantumNous/new-api/common"
)

const quotaPoolStatsMemberBatchSize = 500

var ErrQuotaPoolStatsTimezoneUnsupported = errors.New("quota pool statistics require a whole-hour timezone offset")
var errInvalidQuotaPoolStatsGranularity = errors.New("invalid quota pool statistics granularity")

type quotaPoolHourlyUsageRow struct {
	UserId        int
	CreatedAt     int64
	RequestCount  int
	TokenUsed     int64
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
	lastTrendKey string
	lastActive   int64
}

func GetQuotaPoolStats(poolId int, startTimestamp, endTimestamp int64, granularity QuotaPoolStatsGranularity) (*QuotaPoolStats, error) {
	return GetQuotaPoolStatsInLocation(poolId, startTimestamp, endTimestamp, granularity, common.BeijingTimeLocation)
}

func GetQuotaPoolStatsInLocation(poolId int, startTimestamp, endTimestamp int64, granularity QuotaPoolStatsGranularity, location *time.Location) (*QuotaPoolStats, error) {
	if location == nil {
		location = common.BeijingTimeLocation
	}
	if granularity != QuotaPoolStatsGranularityHour && granularity != QuotaPoolStatsGranularityDay && granularity != QuotaPoolStatsGranularityWeek {
		return nil, errInvalidQuotaPoolStatsGranularity
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
	trend, trendIndexByKey := buildQuotaPoolTrend(startTimestamp, endTimestamp, granularity, location)
	stats := &QuotaPoolStats{
		Granularity: granularity, StartTimestamp: startTimestamp, EndTimestamp: endTimestamp,
		StartTime: formatQuotaPoolStatsTimestamp(startTimestamp, location), EndTime: formatQuotaPoolStatsTimestamp(endTimestamp, location),
		Usage: []QuotaPoolUsageStat{}, Members: []QuotaPoolMemberStat{}, Trend: trend,
		Recharge: []QuotaPoolRechargeStat{}, Summary: QuotaPoolStatsSummary{MemberCount: len(members)}, TimeZone: location.String(),
	}
	activities := make(map[int]*quotaPoolMemberActivity, len(members))
	for _, member := range members {
		activities[member.Id] = &quotaPoolMemberActivity{}
	}
	err = walkQuotaPoolHourlyUsage(ids, bucketStart, bucketEndExclusive, func(row quotaPoolHourlyUsageRow) {
		rowTime := time.Unix(row.CreatedAt, 0).In(location)
		dateKey := rowTime.Format(time.DateOnly)
		trendKey := quotaPoolTrendKey(rowTime, granularity)
		trendIndex, exists := trendIndexByKey[trendKey]
		if !exists {
			return
		}
		stats.Trend[trendIndex].RequestCount += row.RequestCount
		stats.Trend[trendIndex].TokenUsed += row.TokenUsed
		stats.Trend[trendIndex].UsedQuota += row.UsedQuota
		bucket := usage[row.UserId]
		bucket.RequestCount += row.RequestCount
		bucket.TokenUsed += row.TokenUsed
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
		}
		if activity.lastTrendKey != trendKey {
			activity.lastTrendKey = trendKey
			stats.Trend[trendIndex].ActiveMembers++
		}
		if row.CreatedAt > activity.lastActive {
			activity.lastActive = row.CreatedAt
		}
	})
	if err != nil {
		return nil, err
	}
	for index := range stats.Trend {
		stats.Trend[index].ActiveRate = quotaPoolPercentage(stats.Trend[index].ActiveMembers, len(members))
	}

	for _, member := range members {
		bucket := usage[member.Id]
		activity := activities[member.Id]
		memberStat := QuotaPoolMemberStat{
			QuotaPoolUsageStat: QuotaPoolUsageStat{
				UserId: member.Id, Username: member.Username, RequestCount: bucket.RequestCount, TokenUsed: bucket.TokenUsed, UsedQuota: bucket.UsedQuota,
				GptQuota: bucket.GptQuota, ClaudeQuota: bucket.ClaudeQuota, DeepSeekQuota: bucket.DeepSeekQuota,
				GeminiQuota: bucket.GeminiQuota, QwenQuota: bucket.QwenQuota, OtherQuota: bucket.OtherQuota,
			},
			Active: activity.activeDays > 0, ActiveDays: activity.activeDays, LastActiveAt: activity.lastActive,
		}
		if memberStat.LastActiveAt > 0 {
			memberStat.LastActiveTime = time.Unix(memberStat.LastActiveAt, 0).In(location).Format("2006-01-02 15:04:05 -07:00 MST")
		}
		if memberStat.ActiveDays > 0 {
			memberStat.AverageDailyTokens = float64(memberStat.TokenUsed) / float64(memberStat.ActiveDays)
			memberStat.AverageDailyUsage = float64(memberStat.UsedQuota) / float64(memberStat.ActiveDays)
			stats.Summary.ActiveMembers++
		}
		stats.Summary.RequestCount += memberStat.RequestCount
		stats.Summary.TotalTokens += memberStat.TokenUsed
		stats.Summary.TotalUsage += memberStat.UsedQuota
		stats.Members = append(stats.Members, memberStat)
	}
	stats.Summary.ActiveRate = quotaPoolPercentage(stats.Summary.ActiveMembers, stats.Summary.MemberCount)
	if stats.Summary.ActiveMembers > 0 {
		stats.Summary.AverageTokensPerActiveMember = float64(stats.Summary.TotalTokens) / float64(stats.Summary.ActiveMembers)
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

func buildQuotaPoolTrend(startTimestamp, endTimestamp int64, granularity QuotaPoolStatsGranularity, location *time.Location) ([]QuotaPoolTrendStat, map[string]int) {
	trend := make([]QuotaPoolTrendStat, 0)
	indexByKey := make(map[string]int)
	start := time.Unix(startTimestamp, 0).In(location)
	var cursor time.Time
	var advance func(time.Time) time.Time
	switch granularity {
	case QuotaPoolStatsGranularityHour:
		cursor = time.Unix(startTimestamp-startTimestamp%3600, 0).In(location)
		advance = func(value time.Time) time.Time { return value.Add(time.Hour) }
	case QuotaPoolStatsGranularityDay:
		cursor = time.Date(start.Year(), start.Month(), start.Day(), 0, 0, 0, 0, location)
		advance = func(value time.Time) time.Time { return value.AddDate(0, 0, 1) }
	case QuotaPoolStatsGranularityWeek:
		weekday := int(start.Weekday())
		if weekday == 0 {
			weekday = 7
		}
		cursor = time.Date(start.Year(), start.Month(), start.Day()-weekday+1, 0, 0, 0, 0, location)
		advance = func(value time.Time) time.Time { return value.AddDate(0, 0, 7) }
	}
	for cursor.Unix() <= endTimestamp {
		next := advance(cursor)
		bucketStart := max(startTimestamp, cursor.Unix())
		bucketEnd := min(endTimestamp, next.Unix()-1)
		label := cursor.Format(time.DateOnly)
		if granularity == QuotaPoolStatsGranularityHour {
			label = cursor.Format("2006-01-02 15:00 -07:00")
		} else if granularity == QuotaPoolStatsGranularityWeek {
			label = cursor.Format(time.DateOnly) + " — " + next.AddDate(0, 0, -1).Format(time.DateOnly)
		}
		key := quotaPoolTrendKey(cursor, granularity)
		indexByKey[key] = len(trend)
		trend = append(trend, QuotaPoolTrendStat{BucketStart: bucketStart, BucketEnd: bucketEnd, Label: label})
		cursor = next
	}
	return trend, indexByKey
}

func quotaPoolTrendKey(value time.Time, granularity QuotaPoolStatsGranularity) string {
	switch granularity {
	case QuotaPoolStatsGranularityHour:
		return strconv.FormatInt(value.Unix()-value.Unix()%3600, 10)
	case QuotaPoolStatsGranularityWeek:
		weekday := int(value.Weekday())
		if weekday == 0 {
			weekday = 7
		}
		return time.Date(value.Year(), value.Month(), value.Day()-weekday+1, 0, 0, 0, 0, value.Location()).Format(time.DateOnly)
	default:
		return value.Format(time.DateOnly)
	}
}

func formatQuotaPoolStatsTimestamp(timestamp int64, location *time.Location) string {
	return time.Unix(timestamp, 0).In(location).Format("2006-01-02 15:04:05 -07:00 MST")
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
				&row.UserId, &row.CreatedAt, &row.RequestCount, &row.TokenUsed, &row.UsedQuota,
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
