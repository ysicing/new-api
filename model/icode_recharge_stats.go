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
	countRows, err := groupedRechargeCounts(weekStart)
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
	usage, err := aggregateUsage(weekStart, 0, "", ids)
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

// groupedRechargeCounts 使用一次日志扫描同时统计自动充值和临时额度次数，
// 避免两条带 LIKE 条件的周范围查询重复遍历日志表。
func groupedRechargeCounts(since int64) ([]rechargeCountRow, error) {
	const selectSQL = `user_id,
COALESCE(SUM(CASE WHEN type = ? AND (
  content LIKE ? OR content LIKE ? OR other LIKE ?
) THEN 1 ELSE 0 END), 0) AS auto_count,
COALESCE(SUM(CASE WHEN type = ? AND content LIKE ?
  THEN 1 ELSE 0 END), 0) AS temp_count`
	const matchingCondition = `(type = ? AND (
  content LIKE ? OR content LIKE ? OR other LIKE ?
)) OR (type = ? AND content LIKE ?)`
	autoContentPattern := "系统自动赠送%"
	poolAutoContentPattern := "额度池%自动赠送%"
	autoSourcePattern := `%"recharge_source":"auto"%`
	tempContentPattern := "%临时额度%"
	query := LOG_DB.Model(&Log{}).
		Select(
			selectSQL,
			LogTypeSystem, autoContentPattern, poolAutoContentPattern, autoSourcePattern,
			LogTypeManage, tempContentPattern,
		).
		Where("created_at >= ?", since).
		Where(
			matchingCondition,
			LogTypeSystem, autoContentPattern, poolAutoContentPattern, autoSourcePattern,
			LogTypeManage, tempContentPattern,
		).
		Group("user_id")
	var rows []rechargeCountRow
	if err := query.Scan(&rows).Error; err != nil {
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
