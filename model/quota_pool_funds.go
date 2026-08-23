package model

import (
	"errors"
	"sort"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)

func AllocateQuotaFromPool(poolId, userId, amount int, transactionType string, operatorId int) (*QuotaPoolBalanceChange, error) {
	if amount <= 0 {
		return nil, ErrQuotaPoolInvalidAmount
	}
	if poolId == QuotaPoolDefaultUserPoolId {
		if err := IncreaseUserQuota(userId, amount, true); err != nil {
			return nil, err
		}
		return &QuotaPoolBalanceChange{}, nil
	}
	change := &QuotaPoolBalanceChange{PoolId: poolId, Amount: -amount}
	err := DB.Transaction(func(tx *gorm.DB) error {
		pool, user, err := lockQuotaPoolMember(tx, poolId, userId)
		if err != nil {
			return err
		}
		change.QuotaBefore = pool.Quota
		change.QuotaAfter = pool.Quota - amount
		debit := tx.Model(&QuotaPool{}).
			Where("id = ? AND quota >= ?", poolId, amount).
			Update("quota", gorm.Expr("quota - ?", amount))
		if debit.Error != nil {
			return debit.Error
		}
		if debit.RowsAffected != 1 {
			return ErrQuotaPoolInsufficientQuota
		}
		if err := tx.Model(&User{}).Where("id = ?", user.Id).
			Update("quota", gorm.Expr("quota + ?", amount)).Error; err != nil {
			return err
		}
		return tx.Create(&QuotaPoolTransaction{
			PoolId: poolId, Type: transactionType, Amount: -amount,
			QuotaBefore: change.QuotaBefore, QuotaAfter: change.QuotaAfter,
			UserId: userId, OperatorId: operatorId,
		}).Error
	})
	if err != nil {
		return nil, err
	}
	syncCreditUserQuotaCache(userId, amount, "quota pool allocation")
	return change, nil
}

func lockQuotaPoolMember(tx *gorm.DB, poolId, userId int) (*QuotaPool, *User, error) {
	var pool QuotaPool
	if err := lockForUpdate(tx).Where("id = ?", poolId).First(&pool).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil, ErrQuotaPoolNotFound
		}
		return nil, nil, err
	}
	if pool.IsSystemPool() {
		return nil, nil, ErrQuotaPoolSystemReadonly
	}
	if !pool.Enabled {
		return nil, nil, ErrQuotaPoolDisabled
	}
	var user User
	if err := lockForUpdate(tx).Where("id = ?", userId).First(&user).Error; err != nil {
		return nil, nil, err
	}
	if user.QuotaPoolId != poolId {
		return nil, nil, ErrQuotaPoolMemberMismatch
	}
	return &pool, &user, nil
}

func MoveUserBetweenQuotaPools(userId, targetPoolId int, allowSystemTarget bool, operatorId int) (*QuotaPoolMoveResult, error) {
	return moveUserBetweenQuotaPools(userId, targetPoolId, operatorId, quotaPoolMoveOptions{allowSystemTarget: allowSystemTarget})
}

func AddUserToQuotaPool(userId, targetPoolId, operatorId int) (*QuotaPoolMoveResult, error) {
	return moveUserBetweenQuotaPools(userId, targetPoolId, operatorId, quotaPoolMoveOptions{
		requireEligibleCandidate: true,
		requireNewUserSource:     true,
	})
}

type quotaPoolMoveOptions struct {
	allowSystemTarget        bool
	requireEligibleCandidate bool
	requireNewUserSource     bool
}

func moveUserBetweenQuotaPools(userId, targetPoolId, operatorId int, options quotaPoolMoveOptions) (*QuotaPoolMoveResult, error) {
	result := &QuotaPoolMoveResult{NewPoolId: targetPoolId}
	err := DB.Transaction(func(tx *gorm.DB) error {
		var user User
		if err := lockForUpdate(tx).Where("id = ?", userId).First(&user).Error; err != nil {
			return err
		}
		if !IsQuotaPoolMemberRole(user.Role) {
			return ErrQuotaPoolCandidateInvalid
		}
		if options.requireEligibleCandidate && user.Status != common.UserStatusEnabled {
			return ErrQuotaPoolCandidateInvalid
		}
		result.OldPoolId = user.QuotaPoolId
		result.UserQuota = user.Quota
		if user.QuotaPoolId == targetPoolId {
			return ErrQuotaPoolSamePool
		}
		pools, err := lockMovePools(tx, user.QuotaPoolId, targetPoolId)
		if err != nil {
			return err
		}
		if err := validateMoveTarget(pools[targetPoolId], targetPoolId, options.allowSystemTarget); err != nil {
			return err
		}
		if options.requireNewUserSource {
			source := pools[user.QuotaPoolId]
			if source == nil || !source.IsNewUserPool() {
				return ErrQuotaPoolCandidateInvalid
			}
		}
		if target := pools[targetPoolId]; target != nil {
			result.TargetNewUserPool = target.IsNewUserPool()
		}
		if err := reclaimMovingUserQuota(tx, pools[user.QuotaPoolId], &user, operatorId, result); err != nil {
			return err
		}
		if err := tx.Model(&User{}).Where("id = ?", userId).Updates(map[string]any{
			"quota": 0, "quota_pool_id": targetPoolId,
		}).Error; err != nil {
			return err
		}
		return tx.Where("user_id = ? AND pool_id = ?", userId, user.QuotaPoolId).Delete(&QuotaPoolAdmin{}).Error
	})
	if err != nil {
		return nil, err
	}
	_ = invalidateUserCache(userId)
	return result, nil
}

func lockMovePools(tx *gorm.DB, oldPoolId, targetPoolId int) (map[int]*QuotaPool, error) {
	ids := make([]int, 0, 2)
	for _, id := range []int{oldPoolId, targetPoolId} {
		if id > 0 && (len(ids) == 0 || ids[0] != id) {
			ids = append(ids, id)
		}
	}
	sort.Ints(ids)
	result := make(map[int]*QuotaPool, len(ids))
	for _, id := range ids {
		var pool QuotaPool
		if err := lockForUpdate(tx).Where("id = ?", id).First(&pool).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, ErrQuotaPoolNotFound
			}
			return nil, err
		}
		poolCopy := pool
		result[id] = &poolCopy
	}
	return result, nil
}

func validateMoveTarget(target *QuotaPool, targetPoolId int, allowSystemTarget bool) error {
	if targetPoolId == QuotaPoolDefaultUserPoolId {
		return nil
	}
	if target == nil {
		return ErrQuotaPoolNotFound
	}
	if target.IsSystemPool() && !allowSystemTarget {
		return ErrQuotaPoolSystemReadonly
	}
	if !target.Enabled {
		return ErrQuotaPoolDisabled
	}
	return nil
}

func reclaimMovingUserQuota(tx *gorm.DB, oldPool *QuotaPool, user *User, operatorId int, result *QuotaPoolMoveResult) error {
	if oldPool == nil || oldPool.IsNewUserPool() || user.Quota <= 0 {
		return nil
	}
	result.Reclaimed = true
	result.Change = QuotaPoolBalanceChange{
		PoolId: oldPool.Id, Amount: user.Quota,
		QuotaBefore: oldPool.Quota, QuotaAfter: oldPool.Quota + user.Quota,
	}
	if err := tx.Model(&QuotaPool{}).Where("id = ?", oldPool.Id).
		Update("quota", gorm.Expr("quota + ?", user.Quota)).Error; err != nil {
		return err
	}
	return tx.Create(&QuotaPoolTransaction{
		PoolId: oldPool.Id, Type: QuotaPoolTransactionReclaimUser,
		Amount: user.Quota, QuotaBefore: oldPool.Quota, QuotaAfter: oldPool.Quota + user.Quota,
		UserId: user.Id, OperatorId: operatorId,
	}).Error
}
