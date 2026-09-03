package model

type UserQuotaStat struct {
	UserId         int    `json:"user_id"`
	Username       string `json:"username"`
	RemainingQuota int    `json:"remaining_quota"`
	TotalQuota     int    `json:"total_quota"`
	UsedQuota      int    `json:"used_quota"`
	GptQuota       int    `json:"gpt_quota"`
	ClaudeQuota    int    `json:"claude_quota"`
	DeepSeekQuota  int    `json:"deepseek_quota"`
	GeminiQuota    int    `json:"gemini_quota"`
	QwenQuota      int    `json:"qwen_quota"`
	OtherQuota     int    `json:"other_quota"`
}

type UserRechargeStat struct {
	UserQuotaStat
	TotalCount        int `json:"total_count"`
	AutoRechargeCount int `json:"auto_recharge_count"`
	TempQuotaCount    int `json:"temp_quota_count"`
}

type QuotaPoolUsageStat struct {
	UserId        int    `json:"user_id"`
	Username      string `json:"username"`
	RequestCount  int    `json:"request_count"`
	UsedQuota     int    `json:"used_quota"`
	GptQuota      int    `json:"gpt_quota"`
	ClaudeQuota   int    `json:"claude_quota"`
	DeepSeekQuota int    `json:"deepseek_quota"`
	GeminiQuota   int    `json:"gemini_quota"`
	QwenQuota     int    `json:"qwen_quota"`
	OtherQuota    int    `json:"other_quota"`
}

type QuotaPoolMemberStat struct {
	QuotaPoolUsageStat
	Active            bool    `json:"active"`
	ActiveDays        int     `json:"active_days"`
	LastActiveAt      int64   `json:"last_active_at"`
	LastActiveTime    string  `json:"last_active_time"`
	UsageShare        float64 `json:"usage_share"`
	AverageDailyUsage float64 `json:"average_daily_usage"`
}

type QuotaPoolDailyStat struct {
	Date          string  `json:"date"`
	ActiveMembers int     `json:"active_members"`
	ActiveRate    float64 `json:"active_rate"`
	RequestCount  int     `json:"request_count"`
	UsedQuota     int     `json:"used_quota"`
}

type QuotaPoolStatsSummary struct {
	MemberCount                 int     `json:"member_count"`
	ActiveMembers               int     `json:"active_members"`
	ActiveRate                  float64 `json:"active_rate"`
	RequestCount                int     `json:"request_count"`
	TotalUsage                  int     `json:"total_usage"`
	AverageUsagePerActiveMember float64 `json:"average_usage_per_active_member"`
}

type QuotaPoolRechargeStat struct {
	Type   string `json:"type"`
	Count  int    `json:"count"`
	Amount int    `json:"amount"`
}

type QuotaPoolStats struct {
	RangeType      string                  `json:"range_type"`
	StartDate      string                  `json:"start_date"`
	EndDate        string                  `json:"end_date"`
	StartTimestamp int64                   `json:"start_timestamp"`
	EndTimestamp   int64                   `json:"end_timestamp"`
	GeneratedAt    int64                   `json:"generated_at"`
	GeneratedTime  string                  `json:"generated_time"`
	TimeZone       string                  `json:"time_zone"`
	Usage          []QuotaPoolUsageStat    `json:"usage"`
	Members        []QuotaPoolMemberStat   `json:"members"`
	Daily          []QuotaPoolDailyStat    `json:"daily"`
	Summary        QuotaPoolStatsSummary   `json:"summary"`
	Recharge       []QuotaPoolRechargeStat `json:"recharge"`
	TotalUsage     int                     `json:"total_usage"`
	TotalRefill    int                     `json:"total_refill"`
	TotalAllocate  int                     `json:"total_allocate"`
	TotalReclaim   int                     `json:"total_reclaim"`
}

type usageBucket struct {
	RequestCount  int
	UsedQuota     int
	GptQuota      int
	ClaudeQuota   int
	DeepSeekQuota int
	GeminiQuota   int
	QwenQuota     int
	OtherQuota    int
}
