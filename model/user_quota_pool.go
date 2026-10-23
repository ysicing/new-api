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
	for _, user := range users {
		if user == nil || user.QuotaPoolId <= 0 {
			continue
		}
		if _, ok := seen[user.QuotaPoolId]; ok {
			continue
		}
		seen[user.QuotaPoolId] = struct{}{}
		ids = append(ids, user.QuotaPoolId)
	}
	if len(ids) == 0 {
		return nil
	}
	var pools []QuotaPool
	if err := tx.Select("id", "name").Where("id IN ?", ids).Find(&pools).Error; err != nil {
		return err
	}
	names := make(map[int]string, len(pools))
	for _, pool := range pools {
		names[pool.Id] = pool.Name
	}
	for _, user := range users {
		if user != nil {
			user.QuotaPoolName = names[user.QuotaPoolId]
		}
	}
	return nil
}
