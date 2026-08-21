package operation_setting

import "github.com/QuantumNous/new-api/setting/config"

type AutoRechargeSetting struct {
	Enabled      bool `json:"enabled"`
	Interval     int  `json:"interval"`
	Threshold    int  `json:"threshold"`
	Amount       int  `json:"amount"`
	WeeklyLimit  int  `json:"weekly_limit"`
	MonthlyLimit int  `json:"monthly_limit"`
}

var autoRechargeSetting = AutoRechargeSetting{
	Enabled:      false,
	Interval:     30,
	Threshold:    50,
	Amount:       200,
	WeeklyLimit:  0,
	MonthlyLimit: 0,
}

func init() {
	config.GlobalConfig.Register("auto_recharge_setting", &autoRechargeSetting)
}

func GetAutoRechargeSetting() *AutoRechargeSetting {
	return &autoRechargeSetting
}
