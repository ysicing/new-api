package model

import (
	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)

func ReclaimQuotaToPool(poolId, userId, amount, operatorId int) (*QuotaPoolBalanceChange, error) {
	if amount <= 0 || amount > common.MaxWalletQuota {
		return nil, ErrQuotaPoolInvalidAmount
	}
	change := &QuotaPoolBalanceChange{PoolId: poolId, Amount: amount}
	err := DB.Transaction(func(tx *gorm.DB) error {
		pool, user, err := lockQuotaPoolMember(tx, poolId, userId)
		if err != nil {
			return err
		}
		if pool.Quota < 0 || pool.Quota > common.MaxWalletQuota {
			return ErrQuotaPoolInvalidAmount
		}
		if pool.Quota > common.MaxWalletQuota-amount {
			return ErrWalletQuotaLimitExceeded
		}
		change.QuotaBefore = pool.Quota
		change.QuotaAfter = pool.Quota + amount
		debit := tx.Model(&User{}).Where("id = ? AND quota >= ?", user.Id, amount).
			Update("quota", gorm.Expr("quota - ?", amount))
		if debit.Error != nil {
			return debit.Error
		}
		if debit.RowsAffected != 1 {
			return ErrQuotaPoolInsufficientQuota
		}
		credit := tx.Model(&QuotaPool{}).Where("id = ? AND quota >= 0 AND quota <= ?", pool.Id, common.MaxWalletQuota-amount).
			Update("quota", gorm.Expr("quota + ?", amount))
		if credit.Error != nil {
			return credit.Error
		}
		if credit.RowsAffected != 1 {
			return ErrWalletQuotaLimitExceeded
		}
		return tx.Create(&QuotaPoolTransaction{
			PoolId: pool.Id, Type: QuotaPoolTransactionReclaimUser,
			Amount: amount, QuotaBefore: change.QuotaBefore, QuotaAfter: change.QuotaAfter,
			UserId: user.Id, OperatorId: operatorId,
		}).Error
	})
	if err != nil {
		return nil, err
	}
	_ = cacheDecrUserQuota(userId, int64(amount))
	return change, nil
}
