package operation_setting

import (
	"errors"
	"math"
	"strconv"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/config"
	"github.com/shopspring/decimal"
)

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

// ValidateOption 在副本上验证单项更新，避免无效金额或间隔先写入数据库。
func (setting AutoRechargeSetting) ValidateOption(key, value string) error {
	if key == "auto_recharge_setting.enabled" {
		enabled, err := strconv.ParseBool(value)
		if err != nil {
			return err
		}
		// 即使存量配置异常，也必须允许管理员关闭自动充值。
		if !enabled {
			return nil
		}
		setting.Enabled = enabled
	} else {
		amount, err := strconv.Atoi(value)
		if err != nil {
			return err
		}
		switch key {
		case "auto_recharge_setting.interval":
			setting.Interval = amount
		case "auto_recharge_setting.threshold":
			setting.Threshold = amount
		case "auto_recharge_setting.amount":
			setting.Amount = amount
		case "auto_recharge_setting.weekly_limit":
			setting.WeeklyLimit = amount
		case "auto_recharge_setting.monthly_limit":
			setting.MonthlyLimit = amount
		default:
			return errors.New("invalid auto recharge setting")
		}
	}
	_, _, err := setting.QuotaAmounts()
	return err
}

// QuotaAmounts 严格转换钱包金额；配置异常时拒绝充值，不使用单请求额度的 int32 饱和值。
func (setting *AutoRechargeSetting) QuotaAmounts() (threshold, amount int, err error) {
	if setting == nil || setting.Threshold < 0 || setting.Threshold > common.MaxWalletQuota ||
		setting.Amount < 0 || setting.Amount > common.MaxWalletQuota ||
		setting.WeeklyLimit < 0 || setting.WeeklyLimit > common.MaxWalletQuota ||
		setting.MonthlyLimit < 0 || setting.MonthlyLimit > common.MaxWalletQuota ||
		setting.Interval <= 0 || int64(setting.Interval) > math.MaxInt64/int64(time.Minute) ||
		math.IsNaN(common.QuotaPerUnit) || math.IsInf(common.QuotaPerUnit, 0) || common.QuotaPerUnit <= 0 {
		return 0, 0, errors.New("invalid auto recharge setting")
	}
	unit := decimal.NewFromFloat(common.QuotaPerUnit)
	threshold, err = common.WalletQuotaFromDecimalStrict(decimal.NewFromInt(int64(setting.Threshold)).Mul(unit).Truncate(0))
	if err != nil {
		return 0, 0, err
	}
	amount, err = common.WalletQuotaFromDecimalStrict(decimal.NewFromInt(int64(setting.Amount)).Mul(unit).Truncate(0))
	if err != nil {
		return 0, 0, err
	}
	return threshold, amount, nil
}
