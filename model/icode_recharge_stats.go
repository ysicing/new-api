package model

import (
	"sort"
	"time"
)

type rechargeCountRow struct {
	UserId    int `gorm:"column:user_id"`
	AutoCount int `gorm:"column:auto_count"`
	TempCount int `gorm:"column:temp_count"`
}

func GetRechargeLeaderboard(limit int) ([]UserRechargeStat, error) {
	return GetRechargeLeaderboardAt(limit, time.Now())
}

// GetRechargeLeaderboardAt 使用统一的统计时点生成周榜，保证后台快照中
// 各榜单边界一致，也便于在跨周边界时稳定复现结果。
func GetRechargeLeaderboardAt(limit int, now time.Time) ([]UserRechargeStat, error) {
	limit = normalizeStatsLimit(limit)
	weekStart := statsWeekStart(now).Unix()
	countRows, err := groupedRechargeCounts(weekStart, now.Unix())
	if err != nil {
		return nil, err
	}
	if len(countRows) == 0 {
		return []UserRechargeStat{}, nil
	}
	autoCounts := make(map[int]int, len(countRows))
	tempCounts := make(map[int]int, len(countRows))
	ids := make([]int, 0, len(countRows))
	for _, row := range countRows {
		autoCounts[row.UserId] = row.AutoCount
		tempCounts[row.UserId] = row.TempCount
		ids = append(ids, row.UserId)
	}
	usage, err := aggregateOperationsUsage(weekStart, now.Unix(), "", ids)
	if err != nil {
		return nil, err
	}
	users, err := statsUsersById(ids)
	if err != nil {
		return nil, err
	}
	result := make([]UserRechargeStat, 0, len(ids))
	for _, id := range ids {
		user, ok := users[id]
		if !ok {
			continue
		}
		autoCount, tempCount := autoCounts[id], tempCounts[id]
		result = append(result, UserRechargeStat{
			UserQuotaStat: userQuotaStat(user, usage[id]),
			TotalCount:    autoCount + tempCount, AutoRechargeCount: autoCount, TempQuotaCount: tempCount,
		})
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].TotalCount != result[j].TotalCount {
			return result[i].TotalCount > result[j].TotalCount
		}
		if result[i].UsedQuota != result[j].UsedQuota {
			return result[i].UsedQuota > result[j].UsedQuota
		}
		return result[i].UserId < result[j].UserId
	})
	if len(result) > limit {
		result = result[:limit]
	}
	return result, nil
}

// groupedRechargeCounts 只从结构化交易表统计额度池充值，避免扫描原始日志。
func groupedRechargeCounts(since, end int64) ([]rechargeCountRow, error) {
	const selectSQL = `user_id,
COALESCE(SUM(CASE WHEN type = ? THEN 1 ELSE 0 END), 0) AS auto_count,
COALESCE(SUM(CASE WHEN type = ? THEN 1 ELSE 0 END), 0) AS temp_count`
	var rows []rechargeCountRow
	if err := DB.Model(&QuotaPoolTransaction{}).
		Select(selectSQL, QuotaPoolTransactionAllocateAuto, QuotaPoolTransactionAllocateManual).
		Where("created_at >= ? AND created_at <= ?", since, end).
		Where("type IN ?", []string{QuotaPoolTransactionAllocateAuto, QuotaPoolTransactionAllocateManual}).
		Group("user_id").
		Scan(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

func statsWeekStart(now time.Time) time.Time {
	weekday := int(now.Weekday())
	if weekday == 0 {
		weekday = 7
	}
	return time.Date(now.Year(), now.Month(), now.Day()-weekday+1, 0, 0, 0, 0, now.Location())
}
