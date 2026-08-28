package controller

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
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

func quotaAmountToInternal(amount float64) int {
	return int(amount * common.QuotaPerUnit)
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
	status, code, message := http.StatusInternalServerError, "QUOTA_POOL_INTERNAL", "额度池操作失败"
	switch {
	case errors.Is(err, model.ErrQuotaPoolFeatureDisabled):
		status, code, message = http.StatusConflict, "QUOTA_POOL_DISABLED", "额度池功能未启用"
	case errors.Is(err, model.ErrQuotaPoolNotFound):
		status, code, message = http.StatusNotFound, "QUOTA_POOL_NOT_FOUND", "额度池不存在"
	case errors.Is(err, model.ErrQuotaPoolPermissionDenied), errors.Is(err, model.ErrQuotaPoolSystemReadonly):
		status, code, message = http.StatusForbidden, "QUOTA_POOL_PERMISSION_DENIED", "无额度池操作权限"
	case errors.Is(err, model.ErrQuotaPoolDisabled):
		status, code, message = http.StatusConflict, "QUOTA_POOL_DISABLED", "额度池已禁用"
	case errors.Is(err, model.ErrQuotaPoolInsufficientQuota):
		status, code, message = http.StatusConflict, "QUOTA_POOL_INSUFFICIENT", "额度池或用户额度不足"
	case errors.Is(err, model.ErrQuotaPoolMemberMismatch):
		status, code, message = http.StatusBadRequest, "QUOTA_POOL_MEMBER_MISMATCH", "用户不属于该额度池"
	case errors.Is(err, model.ErrQuotaPoolCandidateInvalid):
		status, code, message = http.StatusBadRequest, "QUOTA_POOL_CANDIDATE_INVALID", "用户不符合额度池成员条件"
	case errors.Is(err, model.ErrQuotaPoolRefillLimited):
		status, code, message = http.StatusConflict, "QUOTA_POOL_REFILL_LIMITED", "额度池临时充值超出限制"
	case errors.Is(err, model.ErrQuotaPoolInvalidAmount):
		status, code, message = http.StatusBadRequest, "QUOTA_POOL_INVALID_AMOUNT", "额度金额无效"
	}
	c.AbortWithStatusJSON(status, gin.H{"success": false, "code": code, "message": message})
}

func quotaPoolPage(c *gin.Context, items any, total int64) {
	page := common.GetPageQuery(c)
	page.SetItems(items)
	page.SetTotal(int(total))
	common.ApiSuccess(c, page)
}

func quotaPoolSuccessWithMessage(c *gin.Context, message string, data any) {
	c.JSON(http.StatusOK, gin.H{"success": true, "message": message, "data": data})
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
		logType, c.GetInt("id"), auditContentEN(action, params), c.ClientIP(), action, params,
		map[string]any{"admin_id": c.GetInt("id"), "admin_username": c.GetString("username"), "quota_pool_id": poolId}, nil,
	)
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
