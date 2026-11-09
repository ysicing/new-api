package model

import "time"

type DailyActiveUserStat struct {
	Date        string `json:"date"`
	ActiveUsers int    `json:"active_users"`
}

type ActiveUserStats struct {
	TotalActiveUsers int                   `json:"total_active_users"`
	Daily            []DailyActiveUserStat `json:"daily"`
}

// GetActiveUserStats 从 quota_data 小时聚合表计算活跃用户，不扫描原始日志。
// 用户在同一自然日内有至少一条 count > 0 的模型调用聚合记录即计为活跃。
func GetActiveUserStats(startTimestamp, endTimestamp int64, location *time.Location) (*ActiveUserStats, error) {
	if location == nil {
		location = time.Local
	}
	bucketStart := startTimestamp - startTimestamp%3600
	bucketEndExclusive := endTimestamp - endTimestamp%3600 + 3600
	totalUsers, err := countDistinctActiveUsers(bucketStart, bucketEndExclusive)
	if err != nil {
		return nil, err
	}
	startDate := time.Unix(startTimestamp, 0).In(location)
	startDate = time.Date(startDate.Year(), startDate.Month(), startDate.Day(), 0, 0, 0, 0, location)
	endDate := time.Unix(endTimestamp, 0).In(location)
	endDate = time.Date(endDate.Year(), endDate.Month(), endDate.Day(), 0, 0, 0, 0, location)
	daily := make([]DailyActiveUserStat, 0, int(endDate.Sub(startDate).Hours()/24)+1)
	cursor := bucketStart
	for date := startDate; !date.After(endDate); date = date.AddDate(0, 0, 1) {
		nextDate := date.AddDate(0, 0, 1)
		dayEndExclusive := min(bucketEndExclusive, ceilActiveUserHour(nextDate.Unix()))
		activeUsers := 0
		if dayEndExclusive > cursor {
			activeUsers, err = countDistinctActiveUsers(cursor, dayEndExclusive)
		}
		if err != nil {
			return nil, err
		}
		daily = append(daily, DailyActiveUserStat{
			Date: date.Format(time.DateOnly), ActiveUsers: activeUsers,
		})
		cursor = dayEndExclusive
	}
	return &ActiveUserStats{TotalActiveUsers: totalUsers, Daily: daily}, nil
}

func ceilActiveUserHour(timestamp int64) int64 {
	if timestamp%3600 == 0 {
		return timestamp
	}
	return timestamp + 3600 - timestamp%3600
}

func countDistinctActiveUsers(startTimestamp, endTimestampExclusive int64) (int, error) {
	var count int64
	err := DB.Table("quota_data").
		Where("user_id > 0 AND count > 0 AND created_at >= ? AND created_at < ?", startTimestamp, endTimestampExclusive).
		Distinct("user_id").Count(&count).Error
	return int(count), err
}
