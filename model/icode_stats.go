package model

import (
	"sort"
	"strings"
)

type usageAggregateRow struct {
	UserId    int    `gorm:"column:user_id"`
	ModelName string `gorm:"column:model_name"`
	Quota     int    `gorm:"column:quota"`
}

func GetTopUsers(startTimestamp, endTimestamp int64, modelName string, limit int) ([]UserQuotaStat, error) {
	limit = normalizeStatsLimit(limit)
	if startTimestamp > 0 && endTimestamp > 0 && endTimestamp-startTimestamp > 30*24*60*60 {
		startTimestamp = endTimestamp - 30*24*60*60
	}
	usage, err := aggregateUsage(startTimestamp, endTimestamp, modelName, nil)
	if err != nil {
		return nil, err
	}
	ids := sortedUsageUserIds(usage)
	if len(ids) > limit {
		ids = ids[:limit]
	}
	users, err := statsUsersById(ids)
	if err != nil {
		return nil, err
	}
	result := make([]UserQuotaStat, 0, len(ids))
	for _, id := range ids {
		user, ok := users[id]
		if !ok {
			continue
		}
		bucket := usage[id]
		result = append(result, userQuotaStat(user, bucket))
	}
	return result, nil
}

func aggregateUsage(startTimestamp, endTimestamp int64, modelName string, userIds []int) (map[int]usageBucket, error) {
	query := LOG_DB.Model(&Log{}).
		Select("user_id, model_name, COALESCE(SUM(quota), 0) AS quota").
		Where("type = ?", LogTypeConsume)
	if startTimestamp > 0 {
		query = query.Where("created_at >= ?", startTimestamp)
	}
	if endTimestamp > 0 {
		query = query.Where("created_at <= ?", endTimestamp)
	}
	if modelName != "" {
		query = query.Where("model_name = ?", modelName)
	}
	if len(userIds) > 0 {
		query = query.Where("user_id IN ?", userIds)
	}
	var rows []usageAggregateRow
	if err := query.Group("user_id, model_name").Scan(&rows).Error; err != nil {
		return nil, err
	}
	result := make(map[int]usageBucket)
	for _, row := range rows {
		bucket := result[row.UserId]
		bucket.UsedQuota += row.Quota
		addModelFamilyQuota(&bucket, row.ModelName, row.Quota)
		result[row.UserId] = bucket
	}
	return result, nil
}

func addModelFamilyQuota(bucket *usageBucket, modelName string, quota int) {
	name := strings.ToLower(modelName)
	switch {
	case strings.Contains(name, "gpt") || strings.HasPrefix(name, "o1") || strings.HasPrefix(name, "o3"):
		bucket.GptQuota += quota
	case strings.Contains(name, "claude"):
		bucket.ClaudeQuota += quota
	case strings.Contains(name, "deepseek"):
		bucket.DeepSeekQuota += quota
	case strings.Contains(name, "gemini"):
		bucket.GeminiQuota += quota
	case strings.Contains(name, "qwen"):
		bucket.QwenQuota += quota
	default:
		bucket.OtherQuota += quota
	}
}

func sortedUsageUserIds(usage map[int]usageBucket) []int {
	ids := make([]int, 0, len(usage))
	for id, bucket := range usage {
		if bucket.UsedQuota != 0 {
			ids = append(ids, id)
		}
	}
	sort.Slice(ids, func(i, j int) bool {
		if usage[ids[i]].UsedQuota == usage[ids[j]].UsedQuota {
			return ids[i] < ids[j]
		}
		return usage[ids[i]].UsedQuota > usage[ids[j]].UsedQuota
	})
	return ids
}

func statsUsersById(ids []int) (map[int]User, error) {
	usersById := make(map[int]User, len(ids))
	if len(ids) == 0 {
		return usersById, nil
	}
	var users []User
	if err := DB.Select("id", "username", "quota", "used_quota").Where("id IN ?", ids).Find(&users).Error; err != nil {
		return nil, err
	}
	for _, user := range users {
		usersById[user.Id] = user
	}
	return usersById, nil
}

func userQuotaStat(user User, bucket usageBucket) UserQuotaStat {
	return UserQuotaStat{
		UserId: user.Id, Username: user.Username, RemainingQuota: user.Quota,
		TotalQuota: user.Quota + user.UsedQuota, UsedQuota: bucket.UsedQuota,
		GptQuota: bucket.GptQuota, ClaudeQuota: bucket.ClaudeQuota,
		DeepSeekQuota: bucket.DeepSeekQuota, GeminiQuota: bucket.GeminiQuota,
		QwenQuota: bucket.QwenQuota, OtherQuota: bucket.OtherQuota,
	}
}

func normalizeStatsLimit(limit int) int {
	if limit <= 0 {
		return 10
	}
	if limit > 30 {
		return 30
	}
	return limit
}
