package model

import (
	"errors"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)

func GetQuotaPoolById(poolId int) (*QuotaPool, error) {
	var pool QuotaPool
	if err := DB.Where("id = ?", poolId).First(&pool).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrQuotaPoolNotFound
		}
		return nil, err
	}
	return &pool, nil
}

func CreateQuotaPool(name string, baseQuota, operatorId int) (*QuotaPool, error) {
	name = strings.TrimSpace(name)
	if name == "" || baseQuota <= 0 || baseQuota > common.MaxWalletQuota {
		return nil, ErrQuotaPoolInvalidAmount
	}
	pool := &QuotaPool{
		Name: name, PoolType: QuotaPoolTypeNormal, Enabled: true,
		BaseQuota: baseQuota, Quota: baseQuota,
		AutoRechargeAmount: QuotaPoolAutoRechargeInherit,
		WeeklyLimit:        QuotaPoolAutoRechargeInherit,
		MonthlyLimit:       QuotaPoolAutoRechargeInherit,
		MonthlyRefillDay:   1,
	}
	err := DB.Transaction(func(tx *gorm.DB) error {
		var count int64
		if err := tx.Model(&QuotaPool{}).Where("name = ?", name).Count(&count).Error; err != nil {
			return err
		}
		if count > 0 {
			return ErrQuotaPoolNameExists
		}
		if err := tx.Create(pool).Error; err != nil {
			return err
		}
		return tx.Create(&QuotaPoolTransaction{
			PoolId: pool.Id, Type: QuotaPoolTransactionInitialFund,
			Amount: baseQuota, QuotaBefore: 0, QuotaAfter: baseQuota,
			OperatorId: operatorId,
		}).Error
	})
	if err != nil {
		return nil, err
	}
	return pool, nil
}

func AddQuotaPoolManualRefill(poolId, amount, operatorId int) (*QuotaPoolBalanceChange, error) {
	if amount <= 0 || amount > common.MaxWalletQuota {
		return nil, ErrQuotaPoolInvalidAmount
	}
	change := &QuotaPoolBalanceChange{PoolId: poolId, Amount: amount}
	now := time.Now()
	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location()).Unix()
	nextMonth := time.Date(now.Year(), now.Month()+1, 1, 0, 0, 0, 0, now.Location()).Unix()
	err := DB.Transaction(func(tx *gorm.DB) error {
		var pool QuotaPool
		if err := lockForUpdate(tx).Where("id = ?", poolId).First(&pool).Error; err != nil {
			return mapQuotaPoolRecordError(err)
		}
		if pool.IsSystemPool() {
			return ErrQuotaPoolSystemReadonly
		}
		if !pool.Enabled {
			return ErrQuotaPoolDisabled
		}
		if pool.BaseQuota <= 0 {
			return ErrQuotaPoolRefillBaseInvalid
		}
		if pool.BaseQuota > common.MaxWalletQuota || pool.Quota < 0 || pool.Quota > common.MaxWalletQuota {
			return ErrQuotaPoolInvalidAmount
		}
		if pool.Quota > common.MaxWalletQuota-amount || pool.BaseQuota > common.MaxWalletQuota-amount {
			return ErrWalletQuotaLimitExceeded
		}
		if amount > pool.BaseQuota/2 {
			return ErrQuotaPoolRefillAmountLimited
		}
		var count int64
		if err := tx.Model(&QuotaPoolTransaction{}).
			Where("pool_id = ? AND type = ? AND created_at >= ? AND created_at < ?", poolId, QuotaPoolTransactionManualRefill, monthStart, nextMonth).
			Count(&count).Error; err != nil {
			return err
		}
		if count >= QuotaPoolMonthlyManualRefillLimit {
			return ErrQuotaPoolRefillMonthlyLimited
		}
		change.QuotaBefore = pool.Quota
		change.QuotaAfter = pool.Quota + amount
		credit := tx.Model(&QuotaPool{}).
			Where("id = ? AND quota >= 0 AND quota <= ? AND base_quota <= ?", poolId, common.MaxWalletQuota-amount, common.MaxWalletQuota-amount).
			Updates(map[string]any{
				"quota": gorm.Expr("quota + ?", amount), "base_quota": gorm.Expr("base_quota + ?", amount),
			})
		if credit.Error != nil {
			return credit.Error
		}
		if credit.RowsAffected != 1 {
			return ErrWalletQuotaLimitExceeded
		}
		return tx.Create(&QuotaPoolTransaction{
			PoolId: poolId, Type: QuotaPoolTransactionManualRefill,
			Amount: amount, QuotaBefore: change.QuotaBefore, QuotaAfter: change.QuotaAfter,
			OperatorId: operatorId,
		}).Error
	})
	if err != nil {
		return nil, err
	}
	return change, nil
}

// DeductQuotaPoolBalance 手动扣减额度池的当前可用额度，不改变基础额度。
func DeductQuotaPoolBalance(poolId, amount, operatorId int) (*QuotaPoolBalanceChange, error) {
	if amount <= 0 || amount > common.MaxWalletQuota {
		return nil, ErrQuotaPoolInvalidAmount
	}
	change := &QuotaPoolBalanceChange{PoolId: poolId, Amount: -amount}
	err := DB.Transaction(func(tx *gorm.DB) error {
		var pool QuotaPool
		if err := lockForUpdate(tx).Where("id = ?", poolId).First(&pool).Error; err != nil {
			return mapQuotaPoolRecordError(err)
		}
		if pool.IsSystemPool() {
			return ErrQuotaPoolSystemReadonly
		}
		if !pool.Enabled {
			return ErrQuotaPoolDisabled
		}
		if pool.Quota < 0 || pool.Quota > common.MaxWalletQuota {
			return ErrQuotaPoolInvalidAmount
		}
		if pool.Quota < amount {
			return ErrQuotaPoolInsufficientQuota
		}
		change.QuotaBefore = pool.Quota
		change.QuotaAfter = pool.Quota - amount
		if err := tx.Model(&QuotaPool{}).Where("id = ?", poolId).
			Update("quota", gorm.Expr("quota - ?", amount)).Error; err != nil {
			return err
		}
		return tx.Create(&QuotaPoolTransaction{
			PoolId: poolId, Type: QuotaPoolTransactionManualDeduct,
			Amount: -amount, QuotaBefore: change.QuotaBefore, QuotaAfter: change.QuotaAfter,
			OperatorId: operatorId,
		}).Error
	})
	if err != nil {
		return nil, err
	}
	return change, nil
}

// RefillMonthlyQuotaPool 将月度幂等标记、资金和流水一并提交，复用各数据库的行锁规则。
func RefillMonthlyQuotaPool(poolId, currentMonth, currentDay int) (bool, error) {
	refilled := false
	err := DB.Transaction(func(tx *gorm.DB) error {
		var pool QuotaPool
		err := lockForUpdate(tx).
			Where("id = ? AND pool_type = ? AND enabled = ? AND monthly_refill_enabled = ? AND monthly_refill_day <= ? AND last_refill_month <> ?",
				poolId, QuotaPoolTypeNormal, true, true, currentDay, currentMonth).
			First(&pool).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		if err != nil {
			return err
		}
		if pool.MonthlyRefillAmount <= 0 || pool.MonthlyRefillAmount > common.MaxWalletQuota ||
			pool.MonthlyRefillDay < 1 || pool.MonthlyRefillDay > 28 ||
			pool.Quota < 0 || pool.Quota > common.MaxWalletQuota ||
			pool.BaseQuota < 0 || pool.BaseQuota > common.MaxWalletQuota {
			return ErrQuotaPoolInvalidAmount
		}
		amount := pool.MonthlyRefillAmount
		if pool.MonthlyRefillTopUp {
			amount = max(pool.MonthlyRefillAmount-pool.Quota, 0)
		}
		if pool.Quota > common.MaxWalletQuota-amount || pool.BaseQuota > common.MaxWalletQuota-amount {
			return ErrWalletQuotaLimitExceeded
		}
		updates := map[string]any{"last_refill_month": currentMonth}
		if amount > 0 {
			updates["quota"] = gorm.Expr("quota + ?", amount)
			updates["base_quota"] = gorm.Expr("base_quota + ?", amount)
		}
		updated := tx.Model(&QuotaPool{}).
			Where("id = ? AND last_refill_month <> ? AND quota >= 0 AND quota <= ? AND base_quota >= 0 AND base_quota <= ?",
				pool.Id, currentMonth, common.MaxWalletQuota-amount, common.MaxWalletQuota-amount).
			Updates(updates)
		if updated.Error != nil {
			return updated.Error
		}
		if updated.RowsAffected == 0 {
			return nil
		}
		refilled = true
		if amount == 0 {
			return nil
		}
		return tx.Create(&QuotaPoolTransaction{
			PoolId: pool.Id, Type: QuotaPoolTransactionMonthlyRefill,
			Amount: amount, QuotaBefore: pool.Quota, QuotaAfter: pool.Quota + amount,
		}).Error
	})
	if err != nil {
		return false, err
	}
	return refilled, nil
}

func GrantQuotaPoolAdmin(poolId, userId int) error {
	return DB.Transaction(func(tx *gorm.DB) error {
		var user User
		if err := lockForUpdate(tx).Where("id = ?", userId).First(&user).Error; err != nil {
			return err
		}
		var pool QuotaPool
		if err := lockForUpdate(tx).Where("id = ?", poolId).First(&pool).Error; err != nil {
			return mapQuotaPoolRecordError(err)
		}
		if pool.IsSystemPool() {
			return ErrQuotaPoolSystemReadonly
		}
		if !pool.Enabled {
			return ErrQuotaPoolDisabled
		}
		if !IsQuotaPoolMemberRole(user.Role) || user.QuotaPoolId != poolId {
			return ErrQuotaPoolMemberMismatch
		}
		var admin QuotaPoolAdmin
		err := tx.Where("user_id = ?", userId).First(&admin).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return tx.Create(&QuotaPoolAdmin{PoolId: poolId, UserId: userId, Level: QuotaPoolAdminLevel}).Error
		}
		if err != nil {
			return err
		}
		if admin.PoolId != poolId {
			return ErrQuotaPoolAdminConflict
		}
		return tx.Model(&admin).Update("level", QuotaPoolAdminLevel).Error
	})
}

func RevokeQuotaPoolAdmin(poolId, userId int) error {
	return DB.Where("pool_id = ? AND user_id = ?", poolId, userId).Delete(&QuotaPoolAdmin{}).Error
}

func GetQuotaPoolAdminSummary(userId int) (*QuotaPoolAdminSummary, error) {
	var admin QuotaPoolAdmin
	err := DB.Where("user_id = ?", userId).First(&admin).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &QuotaPoolAdminSummary{PoolId: admin.PoolId}, nil
}

func IsQuotaPoolMemberRole(role int) bool {
	return role == common.RoleCommonUser || role == common.RoleQuotaPoolSuperAdmin || role == common.RoleAdminUser
}

func mapQuotaPoolRecordError(err error) error {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ErrQuotaPoolNotFound
	}
	return err
}
