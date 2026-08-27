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
	countRows, err := groupedRechargeCounts(weekStart, operationsStatsLogTailStart(weekStart, now.Unix()), now.Unix())
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

// groupedRechargeCounts 从结构化交易表统计额度池充值；日志库只补算默认池
// 自动充值和旧格式临时额度的最近尾部，避免按周扫描整张日志表。
func groupedRechargeCounts(since, logTailStart, end int64) ([]rechargeCountRow, error) {
	const selectSQL = `user_id,
COALESCE(SUM(CASE WHEN type = ? THEN 1 ELSE 0 END), 0) AS auto_count,
COALESCE(SUM(CASE WHEN type = ? THEN 1 ELSE 0 END), 0) AS temp_count`
	var transactionRows []rechargeCountRow
	if err := DB.Model(&QuotaPoolTransaction{}).
		Select(selectSQL, QuotaPoolTransactionAllocateAuto, QuotaPoolTransactionAllocateManual).
		Where("created_at >= ? AND created_at <= ?", since, end).
		Where("type IN ?", []string{QuotaPoolTransactionAllocateAuto, QuotaPoolTransactionAllocateManual}).
		Group("user_id").
		Scan(&transactionRows).Error; err != nil {
		return nil, err
	}

	countsByUser := make(map[int]rechargeCountRow, len(transactionRows))
	for _, row := range transactionRows {
		countsByUser[row.UserId] = row
	}

	const tailSelectSQL = `user_id,
COALESCE(SUM(CASE WHEN type = ? AND other LIKE ? AND other LIKE ? THEN 1 ELSE 0 END), 0) AS auto_count,
COALESCE(SUM(CASE WHEN type = ? AND content LIKE ? THEN 1 ELSE 0 END), 0) AS temp_count`
	const tailCondition = `(type = ? AND other LIKE ? AND other LIKE ?) OR (type = ? AND content LIKE ?)`
	autoSourcePattern := `%"recharge_source":"auto"%`
	defaultPoolPattern := `%"quota_pool_id":0%`
	tempContentPattern := "%临时额度%"
	var tailRows []rechargeCountRow
	if err := LOG_DB.Model(&Log{}).
		Select(tailSelectSQL,
			LogTypeSystem, autoSourcePattern, defaultPoolPattern,
			LogTypeManage, tempContentPattern,
		).
		Where("created_at >= ? AND created_at <= ?", logTailStart, end).
		Where(tailCondition,
			LogTypeSystem, autoSourcePattern, defaultPoolPattern,
			LogTypeManage, tempContentPattern,
		).
		Group("user_id").
		Scan(&tailRows).Error; err != nil {
		return nil, err
	}
	for _, row := range tailRows {
		count := countsByUser[row.UserId]
		count.UserId = row.UserId
		count.AutoCount += row.AutoCount
		count.TempCount += row.TempCount
		countsByUser[row.UserId] = count
	}

	rows := make([]rechargeCountRow, 0, len(countsByUser))
	for _, row := range countsByUser {
		rows = append(rows, row)
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
