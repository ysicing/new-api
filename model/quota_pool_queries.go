package model

import (
	"errors"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"gorm.io/gorm"
)

func ListQuotaPoolItems() ([]QuotaPoolListItem, error) {
	items, _, err := ListQuotaPoolItemsPage("", nil)
	return items, err
}

func ListQuotaPoolItemsPage(keyword string, page *common.PageInfo) ([]QuotaPoolListItem, int64, error) {
	query := DB.Model(&QuotaPool{})
	keyword = strings.TrimSpace(keyword)
	if keyword != "" {
		escapedKeyword := strings.NewReplacer("!", "!!", "%", "!%", "_", "!_").Replace(strings.ToLower(keyword))
		like := "%" + escapedKeyword + "%"
		if poolId, err := strconv.Atoi(keyword); err == nil {
			query = query.Where("(id = ? OR LOWER(name) LIKE ? ESCAPE '!')", poolId, like)
		} else {
			query = query.Where("LOWER(name) LIKE ? ESCAPE '!'", like)
		}
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if page != nil {
		if page.PageSize < 1 {
			page.PageSize = common.ItemsPerPage
		}
		lastPage := int64(1)
		if total > 0 {
			lastPage = (total-1)/int64(page.PageSize) + 1
		}
		if page.Page < 1 {
			page.Page = 1
		} else if int64(page.Page) > lastPage {
			page.Page = int(lastPage)
		}
		query = query.Offset(page.GetStartIdx()).Limit(page.GetPageSize())
	}
	var pools []QuotaPool
	if err := query.Order("is_default DESC, id ASC").Find(&pools).Error; err != nil {
		return nil, 0, err
	}
	items := make([]QuotaPoolListItem, 0, len(pools))
	for _, pool := range pools {
		item, err := buildQuotaPoolListItem(pool)
		if err != nil {
			return nil, 0, err
		}
		items = append(items, item)
	}
	return items, total, nil
}

func GetQuotaPoolListItemById(poolId int) (*QuotaPoolListItem, error) {
	pool, err := GetQuotaPoolById(poolId)
	if err != nil {
		return nil, err
	}
	item, err := buildQuotaPoolListItem(*pool)
	return &item, err
}

func buildQuotaPoolListItem(pool QuotaPool) (QuotaPoolListItem, error) {
	config := operation_setting.GetAutoRechargeSetting()
	item := QuotaPoolListItem{
		QuotaPool: pool,
		SystemAutoRecharge: QuotaPoolSystemAutoRecharge{
			Enabled: config.Enabled, Interval: config.Interval,
			Threshold:   int(float64(config.Threshold) * common.QuotaPerUnit),
			Amount:      int(float64(config.Amount) * common.QuotaPerUnit),
			WeeklyLimit: config.WeeklyLimit, MonthlyLimit: config.MonthlyLimit,
		},
	}
	queryPoolId := pool.Id
	if pool.PoolType == QuotaPoolTypeDefault {
		queryPoolId = QuotaPoolDefaultUserPoolId
	}
	if err := DB.Model(&User{}).Where("quota_pool_id = ?", queryPoolId).Count(&item.MemberCount).Error; err != nil {
		return item, err
	}
	if err := DB.Model(&QuotaPoolAdmin{}).Where("pool_id = ?", pool.Id).Count(&item.AdminCount).Error; err != nil {
		return item, err
	}
	return item, nil
}

func ListQuotaPoolMembers(poolId int, keyword string, page *common.PageInfo) ([]QuotaPoolMember, int64, error) {
	queryPoolId := poolId
	if pool, err := GetQuotaPoolById(poolId); err == nil && pool.PoolType == QuotaPoolTypeDefault {
		queryPoolId = QuotaPoolDefaultUserPoolId
	}
	query := DB.Model(&User{}).Where("quota_pool_id = ?", queryPoolId)
	keyword = strings.TrimSpace(keyword)
	if keyword != "" {
		escapedKeyword := strings.NewReplacer("!", "!!", "%", "!%", "_", "!_").Replace(strings.ToLower(keyword))
		like := "%" + escapedKeyword + "%"
		if userId, err := strconv.Atoi(keyword); err == nil {
			query = query.Where("(id = ? OR LOWER(username) LIKE ? ESCAPE '!' OR LOWER(display_name) LIKE ? ESCAPE '!' OR LOWER(email) LIKE ? ESCAPE '!' OR LOWER(department) LIKE ? ESCAPE '!')", userId, like, like, like, like)
		} else {
			query = query.Where("(LOWER(username) LIKE ? ESCAPE '!' OR LOWER(display_name) LIKE ? ESCAPE '!' OR LOWER(email) LIKE ? ESCAPE '!' OR LOWER(department) LIKE ? ESCAPE '!')", like, like, like, like)
		}
	}
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

func ListQuotaPoolAdminContacts(poolId int) ([]QuotaPoolAdminContact, error) {
	var admins []QuotaPoolAdmin
	if err := DB.Where("pool_id = ?", poolId).Find(&admins).Error; err != nil {
		return nil, err
	}
	ids := make([]int, 0, len(admins))
	for _, admin := range admins {
		ids = append(ids, admin.UserId)
	}
	if len(ids) == 0 {
		return []QuotaPoolAdminContact{}, nil
	}
	var users []User
	if err := DB.Select("id", "username", "display_name", "email").Where("id IN ?", ids).Find(&users).Error; err != nil {
		return nil, err
	}
	contacts := make([]QuotaPoolAdminContact, 0, len(users))
	for _, user := range users {
		contacts = append(contacts, QuotaPoolAdminContact{Id: user.Id, Username: user.Username, DisplayName: user.DisplayName, Email: user.Email})
	}
	return contacts, nil
}

func ListQuotaPoolCandidates(keyword string, page *common.PageInfo) ([]QuotaPoolMember, int64, error) {
	var newUserPool QuotaPool
	if err := DB.Where("pool_type = ?", QuotaPoolTypeNewUser).First(&newUserPool).Error; errors.Is(err, gorm.ErrRecordNotFound) {
		return []QuotaPoolMember{}, 0, nil
	} else if err != nil {
		return nil, 0, err
	}
	query := DB.Model(&User{}).Where(
		"quota_pool_id = ? AND role IN ? AND status = ?",
		newUserPool.Id,
		[]int{common.RoleCommonUser, common.RoleQuotaPoolSuperAdmin, common.RoleAdminUser},
		common.UserStatusEnabled,
	)
	keyword = strings.TrimSpace(keyword)
	if keyword != "" {
		escapedKeyword := strings.NewReplacer("!", "!!", "%", "!%", "_", "!_").Replace(strings.ToLower(keyword))
		like := "%" + escapedKeyword + "%"
		if userId, err := strconv.Atoi(keyword); err == nil {
			query = query.Where("(id = ? OR LOWER(username) LIKE ? ESCAPE '!' OR LOWER(display_name) LIKE ? ESCAPE '!' OR LOWER(email) LIKE ? ESCAPE '!' OR LOWER(department) LIKE ? ESCAPE '!')", userId, like, like, like, like)
		} else {
			query = query.Where("(LOWER(username) LIKE ? ESCAPE '!' OR LOWER(display_name) LIKE ? ESCAPE '!' OR LOWER(email) LIKE ? ESCAPE '!' OR LOWER(department) LIKE ? ESCAPE '!')", like, like, like, like)
		}
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
