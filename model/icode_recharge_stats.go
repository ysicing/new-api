package model

import (
	"sort"
	"time"
)

type rechargeCountRow struct {
	UserId int `gorm:"column:user_id"`
	Count  int `gorm:"column:count"`
}

func GetRechargeLeaderboard(limit int) ([]UserRechargeStat, error) {
	limit = normalizeStatsLimit(limit)
	weekStart := statsWeekStart(time.Now()).Unix()
	autoCounts, err := groupedRechargeCounts(
		LogTypeSystem, weekStart,
		"content LIKE ? OR content LIKE ? OR other LIKE ?",
		"系统自动赠送%", "额度池%自动赠送%", `%"recharge_source":"auto"%`,
	)
	if err != nil {
		return nil, err
	}
	tempCounts, err := groupedRechargeCounts(LogTypeManage, weekStart, "content LIKE ?", "%临时额度%")
	if err != nil {
		return nil, err
	}
	ids := rechargeCandidateIds(autoCounts, tempCounts)
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

func groupedRechargeCounts(logType int, since int64, condition string, args ...any) (map[int]int, error) {
	query := LOG_DB.Model(&Log{}).
		Select("user_id, COUNT(*) AS count").
		Where("type = ? AND created_at >= ?", logType, since).
		Where(condition, args...).Group("user_id")
	var rows []rechargeCountRow
	if err := query.Scan(&rows).Error; err != nil {
		return nil, err
	}
	result := make(map[int]int, len(rows))
	for _, row := range rows {
		result[row.UserId] = row.Count
	}
	return result, nil
}

func rechargeCandidateIds(first, second map[int]int) []int {
	seen := make(map[int]struct{}, len(first)+len(second))
	for id := range first {
		seen[id] = struct{}{}
	}
	for id := range second {
		seen[id] = struct{}{}
	}
	ids := make([]int, 0, len(seen))
	for id := range seen {
		ids = append(ids, id)
	}
	return ids
}

func statsWeekStart(now time.Time) time.Time {
	weekday := int(now.Weekday())
	if weekday == 0 {
		weekday = 7
	}
	return time.Date(now.Year(), now.Month(), now.Day()-weekday+1, 0, 0, 0, 0, now.Location())
}
