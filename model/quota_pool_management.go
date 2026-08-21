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
	if name == "" || baseQuota <= 0 {
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
	if amount <= 0 {
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
		if pool.BaseQuota <= 0 || amount*2 > pool.BaseQuota {
			return ErrQuotaPoolRefillLimited
		}
		var count int64
		if err := tx.Model(&QuotaPoolTransaction{}).
			Where("pool_id = ? AND type = ? AND created_at >= ? AND created_at < ?", poolId, QuotaPoolTransactionManualRefill, monthStart, nextMonth).
			Count(&count).Error; err != nil {
			return err
		}
		if count >= 2 {
			return ErrQuotaPoolRefillLimited
		}
		change.QuotaBefore = pool.Quota
		change.QuotaAfter = pool.Quota + amount
		if err := tx.Model(&QuotaPool{}).Where("id = ?", poolId).Updates(map[string]any{
			"quota": gorm.Expr("quota + ?", amount), "base_quota": gorm.Expr("base_quota + ?", amount),
		}).Error; err != nil {
			return err
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

func GrantQuotaPoolAdmin(poolId, userId, level int) error {
	if level != QuotaPoolAdminLevelV1 && level != QuotaPoolAdminLevelV2 {
		return ErrQuotaPoolPermissionDenied
	}
	return DB.Transaction(func(tx *gorm.DB) error {
		var pool QuotaPool
		if err := tx.Where("id = ?", poolId).First(&pool).Error; err != nil {
			return mapQuotaPoolRecordError(err)
		}
		if pool.IsSystemPool() {
			return ErrQuotaPoolSystemReadonly
		}
		if !pool.Enabled {
			return ErrQuotaPoolDisabled
		}
		var user User
		if err := tx.Where("id = ?", userId).First(&user).Error; err != nil {
			return err
		}
		if !IsQuotaPoolMemberRole(user.Role) || user.QuotaPoolId != poolId {
			return ErrQuotaPoolMemberMismatch
		}
		var admin QuotaPoolAdmin
		err := tx.Where("user_id = ?", userId).First(&admin).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return tx.Create(&QuotaPoolAdmin{PoolId: poolId, UserId: userId, Level: level}).Error
		}
		if err != nil {
			return err
		}
		if admin.PoolId != poolId {
			return ErrQuotaPoolAdminConflict
		}
		return tx.Model(&admin).Update("level", level).Error
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
	return &QuotaPoolAdminSummary{PoolId: admin.PoolId, Level: admin.Level}, nil
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
