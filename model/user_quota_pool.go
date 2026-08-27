package model

import (
	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)

func (user *User) applyInitialQuotaPool(tx *gorm.DB) error {
	user.Quota = common.QuotaForNewUser
	user.QuotaPoolId = QuotaPoolDefaultUserPoolId
	if !common.QuotaPoolEnabled {
		return nil
	}
	var pool QuotaPool
	if err := tx.Where("pool_type = ?", QuotaPoolTypeNewUser).First(&pool).Error; err != nil {
		return err
	}
	user.QuotaPoolId = pool.Id
	return nil
}

func fillUserQuotaPoolNames(tx *gorm.DB, users []*User) error {
	if !tx.Migrator().HasTable(&QuotaPool{}) {
		return nil
	}
	ids := make([]int, 0, len(users))
	seen := make(map[int]struct{})
	needsDefaultPool := false
	for _, user := range users {
		if user == nil {
			continue
		}
		if user.QuotaPoolId == QuotaPoolDefaultUserPoolId {
			needsDefaultPool = true
			continue
		}
		if user.QuotaPoolId < 0 {
			continue
		}
		if _, ok := seen[user.QuotaPoolId]; ok {
			continue
		}
		seen[user.QuotaPoolId] = struct{}{}
		ids = append(ids, user.QuotaPoolId)
	}
	if len(ids) == 0 && !needsDefaultPool {
		return nil
	}
	var pools []QuotaPool
	query := tx.Select("id", "name", "pool_type", "is_default")
	if len(ids) > 0 && needsDefaultPool {
		query = query.Where(
			"id IN ? OR pool_type = ? OR is_default = ?",
			ids, QuotaPoolTypeDefault, true,
		)
	} else if len(ids) > 0 {
		query = query.Where("id IN ?", ids)
	} else {
		query = query.Where("pool_type = ? OR is_default = ?", QuotaPoolTypeDefault, true)
	}
	if err := query.Order("id ASC").Find(&pools).Error; err != nil {
		return err
	}
	names := make(map[int]string, len(pools)+1)
	for _, pool := range pools {
		names[pool.Id] = pool.Name
		if names[QuotaPoolDefaultUserPoolId] == "" &&
			(pool.PoolType == QuotaPoolTypeDefault || pool.IsDefault) {
			names[QuotaPoolDefaultUserPoolId] = pool.Name
		}
	}
	for _, user := range users {
		if user != nil {
			user.QuotaPoolName = names[user.QuotaPoolId]
		}
	}
	return nil
}
