package model

import (
	"errors"
	"strings"

	"gorm.io/gorm"
)

var quotaPoolConfigColumns = map[string]struct{}{
	"name": {}, "base_quota": {}, "auto_recharge_amount": {},
	"weekly_limit": {}, "monthly_limit": {}, "monthly_refill_enabled": {},
	"monthly_refill_top_up": {}, "monthly_refill_amount": {}, "monthly_refill_day": {},
}

func UpdateQuotaPoolConfig(poolId int, updates map[string]any, operatorId int) (*QuotaPoolBalanceChange, error) {
	if len(updates) == 0 {
		return nil, nil
	}
	for key := range updates {
		if _, ok := quotaPoolConfigColumns[key]; !ok {
			return nil, ErrQuotaPoolPermissionDenied
		}
	}
	var change *QuotaPoolBalanceChange
	err := DB.Transaction(func(tx *gorm.DB) error {
		var pool QuotaPool
		if err := lockForUpdate(tx).Where("id = ?", poolId).First(&pool).Error; err != nil {
			return mapQuotaPoolRecordError(err)
		}
		if pool.IsSystemPool() {
			return ErrQuotaPoolSystemReadonly
		}
		if nameValue, ok := updates["name"]; ok {
			name, ok := nameValue.(string)
			if !ok || strings.TrimSpace(name) == "" {
				return ErrQuotaPoolNameExists
			}
			name = strings.TrimSpace(name)
			var count int64
			if err := tx.Model(&QuotaPool{}).Where("name = ? AND id <> ?", name, poolId).Count(&count).Error; err != nil {
				return err
			}
			if count > 0 {
				return ErrQuotaPoolNameExists
			}
			updates["name"] = name
		}
		if baseValue, ok := updates["base_quota"]; ok {
			baseQuota, ok := baseValue.(int)
			if !ok || baseQuota <= 0 {
				return ErrQuotaPoolInvalidAmount
			}
			delta := baseQuota - pool.BaseQuota
			if pool.Quota+delta < 0 {
				return ErrQuotaPoolAdjustLimited
			}
			change = &QuotaPoolBalanceChange{
				PoolId: poolId, Amount: delta,
				QuotaBefore: pool.Quota, QuotaAfter: pool.Quota + delta,
			}
			updates["quota"] = gorm.Expr("quota + ?", delta)
		}
		if err := tx.Model(&QuotaPool{}).Where("id = ?", poolId).Updates(updates).Error; err != nil {
			return err
		}
		if change == nil || change.Amount == 0 {
			return nil
		}
		return tx.Create(&QuotaPoolTransaction{
			PoolId: poolId, Type: QuotaPoolTransactionAdjustBase,
			Amount: change.Amount, QuotaBefore: change.QuotaBefore, QuotaAfter: change.QuotaAfter,
			OperatorId: operatorId,
		}).Error
	})
	if err != nil {
		return nil, err
	}
	return change, nil
}

func SetQuotaPoolEnabled(poolId int, enabled bool) error {
	var pool QuotaPool
	if err := DB.Where("id = ?", poolId).First(&pool).Error; err != nil {
		return mapQuotaPoolRecordError(err)
	}
	if pool.IsSystemPool() {
		return ErrQuotaPoolSystemReadonly
	}
	return DB.Model(&pool).Update("enabled", enabled).Error
}

func DeleteQuotaPool(poolId int) error {
	return DB.Transaction(func(tx *gorm.DB) error {
		var pool QuotaPool
		if err := lockForUpdate(tx).Where("id = ?", poolId).First(&pool).Error; err != nil {
			return mapQuotaPoolRecordError(err)
		}
		if pool.IsSystemPool() {
			return ErrQuotaPoolSystemReadonly
		}
		var members int64
		if err := tx.Model(&User{}).Where("quota_pool_id = ?", poolId).Count(&members).Error; err != nil {
			return err
		}
		if members > 0 {
			return ErrQuotaPoolHasMembers
		}
		if err := tx.Where("pool_id = ?", poolId).Delete(&QuotaPoolAdmin{}).Error; err != nil {
			return err
		}
		return tx.Delete(&pool).Error
	})
}

func ListQuotaPools() ([]QuotaPool, error) {
	var pools []QuotaPool
	err := DB.Order("is_default DESC, id ASC").Find(&pools).Error
	return pools, err
}

func IsQuotaPoolNotFound(err error) bool {
	return errors.Is(err, ErrQuotaPoolNotFound)
}
