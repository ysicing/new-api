package controller

import (
	"errors"
	"net/http"
	"strconv"

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
	Level  int `json:"level"`
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

func currentQuotaPoolCapabilities(c *gin.Context, level int) service.QuotaPoolCapabilities {
	return service.ResolveQuotaPoolCapabilities(c.GetInt("role"), level)
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

func recordQuotaPoolAudit(c *gin.Context, poolId int, action string, params map[string]any) {
	if params == nil {
		params = map[string]any{}
	}
	params["quota_pool_id"] = poolId
	model.RecordOperationAuditLog(
		c.GetInt("id"), action, c.ClientIP(), action, params,
		map[string]any{"admin_id": c.GetInt("id"), "admin_username": c.GetString("username"), "quota_pool_id": poolId}, nil,
	)
}
