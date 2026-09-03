package controller

import (
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
)

func loadQuotaPoolStats(c *gin.Context, poolId int, now time.Time) (*model.QuotaPoolStats, bool) {
	rangeNow := now
	statsRange, err := parseQuotaPoolStatsRange(c, rangeNow)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
			"success": false, "code": "QUOTA_POOL_STATS_RANGE_INVALID",
			"message": "统计时间范围无效，最多支持 366 个自然日",
		})
		return nil, false
	}
	stats, _, err := service.GetCachedQuotaPoolStatsInLocation(
		poolId, statsRange.StartTimestamp, statsRange.EndTimestamp, now, rangeNow.Location(),
	)
	if err != nil {
		if errors.Is(err, model.ErrQuotaPoolStatsTimezoneUnsupported) {
			common.SysError(fmt.Sprintf("quota pool statistics timezone unsupported: location=%s", rangeNow.Location()))
		}
		writeQuotaPoolError(c, err)
		return nil, false
	}
	stats.RangeType = statsRange.RangeType
	stats.StartDate = statsRange.StartDate
	stats.EndDate = statsRange.EndDate
	return stats, true
}

func writeQuotaPoolStats(c *gin.Context, poolId int) {
	stats, ok := loadQuotaPoolStats(c, poolId, time.Now())
	if !ok {
		return
	}
	common.ApiSuccess(c, stats)
}
