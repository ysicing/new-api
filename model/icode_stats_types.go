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
	UsedQuota     int    `json:"used_quota"`
	GptQuota      int    `json:"gpt_quota"`
	ClaudeQuota   int    `json:"claude_quota"`
	DeepSeekQuota int    `json:"deepseek_quota"`
	GeminiQuota   int    `json:"gemini_quota"`
	QwenQuota     int    `json:"qwen_quota"`
	OtherQuota    int    `json:"other_quota"`
}

type QuotaPoolRechargeStat struct {
	Type   string `json:"type"`
	Count  int    `json:"count"`
	Amount int    `json:"amount"`
}

type QuotaPoolStats struct {
	Usage         []QuotaPoolUsageStat    `json:"usage"`
	Recharge      []QuotaPoolRechargeStat `json:"recharge"`
	TotalUsage    int                     `json:"total_usage"`
	TotalRefill   int                     `json:"total_refill"`
	TotalAllocate int                     `json:"total_allocate"`
}

type usageBucket struct {
	UsedQuota     int
	GptQuota      int
	ClaudeQuota   int
	DeepSeekQuota int
	GeminiQuota   int
	QwenQuota     int
	OtherQuota    int
}
