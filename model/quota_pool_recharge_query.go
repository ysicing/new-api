package model

import (
	"errors"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)

var ErrQuotaPoolRechargeUserAmbiguous = errors.New("multiple users match the recharge identifier")

type QuotaPoolRechargeRecord struct {
	Id           int    `json:"id"`
	PoolId       int    `json:"pool_id"`
	PoolName     string `json:"pool_name"`
	UserId       int    `json:"user_id"`
	UserName     string `json:"user_name"`
	UserEmail    string `json:"user_email"`
	OperatorId   int    `json:"operator_id"`
	OperatorName string `json:"operator_name"`
	Type         string `json:"type"`
	Amount       int    `json:"amount"`
	CreatedAt    int64  `json:"created_at"`
}

// FindUserByRechargeIdentifier 按 ID、用户名或邮箱做精确匹配。
// 数据库排序规则可能忽略大小写或重音，因此查询后再次用 Go 字符串相等
// 过滤，保证 SQLite、MySQL 与 PostgreSQL 返回一致结果。
func FindUserByRechargeIdentifier(identifier string) (*User, error) {
	identifier = strings.TrimSpace(identifier)
	if identifier == "" {
		return nil, gorm.ErrRecordNotFound
	}
	query := DB.Model(&User{}).Where("username = ? OR email = ?", identifier, identifier)
	userId := 0
	if parsedUserId, err := strconv.Atoi(identifier); err == nil && parsedUserId > 0 {
		userId = parsedUserId
		query = query.Or("id = ?", userId)
	}
	var candidates []User
	if err := query.Omit("password", "access_token").Find(&candidates).Error; err != nil {
		return nil, err
	}
	users := make([]User, 0, len(candidates))
	for _, candidate := range candidates {
		if candidate.Id == userId || candidate.Username == identifier || candidate.Email == identifier {
			users = append(users, candidate)
		}
	}
	if len(users) == 0 {
		return nil, gorm.ErrRecordNotFound
	}
	if len(users) > 1 {
		return nil, ErrQuotaPoolRechargeUserAmbiguous
	}
	return &users[0], nil
}

// ListQuotaPoolRechargeRecords 只返回额度池向成员拨付额度的交易记录。
// 交易表中的拨付金额为负数，这里统一转换为面向管理页面展示的正数。
func ListQuotaPoolRechargeRecords(startTimestamp, endTimestamp int64, page *common.PageInfo) ([]QuotaPoolRechargeRecord, int64, error) {
	types := []string{QuotaPoolTransactionAllocateAuto, QuotaPoolTransactionAllocateManual}
	query := DB.Model(&QuotaPoolTransaction{}).
		Where("created_at >= ? AND created_at <= ?", startTimestamp, endTimestamp).
		Where("type IN ?", types)
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if page == nil {
		page = &common.PageInfo{Page: 1, PageSize: common.ItemsPerPage}
	}
	if page.PageSize < 1 {
		page.PageSize = common.ItemsPerPage
	} else if page.PageSize > 100 {
		page.PageSize = 100
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
	var transactions []QuotaPoolTransaction
	if err := query.Order("id DESC").Offset(page.GetStartIdx()).Limit(page.GetPageSize()).Find(&transactions).Error; err != nil {
		return nil, 0, err
	}

	poolIds := make([]int, 0, len(transactions))
	userIds := make([]int, 0, len(transactions)*2)
	for _, transaction := range transactions {
		poolIds = append(poolIds, transaction.PoolId)
		if transaction.UserId > 0 {
			userIds = append(userIds, transaction.UserId)
		}
		if transaction.OperatorId > 0 {
			userIds = append(userIds, transaction.OperatorId)
		}
	}
	poolNames := make(map[int]string, len(poolIds))
	if len(poolIds) > 0 {
		var pools []QuotaPool
		if err := DB.Unscoped().Select("id", "name").Where("id IN ?", poolIds).Find(&pools).Error; err != nil {
			return nil, 0, err
		}
		for _, pool := range pools {
			poolNames[pool.Id] = pool.Name
		}
	}
	type rechargeUser struct {
		Id       int
		Username string
		Email    string
	}
	users := make(map[int]rechargeUser, len(userIds))
	if len(userIds) > 0 {
		var records []rechargeUser
		if err := DB.Unscoped().Model(&User{}).Select("id", "username", "email").Where("id IN ?", userIds).Find(&records).Error; err != nil {
			return nil, 0, err
		}
		for _, user := range records {
			users[user.Id] = user
		}
	}

	items := make([]QuotaPoolRechargeRecord, 0, len(transactions))
	for _, transaction := range transactions {
		amount := transaction.Amount
		if amount < 0 {
			amount = -amount
		}
		user := users[transaction.UserId]
		operator := users[transaction.OperatorId]
		items = append(items, QuotaPoolRechargeRecord{
			Id: transaction.Id, PoolId: transaction.PoolId, PoolName: poolNames[transaction.PoolId],
			UserId: transaction.UserId, UserName: user.Username, UserEmail: user.Email,
			OperatorId: transaction.OperatorId, OperatorName: operator.Username,
			Type: transaction.Type, Amount: amount, CreatedAt: transaction.CreatedAt,
		})
	}
	return items, total, nil
}
