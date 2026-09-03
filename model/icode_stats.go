package model

import "sort"

type usageAggregateRow struct {
	UserId        int `gorm:"column:user_id"`
	RequestCount  int `gorm:"column:request_count"`
	UsedQuota     int `gorm:"column:used_quota"`
	GptQuota      int `gorm:"column:gpt_quota"`
	ClaudeQuota   int `gorm:"column:claude_quota"`
	DeepSeekQuota int `gorm:"column:deepseek_quota"`
	GeminiQuota   int `gorm:"column:gemini_quota"`
	QwenQuota     int `gorm:"column:qwen_quota"`
	OtherQuota    int `gorm:"column:other_quota"`
}

const usageAggregateMetricsSelect = `COALESCE(SUM(count), 0) AS request_count,
COALESCE(SUM(quota), 0) AS used_quota,
COALESCE(SUM(CASE WHEN COALESCE(LOWER(model_name), '') LIKE '%gpt%'
  OR COALESCE(LOWER(model_name), '') LIKE 'o1%'
  OR COALESCE(LOWER(model_name), '') LIKE 'o3%'
  THEN quota ELSE 0 END), 0) AS gpt_quota,
COALESCE(SUM(CASE WHEN COALESCE(LOWER(model_name), '') LIKE '%claude%'
  THEN quota ELSE 0 END), 0) AS claude_quota,
COALESCE(SUM(CASE WHEN COALESCE(LOWER(model_name), '') LIKE '%deepseek%'
  THEN quota ELSE 0 END), 0) AS deepseek_quota,
COALESCE(SUM(CASE WHEN COALESCE(LOWER(model_name), '') LIKE '%gemini%'
  THEN quota ELSE 0 END), 0) AS gemini_quota,
COALESCE(SUM(CASE WHEN COALESCE(LOWER(model_name), '') LIKE '%qwen%'
  THEN quota ELSE 0 END), 0) AS qwen_quota,
COALESCE(SUM(CASE WHEN NOT (
  COALESCE(LOWER(model_name), '') LIKE '%gpt%'
  OR COALESCE(LOWER(model_name), '') LIKE 'o1%'
  OR COALESCE(LOWER(model_name), '') LIKE 'o3%'
  OR COALESCE(LOWER(model_name), '') LIKE '%claude%'
  OR COALESCE(LOWER(model_name), '') LIKE '%deepseek%'
  OR COALESCE(LOWER(model_name), '') LIKE '%gemini%'
  OR COALESCE(LOWER(model_name), '') LIKE '%qwen%'
) THEN quota ELSE 0 END), 0) AS other_quota`

const usageAggregateSelect = "user_id,\n" + usageAggregateMetricsSelect

func GetTopUsers(startTimestamp, endTimestamp int64, modelName string, limit int) ([]UserQuotaStat, error) {
	limit = normalizeStatsLimit(limit)
	if startTimestamp > 0 && endTimestamp > 0 && endTimestamp-startTimestamp > 30*24*60*60 {
		startTimestamp = endTimestamp - 30*24*60*60
	}
	usage, err := aggregateOperationsUsage(startTimestamp, endTimestamp, modelName, nil)
	if err != nil {
		return nil, err
	}
	ids := make([]int, 0, len(usage))
	for userId, bucket := range usage {
		if bucket.UsedQuota != 0 {
			ids = append(ids, userId)
		}
	}
	sort.Slice(ids, func(i, j int) bool {
		if usage[ids[i]].UsedQuota != usage[ids[j]].UsedQuota {
			return usage[ids[i]].UsedQuota > usage[ids[j]].UsedQuota
		}
		return ids[i] < ids[j]
	})
	if len(ids) > limit {
		ids = ids[:limit]
	}
	users, err := statsUsersById(ids)
	if err != nil {
		return nil, err
	}
	result := make([]UserQuotaStat, 0, len(ids))
	for _, userId := range ids {
		user, ok := users[userId]
		if !ok {
			continue
		}
		result = append(result, userQuotaStat(user, usage[userId]))
	}
	return result, nil
}

// aggregateOperationsUsage 只读取按小时落库的 quota_data 聚合数据。
// 实时性由数据落库周期和上层五分钟缓存共同约束，避免统计请求扫描原始日志。
func aggregateOperationsUsage(startTimestamp, endTimestamp int64, modelName string, userIds []int) (map[int]usageBucket, error) {
	query := DB.Table("quota_data").Select(usageAggregateSelect)
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
	if err := query.Group("user_id").Scan(&rows).Error; err != nil {
		return nil, err
	}
	result := make(map[int]usageBucket, len(rows))
	for _, row := range rows {
		result[row.UserId] = row.bucket()
	}
	return result, nil
}

func (row usageAggregateRow) bucket() usageBucket {
	return usageBucket{
		RequestCount: row.RequestCount, UsedQuota: row.UsedQuota, GptQuota: row.GptQuota, ClaudeQuota: row.ClaudeQuota,
		DeepSeekQuota: row.DeepSeekQuota, GeminiQuota: row.GeminiQuota,
		QwenQuota: row.QwenQuota, OtherQuota: row.OtherQuota,
	}
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
