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
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
			"success": false, "code": "RECHARGE_RECORDS_QUERY_FAILED", "message": "充值记录查询失败",
		})
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
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
			"success": false, "code": "RECHARGE_IDENTIFIER_REQUIRED", "message": "请输入用户 ID、用户名或邮箱",
		})
		return
	}
	identifier := strings.TrimSpace(request.Identifier)
	if identifier == "" {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
			"success": false, "code": "RECHARGE_IDENTIFIER_REQUIRED", "message": "请输入用户 ID、用户名或邮箱",
		})
		return
	}
	result, err := service.GetAutoRechargeEligibility(identifier, time.Now())
	if err != nil {
		status, code, message := http.StatusInternalServerError, "RECHARGE_QUERY_INTERNAL", "充值资格查询失败"
		switch {
		case errors.Is(err, gorm.ErrRecordNotFound):
			status, code, message = http.StatusNotFound, "RECHARGE_USER_NOT_FOUND", "未找到匹配用户"
		case errors.Is(err, model.ErrQuotaPoolRechargeUserAmbiguous):
			status, code, message = http.StatusConflict, "RECHARGE_USER_AMBIGUOUS", "输入匹配多个用户，请使用用户 ID 查询"
		}
		c.AbortWithStatusJSON(status, gin.H{"success": false, "code": code, "message": message})
		return
	}
	common.ApiSuccess(c, result)
}
