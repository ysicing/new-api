package model

import "gorm.io/gorm"

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
