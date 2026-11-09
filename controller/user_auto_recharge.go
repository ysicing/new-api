package controller

import (
	"errors"
	"net/http"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// GetSelfAutoRechargeEligibility 返回认证用户的自动充值资格只读快照。
func GetSelfAutoRechargeEligibility(c *gin.Context) {
	eligibility, err := service.GetSelfAutoRechargeEligibility(
		c.GetInt("id"),
		time.Now(),
	)
	if err != nil {
		status, code, message := http.StatusInternalServerError, "SELF_AUTO_RECHARGE_QUERY_FAILED", "Failed to query automatic recharge eligibility"
		if errors.Is(err, gorm.ErrRecordNotFound) {
			status, code, message = http.StatusNotFound, "SELF_AUTO_RECHARGE_USER_NOT_FOUND", "User does not exist"
		}
		common.ApiErrorStatus(c, status, common.NewMessage(message), gin.H{"code": code})
		c.Abort()
		return
	}
	common.ApiSuccess(c, eligibility)
}
