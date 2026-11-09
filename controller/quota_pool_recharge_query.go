package controller

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type quotaPoolRechargeEligibilityRequest struct {
	Identifier string `json:"identifier"`
}

func GetQuotaPoolRechargeRecords(c *gin.Context) {
	start, end := statsRange(c, time.Now())
	page := common.GetPageQuery(c)
	items, total, err := model.ListQuotaPoolRechargeRecords(start, end, page)
	if err != nil {
		common.ApiErrorStatus(c, http.StatusInternalServerError, common.NewMessage("Failed to query recharge records"), gin.H{"code": "RECHARGE_RECORDS_QUERY_FAILED"})
		c.Abort()
		return
	}
	page.SetItems(items)
	page.SetTotal(int(total))
	common.ApiSuccess(c, page)
}

func GetQuotaPoolRechargeEligibility(c *gin.Context) {
	// 使用 POST body 避免邮箱进入访问日志，但该接口仍是纯只读查询，跳过
	// AdminAuth/RootAuth 对写方法的通用操作审计。
	common.SetContextKey(c, constant.ContextKeyAuditSkip, true)
	var request quotaPoolRechargeEligibilityRequest
	if err := common.DecodeJson(c.Request.Body, &request); err != nil {
		common.ApiErrorStatus(c, http.StatusBadRequest, common.NewMessage("Enter a user ID, username, or email."), gin.H{"code": "RECHARGE_IDENTIFIER_REQUIRED"})
		c.Abort()
		return
	}
	identifier := strings.TrimSpace(request.Identifier)
	if identifier == "" {
		common.ApiErrorStatus(c, http.StatusBadRequest, common.NewMessage("Enter a user ID, username, or email."), gin.H{"code": "RECHARGE_IDENTIFIER_REQUIRED"})
		c.Abort()
		return
	}
	result, err := service.GetAutoRechargeEligibility(identifier, time.Now())
	if err != nil {
		status, code, message := http.StatusInternalServerError, "RECHARGE_QUERY_INTERNAL", "Failed to query recharge eligibility"
		switch {
		case errors.Is(err, gorm.ErrRecordNotFound):
			status, code, message = http.StatusNotFound, "RECHARGE_USER_NOT_FOUND", "No matching user was found."
		case errors.Is(err, model.ErrQuotaPoolRechargeUserAmbiguous):
			status, code, message = http.StatusConflict, "RECHARGE_USER_AMBIGUOUS", "Multiple users matched. Use the user ID to query."
		}
		common.ApiErrorStatus(c, status, common.NewMessage(message), gin.H{"code": code})
		c.Abort()
		return
	}
	common.ApiSuccess(c, result)
}
