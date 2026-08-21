package model

import (
	"errors"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)

func ListQuotaPoolItems() ([]QuotaPoolListItem, error) {
	var pools []QuotaPool
	if err := DB.Order("is_default DESC, id ASC").Find(&pools).Error; err != nil {
		return nil, err
	}
	items := make([]QuotaPoolListItem, 0, len(pools))
	for _, pool := range pools {
		item := QuotaPoolListItem{QuotaPool: pool}
		queryPoolId := pool.Id
		if pool.PoolType == QuotaPoolTypeDefault {
			queryPoolId = QuotaPoolDefaultUserPoolId
		}
		if err := DB.Model(&User{}).Where("quota_pool_id = ?", queryPoolId).Count(&item.MemberCount).Error; err != nil {
			return nil, err
		}
		if err := DB.Model(&QuotaPoolAdmin{}).Where("pool_id = ?", pool.Id).Count(&item.AdminCount).Error; err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, nil
}

func ListQuotaPoolMembers(poolId int, page *common.PageInfo) ([]QuotaPoolMember, int64, error) {
	queryPoolId := poolId
	if pool, err := GetQuotaPoolById(poolId); err == nil && pool.PoolType == QuotaPoolTypeDefault {
		queryPoolId = QuotaPoolDefaultUserPoolId
	}
	query := DB.Model(&User{}).Where("quota_pool_id = ?", queryPoolId)
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var users []User
	if err := query.Order("id ASC").Offset(page.GetStartIdx()).Limit(page.GetPageSize()).Find(&users).Error; err != nil {
		return nil, 0, err
	}
	levels, err := quotaPoolAdminLevels(poolId)
	if err != nil {
		return nil, 0, err
	}
	items := make([]QuotaPoolMember, 0, len(users))
	for _, user := range users {
		items = append(items, QuotaPoolMember{
			Id: user.Id, Username: user.Username, DisplayName: user.DisplayName,
			Email: user.Email, Department: user.Department, Role: user.Role, Status: user.Status,
			Quota: user.Quota, UsedQuota: user.UsedQuota, QuotaPoolId: user.QuotaPoolId,
			QuotaPoolAdminLevel: levels[user.Id],
		})
	}
	return items, total, nil
}

func quotaPoolAdminLevels(poolId int) (map[int]int, error) {
	var admins []QuotaPoolAdmin
	if err := DB.Where("pool_id = ?", poolId).Find(&admins).Error; err != nil {
		return nil, err
	}
	levels := make(map[int]int, len(admins))
	for _, admin := range admins {
		levels[admin.UserId] = admin.Level
	}
	return levels, nil
}

func ListQuotaPoolCandidates(keyword string, page *common.PageInfo) ([]QuotaPoolMember, int64, error) {
	poolIds := []int{QuotaPoolDefaultUserPoolId}
	var newUserPool QuotaPool
	if err := DB.Where("pool_type = ?", QuotaPoolTypeNewUser).First(&newUserPool).Error; err == nil {
		poolIds = append(poolIds, newUserPool.Id)
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, 0, err
	}
	query := DB.Model(&User{}).Where("quota_pool_id IN ?", poolIds)
	if keyword != "" {
		like := "%" + keyword + "%"
		query = query.Where("username LIKE ? OR display_name LIKE ? OR email LIKE ? OR department LIKE ?", like, like, like, like)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var users []User
	if err := query.Order("id ASC").Offset(page.GetStartIdx()).Limit(page.GetPageSize()).Find(&users).Error; err != nil {
		return nil, 0, err
	}
	items := make([]QuotaPoolMember, 0, len(users))
	for _, user := range users {
		items = append(items, QuotaPoolMember{Id: user.Id, Username: user.Username, DisplayName: user.DisplayName, Email: user.Email, Department: user.Department, Role: user.Role, Status: user.Status, Quota: user.Quota, UsedQuota: user.UsedQuota, QuotaPoolId: user.QuotaPoolId})
	}
	return items, total, nil
}

func ListQuotaPoolTransactions(poolId int, page *common.PageInfo) ([]QuotaPoolTransactionItem, int64, error) {
	query := DB.Model(&QuotaPoolTransaction{}).Where("pool_id = ?", poolId)
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var records []QuotaPoolTransaction
	if err := query.Order("id DESC").Offset(page.GetStartIdx()).Limit(page.GetPageSize()).Find(&records).Error; err != nil {
		return nil, 0, err
	}
	names, err := quotaPoolTransactionUserNames(records)
	if err != nil {
		return nil, 0, err
	}
	items := make([]QuotaPoolTransactionItem, 0, len(records))
	for _, record := range records {
		items = append(items, QuotaPoolTransactionItem{QuotaPoolTransaction: record, UserName: names[record.UserId], OperatorName: names[record.OperatorId]})
	}
	return items, total, nil
}

func quotaPoolTransactionUserNames(records []QuotaPoolTransaction) (map[int]string, error) {
	ids := make([]int, 0, len(records)*2)
	for _, record := range records {
		if record.UserId > 0 {
			ids = append(ids, record.UserId)
		}
		if record.OperatorId > 0 {
			ids = append(ids, record.OperatorId)
		}
	}
	var users []User
	if len(ids) > 0 {
		if err := DB.Select("id", "username").Where("id IN ?", ids).Find(&users).Error; err != nil {
			return nil, err
		}
	}
	names := make(map[int]string, len(users))
	for _, user := range users {
		names[user.Id] = user.Username
	}
	return names, nil
}
