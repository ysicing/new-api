package controller

import (
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
)

func ExportQuotaPoolStats(c *gin.Context) {
	id, ok := parseQuotaPoolID(c)
	if !ok || !requireQuotaPoolFeature(c) {
		return
	}
	pool, err := model.GetQuotaPoolById(id)
	if err != nil {
		writeQuotaPoolError(c, err)
		return
	}
	writeQuotaPoolStatsExport(c, pool)
}

func ExportSelfQuotaPoolStats(c *gin.Context) {
	pool, admin, ok := selfQuotaPool(c)
	if !ok {
		return
	}
	if !requireSelfQuotaPoolMembersManagement(c, admin) {
		return
	}
	writeQuotaPoolStatsExport(c, pool)
}

func writeQuotaPoolStatsExport(c *gin.Context, pool *model.QuotaPool) {
	format := strings.ToLower(strings.TrimSpace(c.Query("format")))
	if format != "markdown" && format != "xlsx" {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
			"success": false, "code": "QUOTA_POOL_STATS_EXPORT_FORMAT_INVALID",
			"message": "导出格式仅支持 markdown 或 xlsx",
		})
		return
	}
	stats, ok := loadQuotaPoolStats(c, pool.Id, time.Now())
	if !ok {
		return
	}
	data, filename, contentType, err := service.ExportQuotaPoolStats(format, pool.Name, stats)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
			"success": false, "code": "QUOTA_POOL_STATS_EXPORT_FAILED", "message": "额度池统计导出失败",
		})
		return
	}
	c.Header("Content-Disposition", "attachment; filename*=UTF-8''"+url.PathEscape(filename))
	c.Data(http.StatusOK, contentType, data)
}
