package service

import (
	"errors"
	"time"

	"github.com/QuantumNous/new-api/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type MonthlyQuotaPoolRefillResult struct {
	Refilled int `json:"refilled"`
	Skipped  int `json:"skipped"`
}

func RefillMonthlyQuotaPools(now time.Time) (MonthlyQuotaPoolRefillResult, error) {
	result := MonthlyQuotaPoolRefillResult{}
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
		refilled, err := refillMonthlyQuotaPool(pool.Id, currentMonth, now.Day())
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

func refillMonthlyQuotaPool(poolId, currentMonth, currentDay int) (bool, error) {
	refilled := false
	err := model.DB.Transaction(func(tx *gorm.DB) error {
		var pool model.QuotaPool
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ? AND enabled = ? AND monthly_refill_enabled = ? AND monthly_refill_amount > ? AND monthly_refill_day <= ? AND last_refill_month <> ?", poolId, true, true, 0, currentDay, currentMonth).
			First(&pool).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		if err != nil {
			return err
		}
		amount := pool.MonthlyRefillAmount
		if pool.MonthlyRefillTopUp {
			amount = max(pool.MonthlyRefillAmount-pool.Quota, 0)
		}
		updates := map[string]any{"last_refill_month": currentMonth}
		if amount > 0 {
			updates["quota"] = gorm.Expr("quota + ?", amount)
			updates["base_quota"] = gorm.Expr("base_quota + ?", amount)
		}
		updated := tx.Model(&model.QuotaPool{}).
			Where("id = ? AND last_refill_month <> ?", pool.Id, currentMonth).
			Updates(updates)
		if updated.Error != nil || updated.RowsAffected == 0 {
			return updated.Error
		}
		refilled = true
		if amount == 0 {
			return nil
		}
		return tx.Create(&model.QuotaPoolTransaction{
			PoolId: pool.Id, Type: model.QuotaPoolTransactionMonthlyRefill,
			Amount: amount, QuotaBefore: pool.Quota, QuotaAfter: pool.Quota + amount,
		}).Error
	})
	return refilled, err
}
