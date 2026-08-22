package controller

import (
	"errors"
	"net/http"
	"strconv"
	"time"

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

func quotaPoolStatsRange(c *gin.Context, now time.Time) (int64, int64) {
	start, _ := strconv.ParseInt(c.Query("start_timestamp"), 10, 64)
	end, _ := strconv.ParseInt(c.Query("end_timestamp"), 10, 64)
	if start > 0 || end > 0 {
		return start, end
	}
	if c.Query("period") == "month" {
		return now.AddDate(0, -1, 0).Unix(), now.Unix()
	}
	weekday := int(now.Weekday())
	if weekday == 0 {
		weekday = 7
	}
	weekStart := time.Date(now.Year(), now.Month(), now.Day()-weekday+1, 0, 0, 0, 0, now.Location())
	return weekStart.Unix(), now.Unix()
}

func currentQuotaPoolCapabilities(c *gin.Context, level int) service.QuotaPoolCapabilities {
	return service.ResolveQuotaPoolCapabilities(c.GetInt("role"), level)
}

func selfQuotaPoolCapabilities(c *gin.Context, admin *model.QuotaPoolAdminSummary) service.QuotaPoolCapabilities {
	level := 0
	if admin != nil {
		level = admin.Level
	}
	capabilities := currentQuotaPoolCapabilities(c, level)
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

func requireSelfQuotaPoolV1AdminManagement(c *gin.Context, admin *model.QuotaPoolAdminSummary) bool {
	return requireSelfQuotaPoolCapability(c, admin, func(capabilities service.QuotaPoolCapabilities) bool {
		return capabilities.CanManageV1Admins
	})
}

func requireSelfQuotaPoolV2AdminManagement(c *gin.Context, admin *model.QuotaPoolAdminSummary) bool {
	return requireSelfQuotaPoolCapability(c, admin, func(capabilities service.QuotaPoolCapabilities) bool {
		return capabilities.CanManageV2Admins
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
	model.RecordOperationAuditLog(
		c.GetInt("id"), action, c.ClientIP(), action, params,
		map[string]any{"admin_id": c.GetInt("id"), "admin_username": c.GetString("username"), "quota_pool_id": poolId}, nil,
	)
}
