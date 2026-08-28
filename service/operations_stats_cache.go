package service

import (
	"fmt"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"golang.org/x/sync/singleflight"
)

const (
	operationsStatsCacheTTL = 5 * time.Minute
	operationsStatsLimit    = 30
	operationsStatsCacheNS  = "new-api:operations_stats:v1"
)

// OperationsStatsUserSection 保存用户榜及其实际生成时间。
type OperationsStatsUserSection struct {
	GeneratedAt int64                 `json:"generated_at"`
	Items       []model.UserQuotaStat `json:"items"`
}

// OperationsStatsRechargeSection 保存充值榜及其实际生成时间。
type OperationsStatsRechargeSection struct {
	GeneratedAt int64                    `json:"generated_at"`
	Items       []model.UserRechargeStat `json:"items"`
}

type ModelStatisticsSection struct {
	GeneratedAt int64                  `json:"generated_at"`
	Items       []model.ModelUsageStat `json:"items"`
}

type operationsStatsCacheEntry[T any] struct {
	GeneratedAt int64 `json:"generated_at"`
	Data        T     `json:"data"`
}

var operationsStatsCacheGroup singleflight.Group

func GetOperationsTopUsers(period string, now time.Time) (*OperationsStatsUserSection, error) {
	start := statsWeekStart(now)
	cacheSuffix := fmt.Sprintf("top_users:week:%d", start.Unix())
	if period == "month" {
		start = now.AddDate(0, -1, 0)
		cacheSuffix = "top_users:month"
	}
	entry, err := loadOperationsStatsCache(cacheSuffix, now, func() ([]model.UserQuotaStat, error) {
		return model.GetTopUsers(start.Unix(), now.Unix(), "", operationsStatsLimit)
	})
	if err != nil {
		return nil, err
	}
	return &OperationsStatsUserSection{GeneratedAt: entry.GeneratedAt, Items: entry.Data}, nil
}

func GetOperationsRechargeLeaderboard(now time.Time) (*OperationsStatsRechargeSection, error) {
	weekStart := statsWeekStart(now)
	cacheSuffix := fmt.Sprintf("recharge:week:%d", weekStart.Unix())
	entry, err := loadOperationsStatsCache(cacheSuffix, now, func() ([]model.UserRechargeStat, error) {
		return model.GetRechargeLeaderboardAt(operationsStatsLimit, now)
	})
	if err != nil {
		return nil, err
	}
	return &OperationsStatsRechargeSection{GeneratedAt: entry.GeneratedAt, Items: entry.Data}, nil
}

func GetCachedQuotaPoolStats(poolId int, startTimestamp, endTimestamp int64, now time.Time) (*model.QuotaPoolStats, int64, error) {
	cacheSuffix := fmt.Sprintf("quota_pool:%d:%d:%d", poolId, startTimestamp, endTimestamp)
	entry, err := loadOperationsStatsCache(cacheSuffix, now, func() (*model.QuotaPoolStats, error) {
		return model.GetQuotaPoolStats(poolId, startTimestamp, endTimestamp)
	})
	if err != nil {
		return nil, 0, err
	}
	return entry.Data, entry.GeneratedAt, nil
}

func GetCachedModelStatistics(period string, userId int, now time.Time) (*ModelStatisticsSection, error) {
	start := statsWeekStart(now)
	periodKey := fmt.Sprintf("week:%d", start.Unix())
	if period == "month" {
		start = now.AddDate(0, 0, -30)
		periodKey = "month"
	}
	scopeKey := "all"
	if userId > 0 {
		scopeKey = fmt.Sprintf("user:%d", userId)
	}
	cacheSuffix := fmt.Sprintf("model_statistics:%s:%s", periodKey, scopeKey)
	entry, err := loadOperationsStatsCache(cacheSuffix, now, func() ([]model.ModelUsageStat, error) {
		return model.GetModelStatistics(start.Unix(), now.Unix(), userId)
	})
	if err != nil {
		return nil, err
	}
	return &ModelStatisticsSection{GeneratedAt: entry.GeneratedAt, Items: entry.Data}, nil
}

func loadOperationsStatsCache[T any](suffix string, now time.Time, loader func() (T, error)) (operationsStatsCacheEntry[T], error) {
	key := operationsStatsCacheNS + ":" + suffix
	if entry, ok := readOperationsStatsCache[T](key); ok {
		return entry, nil
	}

	value, err, _ := operationsStatsCacheGroup.Do(key, func() (any, error) {
		if entry, ok := readOperationsStatsCache[T](key); ok {
			return entry, nil
		}
		data, err := loader()
		if err != nil {
			return operationsStatsCacheEntry[T]{}, err
		}
		entry := operationsStatsCacheEntry[T]{GeneratedAt: now.Unix(), Data: data}
		writeOperationsStatsCache(key, entry)
		return entry, nil
	})
	if err != nil {
		return operationsStatsCacheEntry[T]{}, err
	}
	return value.(operationsStatsCacheEntry[T]), nil
}

func readOperationsStatsCache[T any](key string) (operationsStatsCacheEntry[T], bool) {
	if !common.RedisEnabled || common.RDB == nil {
		return operationsStatsCacheEntry[T]{}, false
	}
	raw, err := common.RedisGet(key)
	if err != nil {
		return operationsStatsCacheEntry[T]{}, false
	}
	var entry operationsStatsCacheEntry[T]
	if err := common.UnmarshalJsonStr(raw, &entry); err != nil {
		return operationsStatsCacheEntry[T]{}, false
	}
	return entry, true
}

func writeOperationsStatsCache[T any](key string, entry operationsStatsCacheEntry[T]) {
	if !common.RedisEnabled || common.RDB == nil {
		return
	}
	data, err := common.Marshal(entry)
	if err != nil {
		return
	}
	_ = common.RedisSet(key, string(data), operationsStatsCacheTTL)
}

func statsWeekStart(now time.Time) time.Time {
	weekday := int(now.Weekday())
	if weekday == 0 {
		weekday = 7
	}
	return time.Date(now.Year(), now.Month(), now.Day()-weekday+1, 0, 0, 0, 0, now.Location())
}
