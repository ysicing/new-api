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
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.AbortWithStatusJSON(http.StatusNotFound, gin.H{
				"success": false,
				"code":    "SELF_AUTO_RECHARGE_USER_NOT_FOUND",
				"message": "用户不存在",
			})
			return
		}
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"code":    "SELF_AUTO_RECHARGE_QUERY_FAILED",
			"message": "自动充值资格查询失败",
		})
		return
	}
	common.ApiSuccess(c, eligibility)
}
