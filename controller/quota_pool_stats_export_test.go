package controller

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWriteQuotaPoolStatsExportRejectsUnsupportedFormat(t *testing.T) {
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/quota_pool/1/stats/export?format=csv", nil)

	writeQuotaPoolStatsExport(c, &model.QuotaPool{Id: 1, Name: "测试池"})

	assert.Equal(t, http.StatusBadRequest, recorder.Code)
	assert.Contains(t, recorder.Body.String(), "QUOTA_POOL_STATS_EXPORT_FORMAT_INVALID")
}

func TestExportSelfQuotaPoolStatsRequiresPoolManager(t *testing.T) {
	db := setupModelListControllerTestDB(t)
	require.NoError(t, db.AutoMigrate(
		&model.QuotaData{}, &model.QuotaPool{}, &model.QuotaPoolAdmin{}, &model.QuotaPoolTransaction{},
	))
	pool := model.QuotaPool{Name: "自助导出池", PoolType: model.QuotaPoolTypeNormal, Enabled: true}
	require.NoError(t, db.Create(&pool).Error)
	user := model.User{
		Username: "pool-export-user", Password: "password", AffCode: "pool-export-user",
		Role: common.RoleCommonUser, Status: common.UserStatusEnabled, QuotaPoolId: pool.Id,
	}
	require.NoError(t, db.Create(&user).Error)
	previous := common.QuotaPoolEnabled
	common.QuotaPoolEnabled = true
	t.Cleanup(func() { common.QuotaPoolEnabled = previous })

	request := func() *httptest.ResponseRecorder {
		recorder := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(recorder)
		c.Request = httptest.NewRequest(http.MethodGet, "/api/quota_pool/self/stats/export?format=markdown", nil)
		c.Set("id", user.Id)
		c.Set("role", common.RoleCommonUser)
		ExportSelfQuotaPoolStats(c)
		return recorder
	}

	denied := request()
	assert.Equal(t, http.StatusForbidden, denied.Code)
	require.NoError(t, db.Create(&model.QuotaPoolAdmin{PoolId: pool.Id, UserId: user.Id, Level: model.QuotaPoolAdminLevel}).Error)
	allowed := request()
	assert.Equal(t, http.StatusOK, allowed.Code)
	assert.Equal(t, "text/markdown; charset=utf-8", allowed.Header().Get("Content-Type"))
}

func TestWriteQuotaPoolStatsExportDownloadsMarkdown(t *testing.T) {
	db := setupModelListControllerTestDB(t)
	require.NoError(t, db.AutoMigrate(&model.QuotaData{}, &model.QuotaPool{}, &model.QuotaPoolTransaction{}))
	pool := model.QuotaPool{Name: "导出池", PoolType: model.QuotaPoolTypeNormal, Enabled: true}
	require.NoError(t, db.Create(&pool).Error)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/quota_pool/1/stats/export?format=markdown", nil)

	writeQuotaPoolStatsExport(c, &pool)

	assert.Equal(t, http.StatusOK, recorder.Code)
	assert.Equal(t, "text/markdown; charset=utf-8", recorder.Header().Get("Content-Type"))
	assert.Contains(t, recorder.Header().Get("Content-Disposition"), "attachment;")
	assert.Contains(t, recorder.Body.String(), "# 导出池额度池统计")
}
