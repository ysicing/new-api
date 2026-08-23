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

type QuotaPoolMemberRemoval struct {
	SourcePoolId      int
	UserId            int
	OperatorId        int
	AllowAdminRemoval bool
}

func RemoveQuotaPoolMember(removal QuotaPoolMemberRemoval) (*QuotaPoolMoveResult, error) {
	target, err := GetNewUserQuotaPool()
	if err != nil {
		return nil, err
	}
	return moveUserBetweenQuotaPools(removal.UserId, target.Id, removal.OperatorId, quotaPoolMoveOptions{
		allowSystemTarget:    true,
		requiredSourcePoolId: removal.SourcePoolId,
		requireNormalSource:  true,
		guardAdminRemoval:    true,
		allowAdminRemoval:    removal.AllowAdminRemoval,
	})
}

type quotaPoolMoveOptions struct {
	allowSystemTarget        bool
	requireEligibleCandidate bool
	requireNewUserSource     bool
	requiredSourcePoolId     int
	requireNormalSource      bool
	guardAdminRemoval        bool
	allowAdminRemoval        bool
}

type quotaPoolMoveContext struct {
	userId       int
	targetPoolId int
	operatorId   int
	options      quotaPoolMoveOptions
	result       *QuotaPoolMoveResult
}

func moveUserBetweenQuotaPools(userId, targetPoolId, operatorId int, options quotaPoolMoveOptions) (*QuotaPoolMoveResult, error) {
	context := &quotaPoolMoveContext{
		userId: userId, targetPoolId: targetPoolId, operatorId: operatorId,
		options: options, result: &QuotaPoolMoveResult{NewPoolId: targetPoolId},
	}
	err := DB.Transaction(func(tx *gorm.DB) error {
		return moveUserBetweenQuotaPoolsWithTx(tx, context)
	})
	if err != nil {
		return nil, err
	}
	_ = invalidateUserCache(userId)
	return context.result, nil
}

func moveUserBetweenQuotaPoolsWithTx(tx *gorm.DB, context *quotaPoolMoveContext) error {
	user, pools, err := prepareQuotaPoolMove(tx, context)
	if err != nil {
		return err
	}
	if err := reclaimMovingUserQuota(tx, pools[user.QuotaPoolId], user, context.operatorId, context.result); err != nil {
		return err
	}
	if err := tx.Model(&User{}).Where("id = ?", context.userId).Updates(map[string]any{
		"quota": 0, "quota_pool_id": context.targetPoolId,
	}).Error; err != nil {
		return err
	}
	deletion := tx.Where("user_id = ? AND pool_id = ?", context.userId, user.QuotaPoolId).Delete(&QuotaPoolAdmin{})
	if deletion.Error != nil {
		return deletion.Error
	}
	context.result.AdminRevoked = deletion.RowsAffected > 0
	return nil
}

func prepareQuotaPoolMove(tx *gorm.DB, context *quotaPoolMoveContext) (*User, map[int]*QuotaPool, error) {
	var user User
	if err := lockForUpdate(tx).Where("id = ?", context.userId).First(&user).Error; err != nil {
		return nil, nil, err
	}
	if !IsQuotaPoolMemberRole(user.Role) ||
		(context.options.requireEligibleCandidate && user.Status != common.UserStatusEnabled) {
		return nil, nil, ErrQuotaPoolCandidateInvalid
	}
	context.result.OldPoolId, context.result.UserQuota = user.QuotaPoolId, user.Quota
	if context.options.requiredSourcePoolId > 0 && user.QuotaPoolId != context.options.requiredSourcePoolId {
		return nil, nil, ErrQuotaPoolMemberMismatch
	}
	if user.QuotaPoolId == context.targetPoolId {
		return nil, nil, ErrQuotaPoolSamePool
	}
	pools, err := lockMovePools(tx, user.QuotaPoolId, context.targetPoolId)
	if err != nil {
		return nil, nil, err
	}
	if err := validateMoveTarget(pools[context.targetPoolId], context.targetPoolId, context.options.allowSystemTarget); err != nil {
		return nil, nil, err
	}
	if err := context.validateMoveSource(tx, pools[user.QuotaPoolId], &user); err != nil {
		return nil, nil, err
	}
	context.result.TargetNewUserPool = pools[context.targetPoolId] != nil && pools[context.targetPoolId].IsNewUserPool()
	return &user, pools, nil
}

func (context *quotaPoolMoveContext) validateMoveSource(tx *gorm.DB, source *QuotaPool, user *User) error {
	if context.options.requireNewUserSource && (source == nil || !source.IsNewUserPool()) {
		return ErrQuotaPoolCandidateInvalid
	}
	if context.options.requireNormalSource && (source == nil || source.PoolType != QuotaPoolTypeNormal) {
		return ErrQuotaPoolSystemReadonly
	}
	if !context.options.guardAdminRemoval {
		return nil
	}
	if !context.options.allowAdminRemoval && user.Role != common.RoleCommonUser {
		return ErrQuotaPoolPermissionDenied
	}
	var adminCount int64
	if err := tx.Model(&QuotaPoolAdmin{}).
		Where("pool_id = ? AND user_id = ?", user.QuotaPoolId, user.Id).
		Count(&adminCount).Error; err != nil {
		return err
	}
	if adminCount > 0 && !context.options.allowAdminRemoval {
		return ErrQuotaPoolPermissionDenied
	}
	return nil
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
