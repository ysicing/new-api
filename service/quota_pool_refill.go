package service

import (
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
)

type MonthlyQuotaPoolRefillResult struct {
	Refilled int `json:"refilled"`
	Skipped  int `json:"skipped"`
}

func RefillMonthlyQuotaPools(now time.Time) (MonthlyQuotaPoolRefillResult, error) {
	result := MonthlyQuotaPoolRefillResult{}
	now = now.In(common.BeijingTimeLocation)
	currentMonth := now.Year()*100 + int(now.Month())
	var pools []model.QuotaPool
	err := model.DB.Where(
		"pool_type = ? AND enabled = ? AND monthly_refill_enabled = ? AND monthly_refill_amount > ? AND monthly_refill_day <= ? AND last_refill_month <> ?",
		model.QuotaPoolTypeNormal, true, true, 0, now.Day(), currentMonth,
	).Find(&pools).Error
	if err != nil {
		return result, err
	}
	for _, pool := range pools {
		refilled, err := model.RefillMonthlyQuotaPool(pool.Id, currentMonth, now.Day())
		if err != nil {
			return result, err
		}
		if refilled {
			result.Refilled++
		} else {
			result.Skipped++
		}
	}
	return result, nil
}
