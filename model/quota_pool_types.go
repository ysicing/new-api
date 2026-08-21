package model

import (
	"errors"

	"gorm.io/gorm"
)

const (
	QuotaPoolAdminLevelV1 = 1
	QuotaPoolAdminLevelV2 = 2

	QuotaPoolDefaultUserPoolId = 0
	QuotaPoolUnlimitedQuota    = -1

	QuotaPoolTypeNormal  = "normal"
	QuotaPoolTypeDefault = "default"
	QuotaPoolTypeNewUser = "new_user"

	QuotaPoolDefaultName = "产研中心默认额度池(存量)"
	QuotaPoolNewUserName = "默认额度池"

	QuotaPoolAutoRechargeInherit = -1
	QuotaPoolAutoRechargeOff     = 0
)

const (
	QuotaPoolTransactionInitialFund    = "initial_fund"
	QuotaPoolTransactionManualRefill   = "manual_refill"
	QuotaPoolTransactionMonthlyRefill  = "monthly_refill"
	QuotaPoolTransactionAllocateAuto   = "allocate_auto"
	QuotaPoolTransactionAllocateManual = "allocate_manual"
	QuotaPoolTransactionReclaimUser    = "reclaim_user"
	QuotaPoolTransactionAdjustBase     = "adjust_base_quota"
)

var (
	ErrQuotaPoolNotFound          = errors.New("quota pool not found")
	ErrQuotaPoolFeatureDisabled   = errors.New("quota pool feature disabled")
	ErrQuotaPoolDisabled          = errors.New("quota pool disabled")
	ErrQuotaPoolInvalidAmount     = errors.New("quota pool invalid amount")
	ErrQuotaPoolInsufficientQuota = errors.New("quota pool insufficient quota")
	ErrQuotaPoolMemberMismatch    = errors.New("quota pool member mismatch")
	ErrQuotaPoolPermissionDenied  = errors.New("quota pool permission denied")
	ErrQuotaPoolSystemReadonly    = errors.New("quota pool system pool is read-only")
	ErrQuotaPoolSamePool          = errors.New("user already belongs to quota pool")
	ErrQuotaPoolNameExists        = errors.New("quota pool name exists")
	ErrQuotaPoolRefillLimited     = errors.New("quota pool refill limited")
	ErrQuotaPoolAdminConflict     = errors.New("user manages another quota pool")
	ErrQuotaPoolHasMembers        = errors.New("quota pool has members")
	ErrQuotaPoolAdjustLimited     = errors.New("quota pool adjustment exceeds available quota")
)

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

func (pool *QuotaPool) IsNewUserPool() bool {
	return pool != nil && pool.PoolType == QuotaPoolTypeNewUser
}

func (pool *QuotaPool) IsSystemPool() bool {
	return pool != nil && (pool.IsDefault || pool.PoolType == QuotaPoolTypeDefault || pool.PoolType == QuotaPoolTypeNewUser)
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

type QuotaPoolBalanceChange struct {
	PoolId      int
	Amount      int
	QuotaBefore int
	QuotaAfter  int
}

type QuotaPoolMoveResult struct {
	OldPoolId         int
	NewPoolId         int
	UserQuota         int
	Reclaimed         bool
	TargetNewUserPool bool
	Change            QuotaPoolBalanceChange
}

type QuotaPoolAdminSummary struct {
	PoolId int `json:"pool_id"`
	Level  int `json:"level"`
}

type QuotaPoolListItem struct {
	QuotaPool
	MemberCount int64 `json:"member_count"`
	AdminCount  int64 `json:"admin_count"`
}

type QuotaPoolMember struct {
	Id                  int    `json:"id"`
	Username            string `json:"username"`
	DisplayName         string `json:"display_name"`
	Email               string `json:"email"`
	Department          string `json:"department"`
	Role                int    `json:"role"`
	Status              int    `json:"status"`
	Quota               int    `json:"quota"`
	UsedQuota           int    `json:"used_quota"`
	QuotaPoolId         int    `json:"quota_pool_id"`
	QuotaPoolAdminLevel int    `json:"quota_pool_admin_level"`
}

type QuotaPoolTransactionItem struct {
	QuotaPoolTransaction
	UserName     string `json:"user_name"`
	OperatorName string `json:"operator_name"`
}

type QuotaPoolAdminContact struct {
	Id          int    `json:"id"`
	Username    string `json:"username"`
	DisplayName string `json:"display_name"`
	Email       string `json:"email"`
}
