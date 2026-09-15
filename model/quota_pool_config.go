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

func validateQuotaPoolPolicyUpdates(pool QuotaPool, updates map[string]any) error {
	if value, ok := updates["auto_recharge_amount"]; ok {
		amount, valid := value.(int)
		if !valid || amount < QuotaPoolAutoRechargeInherit {
			return ErrQuotaPoolInvalidAmount
		}
	}
	for _, field := range []string{"weekly_limit", "monthly_limit"} {
		if value, ok := updates[field]; ok {
			limit, valid := value.(int)
			if !valid || limit < QuotaPoolAutoRechargeInherit {
				return ErrQuotaPoolInvalidAmount
			}
		}
	}
	monthlyEnabled := pool.MonthlyRefillEnabled
	if value, ok := updates["monthly_refill_enabled"]; ok {
		var valid bool
		monthlyEnabled, valid = value.(bool)
		if !valid {
			return ErrQuotaPoolInvalidAmount
		}
	}
	monthlyAmount := pool.MonthlyRefillAmount
	if value, ok := updates["monthly_refill_amount"]; ok {
		var valid bool
		monthlyAmount, valid = value.(int)
		if !valid || monthlyAmount < 0 {
			return ErrQuotaPoolInvalidAmount
		}
	}
	monthlyDay := pool.MonthlyRefillDay
	if value, ok := updates["monthly_refill_day"]; ok {
		var valid bool
		monthlyDay, valid = value.(int)
		if !valid || monthlyDay < 1 || monthlyDay > 28 {
			return ErrQuotaPoolInvalidAmount
		}
	}
	if monthlyEnabled && (monthlyAmount <= 0 || monthlyDay < 1 || monthlyDay > 28) {
		return ErrQuotaPoolInvalidAmount
	}
	return nil
}

// QuotaPoolConfigChange 保存事务内的配置变更快照，金额使用内部额度单位。
type QuotaPoolConfigChange struct {
	Field  string `json:"field"`
	Before any    `json:"before"`
	After  any    `json:"after"`
}

// UpdateQuotaPoolConfig 更新池配置，返回余额变化及实际配置变更快照；失败时不返回审计快照。
func UpdateQuotaPoolConfig(poolId int, updates map[string]any, operatorId int) (*QuotaPoolBalanceChange, []QuotaPoolConfigChange, error) {
	if len(updates) == 0 {
		return nil, nil, nil
	}
	for key := range updates {
		if _, ok := quotaPoolConfigColumns[key]; !ok {
			return nil, nil, ErrQuotaPoolPermissionDenied
		}
	}
	var change *QuotaPoolBalanceChange
	changes := make([]QuotaPoolConfigChange, 0, len(updates))
	err := DB.Transaction(func(tx *gorm.DB) error {
		var pool QuotaPool
		if err := lockForUpdate(tx).Where("id = ?", poolId).First(&pool).Error; err != nil {
			return mapQuotaPoolRecordError(err)
		}
		if pool.IsSystemPool() {
			return ErrQuotaPoolSystemReadonly
		}
		if err := validateQuotaPoolPolicyUpdates(pool, updates); err != nil {
			return err
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
		// 在持有行锁时读取旧值；使用实际变化而非表单提交字段数，避免并发和未修改字段造成误导。
		for _, previous := range []struct {
			field string
			value any
		}{
			{"name", pool.Name}, {"base_quota", pool.BaseQuota},
			{"auto_recharge_amount", pool.AutoRechargeAmount},
			{"weekly_limit", pool.WeeklyLimit}, {"monthly_limit", pool.MonthlyLimit},
			{"monthly_refill_enabled", pool.MonthlyRefillEnabled},
			{"monthly_refill_top_up", pool.MonthlyRefillTopUp},
			{"monthly_refill_amount", pool.MonthlyRefillAmount}, {"monthly_refill_day", pool.MonthlyRefillDay},
		} {
			if next, ok := updates[previous.field]; ok && next != previous.value {
				changes = append(changes, QuotaPoolConfigChange{Field: previous.field, Before: previous.value, After: next})
			}
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
		return nil, nil, err
	}
	return change, changes, nil
}

func SetQuotaPoolEnabled(poolId int, enabled bool) error {
	var pool QuotaPool
	if err := DB.Where("id = ?", poolId).First(&pool).Error; err != nil {
		return mapQuotaPoolRecordError(err)
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
