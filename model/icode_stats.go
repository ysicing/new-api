package model

import (
	"sort"

	"github.com/QuantumNous/new-api/common"
)

type usageAggregateRow struct {
	UserId        int `gorm:"column:user_id"`
	UsedQuota     int `gorm:"column:used_quota"`
	GptQuota      int `gorm:"column:gpt_quota"`
	ClaudeQuota   int `gorm:"column:claude_quota"`
	DeepSeekQuota int `gorm:"column:deepseek_quota"`
	GeminiQuota   int `gorm:"column:gemini_quota"`
	QwenQuota     int `gorm:"column:qwen_quota"`
	OtherQuota    int `gorm:"column:other_quota"`
}

const usageAggregateSelect = `user_id,
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

// aggregateOperationsUsage 使用小时聚合表承载历史区间，只从日志库补算最近两个
// 小时桶。额外保留一个完整小时作为安全窗口，覆盖 quota_data 默认五分钟落库
// 以及跨整点时仍停留在各节点内存缓存中的数据。
func aggregateOperationsUsage(startTimestamp, endTimestamp int64, modelName string, userIds []int) (map[int]usageBucket, error) {
	tailStart := operationsStatsLogTailStart(startTimestamp, endTimestamp)
	result := make(map[int]usageBucket)

	if startTimestamp < tailStart {
		query := DB.Table("quota_data").
			Select(usageAggregateSelect).
			Where("created_at >= ? AND created_at < ?", startTimestamp, tailStart)
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
		for _, row := range rows {
			result[row.UserId] = row.bucket()
		}
	}

	rows, err := aggregateUsageRows(tailStart, endTimestamp, modelName, userIds, 0)
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		bucket := result[row.UserId]
		bucket.UsedQuota += row.UsedQuota
		bucket.GptQuota += row.GptQuota
		bucket.ClaudeQuota += row.ClaudeQuota
		bucket.DeepSeekQuota += row.DeepSeekQuota
		bucket.GeminiQuota += row.GeminiQuota
		bucket.QwenQuota += row.QwenQuota
		bucket.OtherQuota += row.OtherQuota
		result[row.UserId] = bucket
	}
	return result, nil
}

func operationsStatsLogTailStart(startTimestamp, endTimestamp int64) int64 {
	if endTimestamp <= 0 {
		endTimestamp = common.GetTimestamp()
	}
	tailStart := endTimestamp - endTimestamp%3600 - 3600
	if tailStart < startTimestamp {
		return startTimestamp
	}
	return tailStart
}

func aggregateUsage(startTimestamp, endTimestamp int64, modelName string, userIds []int) (map[int]usageBucket, error) {
	rows, err := aggregateUsageRows(startTimestamp, endTimestamp, modelName, userIds, 0)
	if err != nil {
		return nil, err
	}
	result := make(map[int]usageBucket, len(rows))
	for _, row := range rows {
		result[row.UserId] = row.bucket()
	}
	return result, nil
}

// aggregateUsageRows 在数据库内按用户完成模型族聚合，并在排行榜场景下
// 下推排序与 LIMIT，避免把整个时间窗口的“用户 × 模型”结果拉回进程排序。
func aggregateUsageRows(startTimestamp, endTimestamp int64, modelName string, userIds []int, limit int) ([]usageAggregateRow, error) {
	query := LOG_DB.Model(&Log{}).
		Select(usageAggregateSelect).
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
	query = query.Group("user_id").Having("COALESCE(SUM(quota), 0) <> 0")
	if limit > 0 {
		query = query.Order("used_quota DESC, user_id ASC").Limit(limit)
	}
	var rows []usageAggregateRow
	if err := query.Scan(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

func (row usageAggregateRow) bucket() usageBucket {
	return usageBucket{
		UsedQuota: row.UsedQuota, GptQuota: row.GptQuota, ClaudeQuota: row.ClaudeQuota,
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
