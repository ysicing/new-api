package controller

import (
	"errors"
	"math"
	"net/http"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"
)

type quotaPoolCreateRequest struct {
	Name      string  `json:"name"`
	BaseQuota float64 `json:"base_quota"`
}

type quotaPoolUpdateRequest struct {
	Name                 *string  `json:"name"`
	BaseQuota            *float64 `json:"base_quota"`
	AutoRechargeAmount   *float64 `json:"auto_recharge_amount"`
	WeeklyLimit          *int     `json:"weekly_limit"`
	MonthlyLimit         *int     `json:"monthly_limit"`
	MonthlyRefillEnabled *bool    `json:"monthly_refill_enabled"`
	MonthlyRefillTopUp   *bool    `json:"monthly_refill_top_up"`
	MonthlyRefillAmount  *float64 `json:"monthly_refill_amount"`
	MonthlyRefillDay     *int     `json:"monthly_refill_day"`
}

type quotaPoolAmountRequest struct {
	Amount float64 `json:"amount"`
}

type quotaPoolMemberRequest struct {
	UserId int `json:"user_id"`
}

type quotaPoolMoveRequest struct {
	PoolId int `json:"pool_id"`
}

type quotaPoolAdminRequest struct {
	UserId int `json:"user_id"`
}

func requireQuotaPoolFeature(c *gin.Context) bool {
	if common.QuotaPoolEnabled {
		return true
	}
	writeQuotaPoolError(c, model.ErrQuotaPoolFeatureDisabled)
	return false
}

func parseQuotaPoolID(c *gin.Context) (int, bool) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id < 0 {
		writeQuotaPoolError(c, model.ErrQuotaPoolNotFound)
		return 0, false
	}
	return id, true
}

func parseQuotaPoolUserID(c *gin.Context) (int, bool) {
	id, err := strconv.Atoi(c.Param("user_id"))
	if err != nil || id <= 0 {
		writeQuotaPoolError(c, model.ErrQuotaPoolMemberMismatch)
		return 0, false
	}
	return id, true
}

func quotaAmountToInternal(amount float64) (int, error) {
	if math.IsNaN(amount) || math.IsInf(amount, 0) || amount < 0 ||
		math.IsNaN(common.QuotaPerUnit) || math.IsInf(common.QuotaPerUnit, 0) || common.QuotaPerUnit <= 0 {
		return 0, model.ErrQuotaPoolInvalidAmount
	}
	quota, err := common.WalletQuotaFromDecimalStrict(decimal.NewFromFloat(amount).Mul(decimal.NewFromFloat(common.QuotaPerUnit)).Truncate(0))
	if err != nil {
		return 0, model.ErrQuotaPoolInvalidAmount
	}
	return quota, nil
}

func currentQuotaPoolCapabilities(c *gin.Context, poolAdmin bool) service.QuotaPoolCapabilities {
	return service.ResolveQuotaPoolCapabilities(c.GetInt("role"), poolAdmin)
}

func selfQuotaPoolCapabilities(c *gin.Context, admin *model.QuotaPoolAdminSummary) service.QuotaPoolCapabilities {
	capabilities := currentQuotaPoolCapabilities(c, admin != nil)
	if admin == nil {
		capabilities.CanView = true
	}
	return capabilities
}

func requireQuotaPoolCapability(c *gin.Context, capabilities service.QuotaPoolCapabilities, predicate func(service.QuotaPoolCapabilities) bool) bool {
	if predicate(capabilities) {
		return true
	}
	writeQuotaPoolError(c, model.ErrQuotaPoolPermissionDenied)
	return false
}

func requireSelfQuotaPoolCapability(c *gin.Context, admin *model.QuotaPoolAdminSummary, requirement func(service.QuotaPoolCapabilities) bool) bool {
	return requireQuotaPoolCapability(c, selfQuotaPoolCapabilities(c, admin), requirement)
}

func requireSelfQuotaPoolEdit(c *gin.Context, admin *model.QuotaPoolAdminSummary) bool {
	return requireSelfQuotaPoolCapability(c, admin, func(capabilities service.QuotaPoolCapabilities) bool {
		return capabilities.CanEdit
	})
}

func requireSelfQuotaPoolMembersManagement(c *gin.Context, admin *model.QuotaPoolAdminSummary) bool {
	return requireSelfQuotaPoolCapability(c, admin, func(capabilities service.QuotaPoolCapabilities) bool {
		return capabilities.CanManageMembers
	})
}

func requireSelfQuotaPoolMemberRemoval(c *gin.Context, admin *model.QuotaPoolAdminSummary) bool {
	return requireSelfQuotaPoolCapability(c, admin, func(capabilities service.QuotaPoolCapabilities) bool {
		return capabilities.CanRemoveMembers
	})
}

func writeQuotaPoolError(c *gin.Context, err error) {
	status, code, message := http.StatusInternalServerError, "QUOTA_POOL_INTERNAL", "Quota pool operation failed"
	var params map[string]any
	switch {
	case errors.Is(err, model.ErrQuotaFrozen):
		status, code, message = http.StatusConflict, "USER_QUOTA_FROZEN", "User quota is frozen"
	case errors.Is(err, model.ErrQuotaPoolFeatureDisabled):
		status, code, message = http.StatusConflict, "QUOTA_POOL_DISABLED", "Quota pools are not enabled"
	case errors.Is(err, model.ErrQuotaPoolNotFound):
		status, code, message = http.StatusNotFound, "QUOTA_POOL_NOT_FOUND", "Quota pool not found"
	case errors.Is(err, model.ErrQuotaPoolPermissionDenied), errors.Is(err, model.ErrQuotaPoolSystemReadonly):
		status, code, message = http.StatusForbidden, "QUOTA_POOL_PERMISSION_DENIED", "Permission denied for quota pool operation"
	case errors.Is(err, model.ErrQuotaPoolDisabled):
		status, code, message = http.StatusConflict, "QUOTA_POOL_DISABLED", "Quota pool is disabled"
	case errors.Is(err, model.ErrQuotaPoolInsufficientQuota):
		status, code, message = http.StatusConflict, "QUOTA_POOL_INSUFFICIENT", "Insufficient quota in the pool or user account"
	case errors.Is(err, model.ErrWalletQuotaLimitExceeded):
		status, code, message = http.StatusConflict, "WALLET_QUOTA_LIMIT_EXCEEDED", "Wallet quota limit exceeded"
	case errors.Is(err, model.ErrTopUpQuotaLimitExceeded):
		status, code, message = http.StatusConflict, "TOP_UP_QUOTA_LIMIT_EXCEEDED", "Top-up quota limit exceeded"
	case errors.Is(err, model.ErrQuotaPoolMemberMismatch):
		status, code, message = http.StatusBadRequest, "QUOTA_POOL_MEMBER_MISMATCH", "User does not belong to this quota pool"
	case errors.Is(err, model.ErrQuotaPoolCandidateInvalid):
		status, code, message = http.StatusBadRequest, "QUOTA_POOL_CANDIDATE_INVALID", "User is not eligible for quota pool membership"
	case errors.Is(err, model.ErrQuotaPoolRefillLimited):
		status, code, message = http.StatusConflict, "QUOTA_POOL_REFILL_LIMITED", "Quota pool temporary refill limit exceeded"
		switch {
		case errors.Is(err, model.ErrQuotaPoolRefillBaseInvalid):
			message = "Quota pool base quota must be greater than zero for temporary refills"
		case errors.Is(err, model.ErrQuotaPoolRefillAmountLimited):
			message = "A temporary refill cannot exceed 50% of the current base quota"
		case errors.Is(err, model.ErrQuotaPoolRefillMonthlyLimited):
			message = "This quota pool has reached its monthly limit of {{limit}} temporary refills"
			params = map[string]any{"limit": model.QuotaPoolMonthlyManualRefillLimit}
		}
	case errors.Is(err, model.ErrQuotaPoolInvalidAmount):
		status, code, message = http.StatusBadRequest, "QUOTA_POOL_INVALID_AMOUNT", "Invalid quota amount"
	case errors.Is(err, model.ErrQuotaPoolStatsTimezoneUnsupported):
		status, code, message = http.StatusInternalServerError, "QUOTA_POOL_STATS_TIMEZONE_UNSUPPORTED", "Quota pool statistics require a whole-hour UTC timezone offset"
	case errors.Is(err, model.ErrQuotaPoolBudgetTagNotFound):
		status, code, message = http.StatusNotFound, "QUOTA_POOL_BUDGET_TAG_NOT_FOUND", "Budget tag not found"
	case errors.Is(err, model.ErrQuotaPoolBudgetTagNameInvalid):
		status, code, message = http.StatusBadRequest, "QUOTA_POOL_BUDGET_TAG_NAME_INVALID", "Invalid budget tag name"
	case errors.Is(err, model.ErrQuotaPoolBudgetTagNameExists):
		status, code, message = http.StatusConflict, "QUOTA_POOL_BUDGET_TAG_NAME_EXISTS", "Budget tag name already exists"
	case errors.Is(err, model.ErrQuotaPoolBudgetTagInUse):
		status, code, message = http.StatusConflict, "QUOTA_POOL_BUDGET_TAG_IN_USE", "Budget tag is still assigned to quota pools"
	case errors.Is(err, model.ErrQuotaPoolBudgetMonthRangeInvalid):
		status, code, message = http.StatusBadRequest, "QUOTA_POOL_BUDGET_MONTH_RANGE_INVALID", "Invalid budget statistics month range"
	case errors.Is(err, model.ErrQuotaPoolBudgetAmountOverflow):
		status, code, message = http.StatusUnprocessableEntity, "QUOTA_POOL_BUDGET_AMOUNT_OVERFLOW", "Budget statistics amount exceeds the supported range"
	}
	translated := common.NewMessage(message, params)
	response := translated.Fields()
	response["success"], response["code"], response["message"] = false, code, translated.Error()
	c.AbortWithStatusJSON(status, response)
}

func quotaPoolPage(c *gin.Context, items any, total int64) {
	page := common.GetPageQuery(c)
	page.SetItems(items)
	page.SetTotal(int(total))
	common.ApiSuccess(c, page)
}

func quotaPoolSuccessWithMessage(c *gin.Context, message string, data any) {
	response := common.NewMessage(message).Fields()
	response["success"], response["message"], response["data"] = true, message, data
	c.JSON(http.StatusOK, response)
}

func recordQuotaPoolAudit(c *gin.Context, poolId int, action string, params map[string]any) {
	if params == nil {
		params = map[string]any{}
	}
	params["quota_pool_id"] = poolId
	enrichQuotaPoolAuditParams(poolId, params)
	logType := model.LogTypeManage
	if action == "quota_pool.member_recharge" || action == "quota_pool.member_reclaim" {
		logType = model.LogTypeTopup
	}
	model.RecordOperationAuditLogWithType(
		logType, c.GetInt("id"), c.GetInt("role"), auditContentEN(action, params), c.ClientIP(), action, params,
		auditOperatorInfo(c), nil, c,
	)
	markAuditLogged(c)
}

func enrichQuotaPoolAuditParams(poolId int, params map[string]any) {
	if userId := auditIntParam(params["user_id"]); userId > 0 {
		if user, err := model.GetUserById(userId, false); err == nil {
			params["user_name"] = quotaPoolAuditUserName(user)
		}
	}
	if poolId > 0 {
		if name, err := model.GetQuotaPoolAuditName(poolId); err == nil {
			params["quota_pool_name"] = name
		}
	}
	targetPoolId := auditIntParam(params["target_pool_id"])
	if targetPoolId > 0 && targetPoolId != poolId {
		if name, err := model.GetQuotaPoolAuditName(targetPoolId); err == nil {
			params["target_pool_name"] = name
		}
	}
}

func auditIntParam(value any) int {
	switch typed := value.(type) {
	case int:
		return typed
	case int64:
		return int(typed)
	case float64:
		return int(typed)
	default:
		return 0
	}
}

func quotaPoolAuditUserName(user *model.User) string {
	if name := strings.TrimSpace(user.DisplayName); name != "" {
		return name
	}
	return user.Username
}
