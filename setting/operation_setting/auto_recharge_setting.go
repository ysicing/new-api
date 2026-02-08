package operation_setting

import (
	"os"
	"strconv"

	"github.com/QuantumNous/new-api/setting/config"
)

// AutoRechargeSetting 自动充值配置
type AutoRechargeSetting struct {
	Enabled      bool `json:"enabled"`       // 是否启用自动充值
	Interval     int  `json:"interval"`      // 检查间隔（分钟）
	Threshold    int  `json:"threshold"`     // 触发阈值（美元）
	Amount       int  `json:"amount"`        // 充值金额（美元）
	WeeklyLimit  int  `json:"weekly_limit"`  // 每周限制次数（0=无限制）
	MonthlyLimit int  `json:"monthly_limit"` // 每月限制次数（0=无限制）
}

var autoRechargeSetting = AutoRechargeSetting{
	Enabled:      true,
	Interval:     30,
	Threshold:    50,
	Amount:       200,
	WeeklyLimit:  0,
	MonthlyLimit: 0,
}

func init() {
	// 从环境变量读取初始值（向后兼容）
	if v := os.Getenv("AUTO_RECHARGE_ENABLED"); v != "" {
		autoRechargeSetting.Enabled, _ = strconv.ParseBool(v)
	}
	if v := os.Getenv("AUTO_RECHARGE_INTERVAL"); v != "" {
		autoRechargeSetting.Interval, _ = strconv.Atoi(v)
	}
	if v := os.Getenv("AUTO_RECHARGE_THRESHOLD"); v != "" {
		autoRechargeSetting.Threshold, _ = strconv.Atoi(v)
	}
	if v := os.Getenv("AUTO_RECHARGE_AMOUNT"); v != "" {
		autoRechargeSetting.Amount, _ = strconv.Atoi(v)
	}

	config.GlobalConfig.Register("auto_recharge_setting", &autoRechargeSetting)
}

// GetAutoRechargeSetting 获取自动充值配置
func GetAutoRechargeSetting() *AutoRechargeSetting {
	return &autoRechargeSetting
}

// IsAutoRechargeEnabled 是否启用自动充值
func IsAutoRechargeEnabled() bool {
	return autoRechargeSetting.Enabled
}
