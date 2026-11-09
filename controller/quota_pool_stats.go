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
		message := common.NewMessage("Invalid statistics range or granularity; the maximum range is 366 days")
		response := message.Fields()
		response["success"], response["code"], response["message"] = false, "QUOTA_POOL_STATS_RANGE_INVALID", message.Error()
		c.AbortWithStatusJSON(http.StatusBadRequest, response)
		return nil, false
	}
	stats, _, err := service.GetCachedQuotaPoolStatsInLocation(
		poolId, statsRange.StartTimestamp, statsRange.EndTimestamp,
		model.QuotaPoolStatsGranularity(statsRange.Granularity), now, rangeNow.Location(),
	)
	if err != nil {
		if errors.Is(err, model.ErrQuotaPoolStatsTimezoneUnsupported) {
			common.SysError(fmt.Sprintf("quota pool statistics timezone unsupported: location=%s", rangeNow.Location()))
		}
		writeQuotaPoolError(c, err)
		return nil, false
	}
	stats.Preset = statsRange.Preset
	return stats, true
}

func writeQuotaPoolStats(c *gin.Context, poolId int) {
	stats, ok := loadQuotaPoolStats(c, poolId, time.Now().In(common.BeijingTimeLocation))
	if !ok {
		return
	}
	common.ApiSuccess(c, stats)
}
