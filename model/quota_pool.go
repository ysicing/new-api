package model

import (
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	QuotaPoolAdminLevelV1 = 1

	QuotaPoolDefaultUserPoolId = 0
	QuotaPoolUnlimitedQuota    = -1

	QuotaPoolTypeNormal  = "normal"
	QuotaPoolTypeDefault = "default"
	QuotaPoolTypeNewUser = "new_user"

	QuotaPoolDefaultName = "产研中心默认额度池(存量)"
	QuotaPoolNewUserName = "默认额度池"

	QuotaPoolAutoRechargeInherit = -1
	QuotaPoolAutoRechargeOff     = 0

	QuotaPoolTransactionInitialFund    = "initial_fund"
	QuotaPoolTransactionManualRefill   = "manual_refill"
	QuotaPoolTransactionMonthlyRefill  = "monthly_refill"
	QuotaPoolTransactionAllocateAuto   = "allocate_auto"
	QuotaPoolTransactionAllocateManual = "allocate_manual"
	QuotaPoolTransactionReclaimUser    = "reclaim_user"
	QuotaPoolTransactionAdjustBase     = "adjust_base_quota"
)

var (
	ErrQuotaPoolNotFound              = errors.New("额度池不存在")
	ErrQuotaPoolDisabled              = errors.New("额度池已禁用")
	ErrQuotaPoolDefaultReadonly       = errors.New("默认额度池不支持该操作")
	ErrQuotaPoolInvalidAmount         = errors.New("额度池金额无效")
	ErrQuotaPoolInsufficientQuota     = errors.New("额度池余额不足")
	ErrQuotaPoolInsufficientUserQuota = errors.New("用户额度不足")
	ErrQuotaPoolReclaimTriggersAuto   = errors.New("扣减后会触发自动充值")
	ErrQuotaPoolNameExists            = errors.New("额度池名称已存在")
	ErrQuotaPoolRefillLimited         = errors.New("额度池临时额度超出限制")
	ErrQuotaPoolAdjustLimited         = errors.New("下调后额度池可用额度不能小于 0")
	ErrQuotaPoolMemberMismatch        = errors.New("用户不属于该额度池")
	ErrQuotaPoolSamePool              = errors.New("用户已在该额度池")
	ErrQuotaPoolSystemReadonly        = errors.New("系统额度池不支持该操作")
)

type quotaPoolAdjustLimitedError struct {
	maxReduction int
}

func (e quotaPoolAdjustLimitedError) Error() string {
	return fmt.Sprintf("可用额度不够，最多减少%s", logger.LogQuota(e.maxReduction))
}

func (e quotaPoolAdjustLimitedError) Unwrap() error {
	return ErrQuotaPoolAdjustLimited
}

type QuotaPool struct {
	Id                   int            `json:"id"`
	Name                 string         `json:"name" gorm:"type:varchar(64);index"`
	PoolType             string         `json:"pool_type" gorm:"type:varchar(32);default:'normal';column:pool_type;index"`
	Enabled              bool           `json:"enabled" gorm:"default:true"`
	IsDefault            bool           `json:"is_default" gorm:"default:false;index"`
	BaseQuota            int            `json:"base_quota" gorm:"type:int;default:0;column:base_quota"`
	Quota                int            `json:"quota" gorm:"type:int;default:0"`
	AutoRechargeAmount   int            `json:"auto_recharge_amount" gorm:"type:int;default:-1;column:auto_recharge_amount"`
	WeeklyLimit          int            `json:"weekly_limit" gorm:"type:int;default:-1;column:weekly_limit"`
	MonthlyLimit         int            `json:"monthly_limit" gorm:"type:int;default:-1;column:monthly_limit"`
	MonthlyRefillEnabled bool           `json:"monthly_refill_enabled" gorm:"default:false;column:monthly_refill_enabled"`
	MonthlyRefillTopUp   bool           `json:"monthly_refill_top_up" gorm:"default:false;column:monthly_refill_top_up"`
	MonthlyRefillAmount  int            `json:"monthly_refill_amount" gorm:"type:int;default:0;column:monthly_refill_amount"`
	MonthlyRefillDay     int            `json:"monthly_refill_day" gorm:"type:int;default:1;column:monthly_refill_day"`
	LastRefillMonth      int            `json:"last_refill_month" gorm:"type:int;default:0;column:last_refill_month"`
	CreatedAt            int64          `json:"created_at" gorm:"autoCreateTime;column:created_at"`
	UpdatedAt            int64          `json:"updated_at" gorm:"autoUpdateTime;column:updated_at"`
	DeletedAt            gorm.DeletedAt `json:"deleted_at" gorm:"index"`
}

type QuotaPoolAdmin struct {
	Id        int   `json:"id"`
	PoolId    int   `json:"pool_id" gorm:"index;column:pool_id"`
	UserId    int   `json:"user_id" gorm:"uniqueIndex;column:user_id"`
	Level     int   `json:"level" gorm:"type:int;default:1"`
	CreatedAt int64 `json:"created_at" gorm:"autoCreateTime;column:created_at"`
}

type QuotaPoolTransaction struct {
	Id          int    `json:"id"`
	PoolId      int    `json:"pool_id" gorm:"index;column:pool_id"`
	Type        string `json:"type" gorm:"type:varchar(32);index"`
	Amount      int    `json:"amount" gorm:"type:int;default:0"`
	QuotaBefore int    `json:"quota_before" gorm:"type:int;default:0;column:quota_before"`
	QuotaAfter  int    `json:"quota_after" gorm:"type:int;default:0;column:quota_after"`
	UserId      int    `json:"user_id" gorm:"index;column:user_id"`
	OperatorId  int    `json:"operator_id" gorm:"index;column:operator_id"`
	CreatedAt   int64  `json:"created_at" gorm:"autoCreateTime;column:created_at"`
}

type QuotaPoolTransactionItem struct {
	QuotaPoolTransaction
	UserName     string `json:"user_name"`
	OperatorName string `json:"operator_name"`
}

type QuotaPoolOperationLog struct {
	Id            int    `json:"id"`
	UserId        int    `json:"user_id"`
	Username      string `json:"username"`
	Content       string `json:"content"`
	CreatedAt     int64  `json:"created_at"`
	AdminId       int    `json:"admin_id"`
	AdminUsername string `json:"admin_username"`
	QuotaPoolId   int    `json:"quota_pool_id"`
}

type QuotaPoolTransactionFilter struct {
	UserKeyword    string
	Types          []string
	StartTimestamp int64
	EndTimestamp   int64
}

type QuotaPoolOperationLogFilter struct {
	Keyword        string
	StartTimestamp int64
	EndTimestamp   int64
}

type QuotaPoolAdminSummary struct {
	PoolId int `json:"pool_id"`
	Level  int `json:"level"`
}

type QuotaPoolAdminContact struct {
	Id          int    `json:"id"`
	Username    string `json:"username"`
	DisplayName string `json:"display_name"`
	Email       string `json:"email"`
}

type QuotaPoolSystemAutoRecharge struct {
	Enabled      bool `json:"enabled"`
	Interval     int  `json:"interval"`
	Threshold    int  `json:"threshold"`
	Amount       int  `json:"amount"`
	WeeklyLimit  int  `json:"weekly_limit"`
	MonthlyLimit int  `json:"monthly_limit"`
}

type QuotaPoolBalanceChange struct {
	PoolId      int
	Amount      int
	QuotaBefore int
	QuotaAfter  int
}

type QuotaPoolTransferResult struct {
	PoolChanged bool
	Change      QuotaPoolBalanceChange
}

type QuotaPoolMoveResult struct {
	OldPoolId         int
	NewPoolId         int
	UserQuota         int
	Reclaimed         bool
	TargetNewUserPool bool
	Change            QuotaPoolBalanceChange
}

type QuotaPoolListItem struct {
	QuotaPool
	MemberCount        int64                       `json:"member_count"`
	AdminCount         int64                       `json:"admin_count"`
	SystemAutoRecharge QuotaPoolSystemAutoRecharge `json:"system_auto_recharge"`
}

type QuotaPoolMember struct {
	Id                  int    `json:"id"`
	Username            string `json:"username"`
	DisplayName         string `json:"display_name"`
	Role                int    `json:"role"`
	Status              int    `json:"status"`
	Email               string `json:"email"`
	Group               string `json:"group"`
	Quota               int    `json:"quota"`
	UsedQuota           int    `json:"used_quota"`
	QuotaPoolId         int    `json:"quota_pool_id"`
	QuotaPoolAdminLevel int    `json:"quota_pool_admin_level"`
	CreatedAt           int64  `json:"created_at"`
	LastLoginAt         int64  `json:"last_login_at"`
}

type QuotaPoolCandidate struct {
	Id          int    `json:"id"`
	Username    string `json:"username"`
	DisplayName string `json:"display_name"`
	Email       string `json:"email"`
	Department  string `json:"department"`
}

type QuotaPoolUsageStat struct {
	UserId        int    `json:"user_id" gorm:"column:user_id"`
	Username      string `json:"username" gorm:"column:username"`
	UsedQuota     int    `json:"used_quota" gorm:"column:used_quota"`
	GptQuota      int    `json:"gpt_quota" gorm:"column:gpt_quota"`
	ClaudeQuota   int    `json:"claude_quota" gorm:"column:claude_quota"`
	DeepSeekQuota int    `json:"deepseek_quota" gorm:"column:deepseek_quota"`
	GeminiQuota   int    `json:"gemini_quota" gorm:"column:gemini_quota"`
	QwenQuota     int    `json:"qwen_quota" gorm:"column:qwen_quota"`
	OtherQuota    int    `json:"other_quota" gorm:"column:other_quota"`
}

type QuotaPoolRechargeStat struct {
	Type   string `json:"type" gorm:"column:type"`
	Count  int    `json:"count" gorm:"column:count"`
	Amount int    `json:"amount" gorm:"column:amount"`
}

type QuotaPoolStats struct {
	Usage         []QuotaPoolUsageStat    `json:"usage"`
	Recharge      []QuotaPoolRechargeStat `json:"recharge"`
	TotalUsage    int                     `json:"total_usage"`
	TotalRefill   int                     `json:"total_refill"`
	TotalAllocate int                     `json:"total_allocate"`
}

func normalizeQuotaPoolId(poolId int) int {
	if poolId < 0 {
		return QuotaPoolDefaultUserPoolId
	}
	return poolId
}

func IsQuotaPoolMemberRole(role int) bool {
	return role == common.RoleCommonUser || role == common.RoleQuotaPoolSuperAdmin || role == common.RoleAdminUser
}

func quotaPoolMemberRoles() []int {
	return []int{common.RoleCommonUser, common.RoleQuotaPoolSuperAdmin, common.RoleAdminUser}
}

func newDefaultQuotaPool() *QuotaPool {
	return &QuotaPool{
		Name:               QuotaPoolDefaultName,
		PoolType:           QuotaPoolTypeDefault,
		Enabled:            true,
		IsDefault:          true,
		BaseQuota:          QuotaPoolUnlimitedQuota,
		Quota:              QuotaPoolUnlimitedQuota,
		AutoRechargeAmount: QuotaPoolAutoRechargeInherit,
		WeeklyLimit:        QuotaPoolAutoRechargeInherit,
		MonthlyLimit:       QuotaPoolAutoRechargeInherit,
		MonthlyRefillDay:   1,
	}
}

func newNewUserQuotaPool() *QuotaPool {
	return &QuotaPool{
		Name:                 QuotaPoolNewUserName,
		PoolType:             QuotaPoolTypeNewUser,
		Enabled:              true,
		IsDefault:            false,
		BaseQuota:            QuotaPoolUnlimitedQuota,
		Quota:                QuotaPoolUnlimitedQuota,
		AutoRechargeAmount:   QuotaPoolAutoRechargeOff,
		WeeklyLimit:          QuotaPoolAutoRechargeOff,
		MonthlyLimit:         QuotaPoolAutoRechargeOff,
		MonthlyRefillEnabled: false,
		MonthlyRefillDay:     1,
	}
}

func (pool *QuotaPool) IsNewUserPool() bool {
	return pool != nil && pool.PoolType == QuotaPoolTypeNewUser
}

func (pool *QuotaPool) IsSystemPool() bool {
	return pool != nil && (pool.IsDefault || pool.PoolType == QuotaPoolTypeDefault || pool.PoolType == QuotaPoolTypeNewUser)
}

func IsQuotaPoolCandidateSourcePoolId(poolId int) bool {
	poolId = normalizeQuotaPoolId(poolId)
	if poolId == QuotaPoolDefaultUserPoolId {
		return true
	}
	pool, err := GetNewUserQuotaPool()
	return err == nil && pool.Id == poolId
}

func GetDefaultQuotaPool() (*QuotaPool, error) {
	pool := &QuotaPool{}
	if err := DB.Where("is_default = ? OR pool_type = ?", true, QuotaPoolTypeDefault).
		Order("is_default desc, id asc").First(pool).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrQuotaPoolNotFound
		}
		return nil, err
	}
	return pool, nil
}

func GetNewUserQuotaPool() (*QuotaPool, error) {
	pool := &QuotaPool{}
	if err := DB.Where("pool_type = ?", QuotaPoolTypeNewUser).Order("id asc").First(pool).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrQuotaPoolNotFound
		}
		return nil, err
	}
	return pool, nil
}

func SyncDefaultQuotaPool() (*QuotaPool, error) {
	pool, err := GetDefaultQuotaPool()
	if err == nil {
		return normalizeDefaultQuotaPool(DB, pool)
	}
	if !errors.Is(err, ErrQuotaPoolNotFound) {
		return nil, err
	}
	pool = newDefaultQuotaPool()
	if err := DB.Create(pool).Error; err != nil {
		return nil, err
	}
	return pool, nil
}

func normalizeDefaultQuotaPool(tx *gorm.DB, pool *QuotaPool) (*QuotaPool, error) {
	if pool.PoolType == QuotaPoolTypeDefault && pool.Name == QuotaPoolDefaultName && pool.IsDefault {
		return pool, nil
	}
	updates := map[string]interface{}{
		"name":       QuotaPoolDefaultName,
		"pool_type":  QuotaPoolTypeDefault,
		"is_default": true,
	}
	if err := tx.Model(&QuotaPool{}).Where("id = ?", pool.Id).Updates(updates).Error; err != nil {
		return nil, err
	}
	pool.Name = QuotaPoolDefaultName
	pool.PoolType = QuotaPoolTypeDefault
	pool.IsDefault = true
	return pool, nil
}

func SyncNewUserQuotaPool() (*QuotaPool, error) {
	return syncNewUserQuotaPool(DB)
}

func SyncSystemQuotaPools() error {
	if _, err := SyncDefaultQuotaPool(); err != nil {
		return err
	}
	if _, err := SyncNewUserQuotaPool(); err != nil {
		return err
	}
	return nil
}

func syncNewUserQuotaPool(tx *gorm.DB) (*QuotaPool, error) {
	pool := &QuotaPool{}
	err := tx.Where("pool_type = ?", QuotaPoolTypeNewUser).Order("id asc").First(pool).Error
	if err == nil {
		return normalizeNewUserQuotaPool(tx, pool)
	}
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	pool = newNewUserQuotaPool()
	if err := tx.Create(pool).Error; err != nil {
		return nil, err
	}
	return normalizeNewUserQuotaPool(tx, pool)
}

func normalizeNewUserQuotaPool(tx *gorm.DB, pool *QuotaPool) (*QuotaPool, error) {
	updates := map[string]interface{}{
		"name":                   QuotaPoolNewUserName,
		"pool_type":              QuotaPoolTypeNewUser,
		"enabled":                true,
		"is_default":             false,
		"base_quota":             QuotaPoolUnlimitedQuota,
		"quota":                  QuotaPoolUnlimitedQuota,
		"auto_recharge_amount":   QuotaPoolAutoRechargeOff,
		"weekly_limit":           QuotaPoolAutoRechargeOff,
		"monthly_limit":          QuotaPoolAutoRechargeOff,
		"monthly_refill_enabled": false,
		"monthly_refill_amount":  0,
		"monthly_refill_day":     1,
	}
	if !newUserQuotaPoolNeedsNormalize(pool) {
		return pool, nil
	}
	if err := tx.Model(&QuotaPool{}).Where("id = ?", pool.Id).Updates(updates).Error; err != nil {
		return nil, err
	}
	for key, value := range updates {
		switch key {
		case "name":
			pool.Name = value.(string)
		case "pool_type":
			pool.PoolType = value.(string)
		case "enabled":
			pool.Enabled = value.(bool)
		case "is_default":
			pool.IsDefault = value.(bool)
		case "base_quota":
			pool.BaseQuota = value.(int)
		case "quota":
			pool.Quota = value.(int)
		case "auto_recharge_amount":
			pool.AutoRechargeAmount = value.(int)
		case "weekly_limit":
			pool.WeeklyLimit = value.(int)
		case "monthly_limit":
			pool.MonthlyLimit = value.(int)
		case "monthly_refill_enabled":
			pool.MonthlyRefillEnabled = value.(bool)
		case "monthly_refill_amount":
			pool.MonthlyRefillAmount = value.(int)
		case "monthly_refill_day":
			pool.MonthlyRefillDay = value.(int)
		}
	}
	return pool, nil
}

func newUserQuotaPoolNeedsNormalize(pool *QuotaPool) bool {
	return pool.Name != QuotaPoolNewUserName ||
		pool.PoolType != QuotaPoolTypeNewUser ||
		!pool.Enabled ||
		pool.IsDefault ||
		pool.BaseQuota != QuotaPoolUnlimitedQuota ||
		pool.Quota != QuotaPoolUnlimitedQuota ||
		pool.AutoRechargeAmount != QuotaPoolAutoRechargeOff ||
		pool.WeeklyLimit != QuotaPoolAutoRechargeOff ||
		pool.MonthlyLimit != QuotaPoolAutoRechargeOff ||
		pool.MonthlyRefillEnabled ||
		pool.MonthlyRefillAmount != 0 ||
		pool.MonthlyRefillDay != 1
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

func ListQuotaPoolAdminContacts(poolId int) ([]*QuotaPoolAdminContact, error) {
	var admins []*QuotaPoolAdminContact
	err := DB.Model(&QuotaPoolAdmin{}).
		Select("users.id", "users.username", "users.display_name", "users.email").
		Joins("JOIN users ON users.id = quota_pool_admins.user_id").
		Where("quota_pool_admins.pool_id = ?", poolId).
		Order("quota_pool_admins.id asc").
		Find(&admins).Error
	return admins, err
}

func GetQuotaPoolById(poolId int) (*QuotaPool, error) {
	poolId = normalizeQuotaPoolId(poolId)
	if poolId == QuotaPoolDefaultUserPoolId {
		return GetDefaultQuotaPool()
	}
	pool := &QuotaPool{}
	if err := DB.First(pool, "id = ?", poolId).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrQuotaPoolNotFound
		}
		return nil, err
	}
	return pool, nil
}

func ListQuotaPools() ([]QuotaPoolListItem, error) {
	if common.QuotaPoolEnabled {
		if err := SyncSystemQuotaPools(); err != nil {
			return nil, err
		}
	}
	var pools []QuotaPool
	if err := DB.Order("is_default desc, id desc").Find(&pools).Error; err != nil {
		return nil, err
	}
	items := make([]QuotaPoolListItem, 0, len(pools))
	for _, pool := range pools {
		item := buildQuotaPoolListItem(pool)
		items = append(items, item)
	}
	return items, nil
}

func GetQuotaPoolListItemById(poolId int) (*QuotaPoolListItem, error) {
	pool, err := GetQuotaPoolById(poolId)
	if err != nil {
		return nil, err
	}
	item := buildQuotaPoolListItem(*pool)
	return &item, nil
}

func buildQuotaPoolListItem(pool QuotaPool) QuotaPoolListItem {
	item := QuotaPoolListItem{QuotaPool: pool, SystemAutoRecharge: systemAutoRechargeForQuotaPool()}
	if pool.IsDefault {
		DB.Model(&User{}).Where("quota_pool_id = ?", QuotaPoolDefaultUserPoolId).Count(&item.MemberCount)
		return item
	}
	DB.Model(&User{}).Where("quota_pool_id = ?", pool.Id).Count(&item.MemberCount)
	DB.Model(&QuotaPoolAdmin{}).Where("pool_id = ?", pool.Id).Count(&item.AdminCount)
	return item
}

func systemAutoRechargeForQuotaPool() QuotaPoolSystemAutoRecharge {
	cfg := operation_setting.GetAutoRechargeSetting()
	return QuotaPoolSystemAutoRecharge{
		Enabled:      cfg.Enabled,
		Interval:     cfg.Interval,
		Threshold:    int(float64(cfg.Threshold) * common.QuotaPerUnit),
		Amount:       int(float64(cfg.Amount) * common.QuotaPerUnit),
		WeeklyLimit:  cfg.WeeklyLimit,
		MonthlyLimit: cfg.MonthlyLimit,
	}
}

func ListQuotaPoolTransactions(poolId int, pageInfo *common.PageInfo, filter *QuotaPoolTransactionFilter) ([]*QuotaPoolTransactionItem, int64, error) {
	var total int64
	query := DB.Model(&QuotaPoolTransaction{}).Where("pool_id = ?", poolId)
	query = applyQuotaPoolTransactionFilter(query, filter)
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var txs []*QuotaPoolTransaction
	if err := query.Order("id desc").Limit(pageInfo.GetPageSize()).Offset(pageInfo.GetStartIdx()).Find(&txs).Error; err != nil {
		return nil, 0, err
	}
	items, err := buildQuotaPoolTransactionItems(txs)
	return items, total, err
}

func applyQuotaPoolTransactionFilter(query *gorm.DB, filter *QuotaPoolTransactionFilter) *gorm.DB {
	if filter == nil {
		return query
	}
	if len(filter.Types) > 0 {
		query = query.Where("type IN ?", filter.Types)
	}
	if filter.StartTimestamp > 0 {
		query = query.Where("created_at >= ?", filter.StartTimestamp)
	}
	if filter.EndTimestamp > 0 {
		query = query.Where("created_at <= ?", filter.EndTimestamp)
	}
	if filter.UserKeyword == "" {
		return query
	}

	keyword := strings.TrimSpace(filter.UserKeyword)
	if keyword == "" {
		return query
	}
	like := "%" + keyword + "%"
	userQuery := DB.Model(&User{}).Select("id")
	if userId, err := strconv.Atoi(keyword); err == nil {
		userQuery = userQuery.Where("id = ? OR username LIKE ? OR email LIKE ? OR display_name LIKE ?", userId, like, like, like)
		query = query.Where("user_id = ? OR operator_id = ? OR user_id IN (?) OR operator_id IN (?)", userId, userId, userQuery, userQuery)
		return query
	}
	userQuery = userQuery.Where("username LIKE ? OR email LIKE ? OR display_name LIKE ?", like, like, like)
	query = query.Where("user_id IN (?) OR operator_id IN (?)", userQuery, userQuery)
	return query
}

func buildQuotaPoolTransactionItems(txs []*QuotaPoolTransaction) ([]*QuotaPoolTransactionItem, error) {
	userIds := make([]int, 0, len(txs)*2)
	seen := map[int]bool{}
	for _, tx := range txs {
		for _, userId := range []int{tx.UserId, tx.OperatorId} {
			if userId <= 0 || seen[userId] {
				continue
			}
			seen[userId] = true
			userIds = append(userIds, userId)
		}
	}

	userNames := map[int]string{}
	if len(userIds) > 0 {
		var users []User
		if err := DB.Select("id", "username").Where("id IN ?", userIds).Find(&users).Error; err != nil {
			return nil, err
		}
		for _, user := range users {
			userNames[user.Id] = user.Username
		}
	}

	items := make([]*QuotaPoolTransactionItem, 0, len(txs))
	for _, tx := range txs {
		items = append(items, &QuotaPoolTransactionItem{
			QuotaPoolTransaction: *tx,
			UserName:             userNames[tx.UserId],
			OperatorName:         userNames[tx.OperatorId],
		})
	}
	return items, nil
}

func ListQuotaPoolOperationLogs(poolId int, pageInfo *common.PageInfo, filter *QuotaPoolOperationLogFilter) ([]*QuotaPoolOperationLog, int64, error) {
	query := LOG_DB.Model(&Log{}).Where("type = ?", LogTypeManage)
	query = applyQuotaPoolOperationLogFilter(query, poolId, filter)
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var logs []*Log
	if err := query.Order("id desc").Limit(pageInfo.GetPageSize()).Offset(pageInfo.GetStartIdx()).Find(&logs).Error; err != nil {
		return nil, 0, err
	}
	items := make([]*QuotaPoolOperationLog, 0, len(logs))
	for _, log := range logs {
		item, ok := quotaPoolOperationLogFromLog(log, poolId)
		if ok {
			items = append(items, item)
		}
	}
	return items, total, nil
}

func applyQuotaPoolOperationLogFilter(query *gorm.DB, poolId int, filter *QuotaPoolOperationLogFilter) *gorm.DB {
	poolIdPatternObjectEnd := fmt.Sprintf("%%{\"quota_pool_id\":%d}%%", poolId)
	poolIdPatternObjectComma := fmt.Sprintf("%%{\"quota_pool_id\":%d,%%", poolId)
	poolIdPatternFieldEnd := fmt.Sprintf("%%,\"quota_pool_id\":%d}%%", poolId)
	poolIdPatternFieldComma := fmt.Sprintf("%%,\"quota_pool_id\":%d,%%", poolId)
	query = query.Where(
		"(other LIKE ? OR other LIKE ? OR other LIKE ? OR other LIKE ?)",
		poolIdPatternObjectEnd,
		poolIdPatternObjectComma,
		poolIdPatternFieldEnd,
		poolIdPatternFieldComma,
	)
	if filter == nil {
		return query
	}
	if filter.StartTimestamp > 0 {
		query = query.Where("created_at >= ?", filter.StartTimestamp)
	}
	if filter.EndTimestamp > 0 {
		query = query.Where("created_at <= ?", filter.EndTimestamp)
	}
	keyword := strings.TrimSpace(filter.Keyword)
	if keyword == "" {
		return query
	}
	likePattern, err := sanitizeLikePattern("%" + keyword + "%")
	if err != nil {
		return query
	}
	query = query.Where("(content LIKE ? ESCAPE '!' OR username LIKE ? ESCAPE '!' OR other LIKE ? ESCAPE '!')", likePattern, likePattern, likePattern)
	return query
}

func quotaPoolOperationLogFromLog(log *Log, poolId int) (*QuotaPoolOperationLog, bool) {
	if log == nil || log.Other == "" {
		return nil, false
	}
	other, err := common.StrToMap(log.Other)
	if err != nil {
		return nil, false
	}
	adminInfo, ok := other["admin_info"].(map[string]interface{})
	if !ok {
		return nil, false
	}
	logPoolId := intFromLogValue(adminInfo["quota_pool_id"])
	if logPoolId != poolId {
		return nil, false
	}
	return &QuotaPoolOperationLog{
		Id:            log.Id,
		UserId:        log.UserId,
		Username:      log.Username,
		Content:       log.Content,
		CreatedAt:     log.CreatedAt,
		AdminId:       intFromLogValue(adminInfo["admin_id"]),
		AdminUsername: stringFromLogValue(adminInfo["admin_username"]),
		QuotaPoolId:   logPoolId,
	}, true
}

func intFromLogValue(value interface{}) int {
	switch v := value.(type) {
	case int:
		return v
	case int64:
		return int(v)
	case float64:
		return int(v)
	case string:
		n, _ := strconv.Atoi(v)
		return n
	default:
		return 0
	}
}

func stringFromLogValue(value interface{}) string {
	if s, ok := value.(string); ok {
		return s
	}
	return ""
}

func ListQuotaPoolMembers(poolId int, pageInfo *common.PageInfo) ([]*QuotaPoolMember, int64, error) {
	var total int64
	query := DB.Model(&User{}).Where("quota_pool_id = ?", poolId)
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	selectColumns := []string{
		"users.id",
		"users.username",
		"users.display_name",
		"users.role",
		"users.status",
		"users.email",
		"users." + commonGroupCol + " AS " + commonGroupCol,
		"users.quota",
		"users.used_quota",
		"users.quota_pool_id",
		"COALESCE(quota_pool_admins.level, 0) AS quota_pool_admin_level",
		"users.created_at",
		"users.last_login_at",
	}
	var users []*QuotaPoolMember
	err := query.Select(strings.Join(selectColumns, ", ")).
		Joins("LEFT JOIN quota_pool_admins ON quota_pool_admins.user_id = users.id AND quota_pool_admins.pool_id = ?", poolId).
		Order("users.id desc").Limit(pageInfo.GetPageSize()).Offset(pageInfo.GetStartIdx()).Find(&users).Error
	return users, total, err
}

func ListDefaultQuotaPoolCandidates(keyword string, pageInfo *common.PageInfo) ([]*QuotaPoolCandidate, int64, error) {
	poolIds := []int{QuotaPoolDefaultUserPoolId}
	if pool, err := GetNewUserQuotaPool(); err == nil {
		poolIds = append(poolIds, pool.Id)
	} else if !errors.Is(err, ErrQuotaPoolNotFound) {
		return nil, 0, err
	}
	query := DB.Model(&User{}).Where("quota_pool_id IN ? AND role IN ? AND status = ?", poolIds, quotaPoolMemberRoles(), common.UserStatusEnabled)
	if keyword != "" {
		like := "%" + keyword + "%"
		if userId, err := strconv.Atoi(keyword); err == nil {
			query = query.Where("id = ? OR username LIKE ? OR email LIKE ? OR display_name LIKE ? OR department LIKE ?", userId, like, like, like, like)
		} else {
			query = query.Where("username LIKE ? OR email LIKE ? OR display_name LIKE ? OR department LIKE ?", like, like, like, like)
		}
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var users []*QuotaPoolCandidate
	selectColumns := []string{
		"id",
		"username",
		"display_name",
		"email",
		"department",
	}
	err := query.Select(strings.Join(selectColumns, ", ")).
		Order("id desc").Limit(pageInfo.GetPageSize()).Offset(pageInfo.GetStartIdx()).Find(&users).Error
	return users, total, err
}

func GetQuotaPoolStats(poolId int, startTimestamp int64, endTimestamp int64) (*QuotaPoolStats, error) {
	usage, err := getQuotaPoolUsageStats(poolId, startTimestamp, endTimestamp)
	if err != nil {
		return nil, err
	}
	recharge, err := getQuotaPoolRechargeStats(poolId, startTimestamp, endTimestamp)
	if err != nil {
		return nil, err
	}
	stats := &QuotaPoolStats{
		Usage:    usage,
		Recharge: recharge,
	}
	for _, item := range usage {
		stats.TotalUsage += item.UsedQuota
	}
	for _, item := range recharge {
		if item.Type == QuotaPoolTransactionAllocateAuto || item.Type == QuotaPoolTransactionAllocateManual {
			stats.TotalAllocate += -item.Amount
		} else if item.Type == QuotaPoolTransactionInitialFund || item.Type == QuotaPoolTransactionManualRefill || item.Type == QuotaPoolTransactionMonthlyRefill {
			stats.TotalRefill += item.Amount
		}
	}
	return stats, nil
}

func getQuotaPoolUsageStats(poolId int, startTimestamp int64, endTimestamp int64) ([]QuotaPoolUsageStat, error) {
	var members []User
	if err := DB.Select("id", "username").Where("quota_pool_id = ?", poolId).Find(&members).Error; err != nil {
		return nil, err
	}
	if len(members) == 0 {
		return []QuotaPoolUsageStat{}, nil
	}
	userIds := make([]int, 0, len(members))
	userNames := make(map[int]string, len(members))
	for _, member := range members {
		userIds = append(userIds, member.Id)
		userNames[member.Id] = member.Username
	}

	usedMap, err := getQuotaPoolUsageStatsFromUsageData(startTimestamp, endTimestamp, userIds)
	if err != nil {
		return nil, err
	}
	results := make([]QuotaPoolUsageStat, 0, len(usedMap))
	for userId, stat := range usedMap {
		if stat.UsedQuota == 0 {
			continue
		}
		results = append(results, QuotaPoolUsageStat{
			UserId:        userId,
			Username:      userNames[userId],
			UsedQuota:     stat.UsedQuota,
			GptQuota:      stat.GptQuota,
			ClaudeQuota:   stat.ClaudeQuota,
			DeepSeekQuota: stat.DeepSeekQuota,
			GeminiQuota:   stat.GeminiQuota,
			QwenQuota:     stat.QwenQuota,
			OtherQuota:    stat.OtherQuota,
		})
	}
	sort.Slice(results, func(i, j int) bool {
		return results[i].UsedQuota > results[j].UsedQuota
	})
	return results, nil
}

func getQuotaPoolUsageStatsFromUsageData(startTimestamp int64, endTimestamp int64, userIds []int) (map[int]rechargeUsageStat, error) {
	usedMap := make(map[int]rechargeUsageStat, len(userIds))
	if len(userIds) == 0 {
		return usedMap, nil
	}

	currentHourStart := currentHourStartTimestamp()
	if settledStart, settledEnd, ok := topUsersSettledRange(startTimestamp, endTimestamp, currentHourStart); ok {
		settledStats, err := getRechargeUsageStatsFromQuotaData(settledStart, settledEnd, userIds)
		if err != nil {
			return nil, err
		}
		mergeRechargeUsageStats(usedMap, settledStats)

		missingUserIds := rechargeUsageMissingUserIds(userIds, settledStats)
		if len(missingUserIds) > 0 {
			missingStats, err := getRechargeUsageStatsFromLogs(settledStart, settledEnd, missingUserIds)
			if err != nil {
				return nil, err
			}
			mergeRechargeUsageStats(usedMap, missingStats)
		}
	}

	if currentStart, currentEnd, ok := topUsersCurrentHourRange(startTimestamp, endTimestamp, currentHourStart); ok {
		currentStats, err := getRechargeUsageStatsFromLogs(currentStart, currentEnd, userIds)
		if err != nil {
			return nil, err
		}
		mergeRechargeUsageStats(usedMap, currentStats)
	}

	return usedMap, nil
}

func getQuotaPoolRechargeStats(poolId int, startTimestamp int64, endTimestamp int64) ([]QuotaPoolRechargeStat, error) {
	tx := DB.Model(&QuotaPoolTransaction{}).
		Select("type, COUNT(*) as count, COALESCE(SUM(amount), 0) as amount").
		Where("pool_id = ?", poolId)

	if startTimestamp != 0 {
		tx = tx.Where("created_at >= ?", startTimestamp)
	}
	if endTimestamp != 0 {
		tx = tx.Where("created_at <= ?", endTimestamp)
	}

	var results []QuotaPoolRechargeStat
	err := tx.Group("type").
		Order("type asc").
		Scan(&results).Error
	for i := range results {
		if results[i].Type == QuotaPoolTransactionReclaimUser && results[i].Amount > 0 {
			results[i].Amount = -results[i].Amount
		}
	}
	return results, err
}

func quotaPoolNameExists(tx *gorm.DB, name string, excludeId int) (bool, error) {
	var count int64
	query := tx.Model(&QuotaPool{}).Where("name = ?", name)
	if excludeId > 0 {
		query = query.Where("id <> ?", excludeId)
	}
	if err := query.Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

func defaultQuotaPoolMonthlyRefillDay(now time.Time) int {
	day := now.Day()
	if day > 28 {
		return 28
	}
	return day
}

func CreateQuotaPool(name string, baseQuota int, operatorId int) (*QuotaPool, error) {
	if name == "" || baseQuota <= 0 {
		return nil, ErrQuotaPoolInvalidAmount
	}
	now := time.Now()
	monthlyRefillDay := defaultQuotaPoolMonthlyRefillDay(now)
	currentMonth := now.Year()*100 + int(now.Month())
	var pool *QuotaPool
	err := DB.Transaction(func(tx *gorm.DB) error {
		exists, err := quotaPoolNameExists(tx, name, 0)
		if err != nil {
			return err
		}
		if exists {
			return ErrQuotaPoolNameExists
		}
		pool = &QuotaPool{
			Name:                 name,
			PoolType:             QuotaPoolTypeNormal,
			Enabled:              true,
			IsDefault:            false,
			BaseQuota:            baseQuota,
			Quota:                baseQuota,
			AutoRechargeAmount:   QuotaPoolAutoRechargeInherit,
			WeeklyLimit:          QuotaPoolAutoRechargeInherit,
			MonthlyLimit:         QuotaPoolAutoRechargeInherit,
			MonthlyRefillEnabled: true,
			MonthlyRefillAmount:  baseQuota,
			MonthlyRefillDay:     monthlyRefillDay,
			LastRefillMonth:      currentMonth,
		}
		if err := tx.Create(pool).Error; err != nil {
			return err
		}
		return tx.Create(&QuotaPoolTransaction{
			PoolId:      pool.Id,
			Type:        QuotaPoolTransactionInitialFund,
			Amount:      baseQuota,
			QuotaBefore: 0,
			QuotaAfter:  baseQuota,
			OperatorId:  operatorId,
		}).Error
	})
	if err != nil {
		return nil, err
	}
	return pool, nil
}

func UpdateQuotaPoolConfig(poolId int, updates map[string]interface{}, operatorId int) (*QuotaPoolBalanceChange, error) {
	if poolId == QuotaPoolDefaultUserPoolId {
		return nil, ErrQuotaPoolDefaultReadonly
	}
	var change *QuotaPoolBalanceChange
	err := DB.Transaction(func(tx *gorm.DB) error {
		pool := &QuotaPool{}
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(pool, "id = ?", poolId).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrQuotaPoolNotFound
			}
			return err
		}
		if pool.IsSystemPool() {
			return ErrQuotaPoolSystemReadonly
		}
		if nameVal, ok := updates["name"].(string); ok && nameVal != "" && nameVal != pool.Name {
			exists, err := quotaPoolNameExists(tx, nameVal, poolId)
			if err != nil {
				return err
			}
			if exists {
				return ErrQuotaPoolNameExists
			}
		}
		if baseQuota, ok := updates["base_quota"].(int); ok {
			if baseQuota <= 0 {
				return ErrQuotaPoolInvalidAmount
			}
			delta := baseQuota - pool.BaseQuota
			quotaAfter := pool.Quota + delta
			if quotaAfter < 0 {
				return quotaPoolAdjustLimitedError{maxReduction: pool.Quota}
			}
			updates["quota"] = quotaAfter
			change = &QuotaPoolBalanceChange{
				PoolId:      poolId,
				Amount:      delta,
				QuotaBefore: pool.Quota,
				QuotaAfter:  quotaAfter,
			}
		}
		if err := tx.Model(&QuotaPool{}).Where("id = ?", poolId).Updates(updates).Error; err != nil {
			return err
		}
		if change != nil && change.Amount != 0 {
			return tx.Create(&QuotaPoolTransaction{
				PoolId:      poolId,
				Type:        QuotaPoolTransactionAdjustBase,
				Amount:      change.Amount,
				QuotaBefore: change.QuotaBefore,
				QuotaAfter:  change.QuotaAfter,
				OperatorId:  operatorId,
			}).Error
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return change, nil
}

func SetQuotaPoolEnabled(poolId int, enabled bool) error {
	if poolId == QuotaPoolDefaultUserPoolId {
		return ErrQuotaPoolDefaultReadonly
	}
	return DB.Transaction(func(tx *gorm.DB) error {
		pool := &QuotaPool{}
		if err := tx.First(pool, "id = ?", poolId).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrQuotaPoolNotFound
			}
			return err
		}
		if pool.IsSystemPool() {
			return ErrQuotaPoolSystemReadonly
		}
		return tx.Model(&QuotaPool{}).Where("id = ?", poolId).Update("enabled", enabled).Error
	})
}

func DeleteQuotaPool(poolId int) error {
	if poolId == QuotaPoolDefaultUserPoolId {
		return ErrQuotaPoolDefaultReadonly
	}
	return DB.Transaction(func(tx *gorm.DB) error {
		pool := &QuotaPool{}
		if err := tx.First(pool, "id = ?", poolId).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrQuotaPoolNotFound
			}
			return err
		}
		if pool.IsSystemPool() {
			return ErrQuotaPoolSystemReadonly
		}
		var count int64
		if err := tx.Model(&User{}).Where("quota_pool_id = ?", poolId).Count(&count).Error; err != nil {
			return err
		}
		if count > 0 {
			return errors.New("额度池仍有成员，不能删除")
		}
		if err := tx.Where("pool_id = ?", poolId).Delete(&QuotaPoolAdmin{}).Error; err != nil {
			return err
		}
		return tx.Delete(pool).Error
	})
}

func TransferQuotaFromPoolToUser(poolId int, userId int, amount int) (*QuotaPoolTransferResult, error) {
	if amount <= 0 {
		return nil, ErrQuotaPoolInvalidAmount
	}
	poolId = normalizeQuotaPoolId(poolId)
	if poolId == QuotaPoolDefaultUserPoolId {
		if err := IncreaseUserQuota(userId, amount, true); err != nil {
			return nil, err
		}
		return &QuotaPoolTransferResult{PoolChanged: false}, nil
	}

	result := &QuotaPoolTransferResult{PoolChanged: true}
	err := DB.Transaction(func(tx *gorm.DB) error {
		pool := &QuotaPool{}
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(pool, "id = ?", poolId).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrQuotaPoolNotFound
			}
			return err
		}
		if pool.IsSystemPool() {
			return ErrQuotaPoolSystemReadonly
		}
		if !pool.Enabled {
			return ErrQuotaPoolDisabled
		}
		user := &User{}
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(user, "id = ?", userId).Error; err != nil {
			return err
		}
		if normalizeQuotaPoolId(user.QuotaPoolId) != poolId {
			return ErrQuotaPoolMemberMismatch
		}
		if pool.Quota < amount {
			return ErrQuotaPoolInsufficientQuota
		}
		before := pool.Quota
		debitResult := tx.Model(&QuotaPool{}).
			Where("id = ? AND quota >= ?", poolId, amount).
			Update("quota", gorm.Expr("quota - ?", amount))
		if debitResult.Error != nil {
			return debitResult.Error
		}
		if debitResult.RowsAffected == 0 {
			return ErrQuotaPoolInsufficientQuota
		}
		if err := tx.Model(&User{}).Where("id = ?", userId).Update("quota", gorm.Expr("quota + ?", amount)).Error; err != nil {
			return err
		}
		result.Change = QuotaPoolBalanceChange{
			PoolId:      poolId,
			Amount:      -amount,
			QuotaBefore: before,
			QuotaAfter:  before - amount,
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	gopoolUpdateUserQuotaCache(userId)
	return result, nil
}

func ReclaimQuotaFromUserToPool(poolId int, userId int, amount int, autoRechargeThresholdQuota int) (*QuotaPoolTransferResult, error) {
	if amount <= 0 {
		return nil, ErrQuotaPoolInvalidAmount
	}
	poolId = normalizeQuotaPoolId(poolId)
	if poolId == QuotaPoolDefaultUserPoolId {
		return nil, ErrQuotaPoolDefaultReadonly
	}

	result := &QuotaPoolTransferResult{PoolChanged: true}
	err := DB.Transaction(func(tx *gorm.DB) error {
		pool := &QuotaPool{}
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(pool, "id = ?", poolId).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrQuotaPoolNotFound
			}
			return err
		}
		if pool.IsSystemPool() {
			return ErrQuotaPoolSystemReadonly
		}
		if !pool.Enabled {
			return ErrQuotaPoolDisabled
		}
		user := &User{}
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(user, "id = ?", userId).Error; err != nil {
			return err
		}
		if normalizeQuotaPoolId(user.QuotaPoolId) != poolId {
			return ErrQuotaPoolMemberMismatch
		}
		if user.Quota < amount {
			return ErrQuotaPoolInsufficientUserQuota
		}
		if autoRechargeThresholdQuota >= 0 && user.Quota-amount <= autoRechargeThresholdQuota {
			return ErrQuotaPoolReclaimTriggersAuto
		}
		before := pool.Quota
		debitResult := tx.Model(&User{}).
			Where("id = ? AND quota >= ?", userId, amount).
			Update("quota", gorm.Expr("quota - ?", amount))
		if debitResult.Error != nil {
			return debitResult.Error
		}
		if debitResult.RowsAffected == 0 {
			return ErrQuotaPoolInsufficientUserQuota
		}
		if err := tx.Model(&QuotaPool{}).Where("id = ?", poolId).Update("quota", gorm.Expr("quota + ?", amount)).Error; err != nil {
			return err
		}
		result.Change = QuotaPoolBalanceChange{
			PoolId:      poolId,
			Amount:      amount,
			QuotaBefore: before,
			QuotaAfter:  before + amount,
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	gopoolUpdateUserQuotaCache(userId)
	return result, nil
}

func gopoolUpdateUserQuotaCache(userId int) {
	if !common.RedisEnabled {
		return
	}
	quota, err := GetUserQuota(userId, true)
	if err != nil {
		common.SysLog(fmt.Sprintf("failed to reload user quota cache for user %d: %s", userId, err.Error()))
		return
	}
	if err := updateUserQuotaCache(userId, quota); err != nil {
		common.SysLog(fmt.Sprintf("failed to update user quota cache for user %d: %s", userId, err.Error()))
	}
}

func MoveUserQuotaPool(userId int, targetPoolId int) (*QuotaPoolMoveResult, error) {
	return moveUserQuotaPool(userId, targetPoolId, false)
}

func MoveUserQuotaPoolAllowSystemTarget(userId int, targetPoolId int) (*QuotaPoolMoveResult, error) {
	return moveUserQuotaPool(userId, targetPoolId, true)
}

func moveUserQuotaPool(userId int, targetPoolId int, allowSystemTarget bool) (*QuotaPoolMoveResult, error) {
	targetPoolId = normalizeQuotaPoolId(targetPoolId)
	result := &QuotaPoolMoveResult{NewPoolId: targetPoolId}

	err := DB.Transaction(func(tx *gorm.DB) error {
		if targetPoolId != QuotaPoolDefaultUserPoolId {
			target := &QuotaPool{}
			if err := tx.First(target, "id = ?", targetPoolId).Error; err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					return ErrQuotaPoolNotFound
				}
				return err
			}
			result.TargetNewUserPool = target.IsNewUserPool()
			if target.IsSystemPool() && !allowSystemTarget {
				return ErrQuotaPoolSystemReadonly
			}
			if !target.Enabled {
				return ErrQuotaPoolDisabled
			}
		}

		user := &User{}
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(user, "id = ?", userId).Error; err != nil {
			return err
		}
		oldPoolId := normalizeQuotaPoolId(user.QuotaPoolId)
		result.OldPoolId = oldPoolId
		result.UserQuota = user.Quota
		if oldPoolId == targetPoolId {
			return ErrQuotaPoolSamePool
		}

		var oldPool *QuotaPool
		shouldReclaimOldPool := oldPoolId != QuotaPoolDefaultUserPoolId && user.Quota > 0
		if shouldReclaimOldPool {
			oldPool = &QuotaPool{}
			if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(oldPool, "id = ?", oldPoolId).Error; err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					return ErrQuotaPoolNotFound
				}
				return err
			}
			if oldPool.IsNewUserPool() {
				shouldReclaimOldPool = false
			}
		}
		if shouldReclaimOldPool {
			before := oldPool.Quota
			if err := tx.Model(&QuotaPool{}).Where("id = ?", oldPoolId).Update("quota", gorm.Expr("quota + ?", user.Quota)).Error; err != nil {
				return err
			}
			result.Reclaimed = true
			result.Change = QuotaPoolBalanceChange{
				PoolId:      oldPoolId,
				Amount:      user.Quota,
				QuotaBefore: before,
				QuotaAfter:  before + user.Quota,
			}
		}

		if err := tx.Model(&User{}).Where("id = ?", userId).Updates(map[string]interface{}{
			"quota":         0,
			"quota_pool_id": targetPoolId,
		}).Error; err != nil {
			return err
		}
		if oldPoolId != targetPoolId {
			if err := tx.Where("user_id = ? AND pool_id = ?", userId, oldPoolId).Delete(&QuotaPoolAdmin{}).Error; err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	gopoolUpdateUserQuotaCache(userId)
	return result, nil
}

func GrantQuotaPoolAdmin(poolId int, userId int, level int) error {
	if level != QuotaPoolAdminLevelV1 {
		return errors.New("额度池只支持池管理员权限")
	}
	if poolId == QuotaPoolDefaultUserPoolId {
		return ErrQuotaPoolDefaultReadonly
	}
	return DB.Transaction(func(tx *gorm.DB) error {
		pool := &QuotaPool{}
		if err := tx.First(pool, "id = ?", poolId).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrQuotaPoolNotFound
			}
			return err
		}
		if pool.IsSystemPool() {
			return ErrQuotaPoolSystemReadonly
		}
		if !pool.Enabled {
			return ErrQuotaPoolDisabled
		}
		user := &User{}
		if err := tx.First(user, "id = ?", userId).Error; err != nil {
			return err
		}
		if !IsQuotaPoolMemberRole(user.Role) {
			return errors.New("只能任命普通用户、池超级管理员或系统子管理员为额度池管理员")
		}
		if normalizeQuotaPoolId(user.QuotaPoolId) != poolId {
			return errors.New("用户不是该额度池成员")
		}
		admin := &QuotaPoolAdmin{}
		err := tx.Where("user_id = ?", userId).First(admin).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return tx.Create(&QuotaPoolAdmin{PoolId: poolId, UserId: userId, Level: level}).Error
		}
		if err != nil {
			return err
		}
		if admin.PoolId != poolId {
			return errors.New("用户已是其他额度池管理员")
		}
		return tx.Model(&QuotaPoolAdmin{}).Where("id = ?", admin.Id).Update("level", level).Error
	})
}

func RevokeQuotaPoolAdmin(poolId int, userId int) error {
	if poolId == QuotaPoolDefaultUserPoolId {
		return ErrQuotaPoolDefaultReadonly
	}
	pool := &QuotaPool{}
	if err := DB.First(pool, "id = ?", poolId).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrQuotaPoolNotFound
		}
		return err
	}
	if pool.IsSystemPool() {
		return ErrQuotaPoolSystemReadonly
	}
	return DB.Where("pool_id = ? AND user_id = ?", poolId, userId).Delete(&QuotaPoolAdmin{}).Error
}

func RecordQuotaPoolTransaction(poolId int, txType string, amount int, before int, after int, userId int, operatorId int) {
	if poolId == QuotaPoolDefaultUserPoolId {
		return
	}
	record := &QuotaPoolTransaction{
		PoolId:      poolId,
		Type:        txType,
		Amount:      amount,
		QuotaBefore: before,
		QuotaAfter:  after,
		UserId:      userId,
		OperatorId:  operatorId,
	}
	if err := DB.Create(record).Error; err != nil {
		common.SysLog("failed to record quota pool transaction: " + err.Error())
	}
}

func CountQuotaPoolManualRefills(poolId int, startTs int64, endTs int64) (int64, error) {
	var count int64
	err := DB.Model(&QuotaPoolTransaction{}).
		Where("pool_id = ? AND type = ? AND created_at >= ? AND created_at < ?", poolId, QuotaPoolTransactionManualRefill, startTs, endTs).
		Count(&count).Error
	return count, err
}

func AddQuotaPoolManualRefill(poolId int, amount int, operatorId int) (*QuotaPoolBalanceChange, error) {
	if amount <= 0 {
		return nil, ErrQuotaPoolInvalidAmount
	}
	now := time.Now()
	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location()).Unix()
	nextMonthStart := time.Date(now.Year(), now.Month()+1, 1, 0, 0, 0, 0, now.Location()).Unix()

	change := &QuotaPoolBalanceChange{PoolId: poolId, Amount: amount}
	err := DB.Transaction(func(tx *gorm.DB) error {
		pool := &QuotaPool{}
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(pool, "id = ?", poolId).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrQuotaPoolNotFound
			}
			return err
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
			Where("pool_id = ? AND type = ? AND created_at >= ? AND created_at < ?", poolId, QuotaPoolTransactionManualRefill, monthStart, nextMonthStart).
			Count(&count).Error; err != nil {
			return err
		}
		if count >= 2 {
			return ErrQuotaPoolRefillLimited
		}
		before := pool.Quota
		if err := tx.Model(&QuotaPool{}).Where("id = ?", poolId).Updates(map[string]interface{}{
			"quota":      gorm.Expr("quota + ?", amount),
			"base_quota": gorm.Expr("base_quota + ?", amount),
		}).Error; err != nil {
			return err
		}
		after := before + amount
		change.QuotaBefore = before
		change.QuotaAfter = after
		return tx.Create(&QuotaPoolTransaction{
			PoolId:      poolId,
			Type:        QuotaPoolTransactionManualRefill,
			Amount:      amount,
			QuotaBefore: before,
			QuotaAfter:  after,
			OperatorId:  operatorId,
		}).Error
	})
	if err != nil {
		return nil, err
	}
	return change, nil
}

func FormatQuotaPoolTransferLog(operatorId int, amount int) string {
	return fmt.Sprintf("池管理员(ID:%d)添加%s临时额度", operatorId, logger.LogQuota(amount))
}
