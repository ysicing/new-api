package controller

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupQuotaPoolBudgetControllerTest(t *testing.T) {
	t.Helper()
	db := setupModelListControllerTestDB(t)
	require.NoError(t, db.AutoMigrate(
		&model.Log{}, &model.QuotaPool{}, &model.QuotaPoolTransaction{}, &model.QuotaPoolBudgetTag{},
	))
	previousEnabled := common.QuotaPoolEnabled
	common.QuotaPoolEnabled = true
	t.Cleanup(func() { common.QuotaPoolEnabled = previousEnabled })
}

func quotaPoolBudgetContext(method, target, body string) (*gin.Context, *httptest.ResponseRecorder) {
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(method, target, strings.NewReader(body))
	context.Set("id", 1)
	context.Set("role", common.RoleRootUser)
	context.Set("username", "root")
	return context, recorder
}

func TestQuotaPoolBudgetTagHandlersCreateAssignAndList(t *testing.T) {
	setupQuotaPoolBudgetControllerTest(t)
	pool := model.QuotaPool{Name: "研发额度池", PoolType: model.QuotaPoolTypeNormal, Enabled: true}
	require.NoError(t, model.DB.Create(&pool).Error)

	context, recorder := quotaPoolBudgetContext(http.MethodPost, "/api/quota_pool/budget_tags", `{"name":"产研中心"}`)
	CreateQuotaPoolBudgetTag(context)
	assert.Equal(t, http.StatusOK, recorder.Code)
	var tag model.QuotaPoolBudgetTag
	require.NoError(t, model.DB.Where("name = ?", "产研中心").First(&tag).Error)

	context, recorder = quotaPoolBudgetContext(http.MethodPut, "/api/quota_pool/1/budget_tag", `{"tag_id":`+strconv.Itoa(tag.Id)+`}`)
	context.Params = gin.Params{{Key: "id", Value: strconv.Itoa(pool.Id)}}
	SetQuotaPoolBudgetTag(context)
	assert.Equal(t, http.StatusOK, recorder.Code)

	context, recorder = quotaPoolBudgetContext(http.MethodGet, "/api/quota_pool/budget_tags", "")
	GetQuotaPoolBudgetTags(context)
	assert.Equal(t, http.StatusOK, recorder.Code)
	assert.Contains(t, recorder.Body.String(), `"name":"产研中心"`)
	assert.Contains(t, recorder.Body.String(), `"pool_count":1`)

	var auditCount int64
	require.NoError(t, model.DB.Model(&model.Log{}).
		Where("type = ?", model.LogTypeManage).Count(&auditCount).Error)
	assert.EqualValues(t, 2, auditCount)
}

func TestReplaceQuotaPoolBudgetTagPoolsHandlerAssignsSelectedPools(t *testing.T) {
	setupQuotaPoolBudgetControllerTest(t)
	tag, err := model.CreateQuotaPoolBudgetTag("产研中心")
	require.NoError(t, err)
	pools := []model.QuotaPool{
		{Name: "研发一部", PoolType: model.QuotaPoolTypeNormal, Enabled: true},
		{Name: "研发二部", PoolType: model.QuotaPoolTypeNormal, Enabled: true},
	}
	require.NoError(t, model.DB.Create(&pools).Error)
	body := `{"pool_ids":[` + strconv.Itoa(pools[0].Id) + `,` + strconv.Itoa(pools[1].Id) + `]}`
	context, recorder := quotaPoolBudgetContext(http.MethodPut, "/api/quota_pool/budget_tags/1/pools", body)
	context.Params = gin.Params{{Key: "tag_id", Value: strconv.Itoa(tag.Id)}}

	ReplaceQuotaPoolBudgetTagPools(context)

	assert.Equal(t, http.StatusOK, recorder.Code)
	var assigned int64
	require.NoError(t, model.DB.Model(&model.QuotaPool{}).Where("budget_tag_id = ?", tag.Id).Count(&assigned).Error)
	assert.EqualValues(t, 2, assigned)
}

func TestGetQuotaPoolBudgetStatsSupportsInclusiveMonthRange(t *testing.T) {
	setupQuotaPoolBudgetControllerTest(t)
	tag, err := model.CreateQuotaPoolBudgetTag("产研中心")
	require.NoError(t, err)
	pool := model.QuotaPool{Name: "研发额度池", PoolType: model.QuotaPoolTypeNormal, Enabled: true, BudgetTagId: tag.Id}
	require.NoError(t, model.DB.Create(&pool).Error)
	beijing := common.BeijingTimeLocation
	require.NoError(t, model.DB.Create(&[]model.QuotaPoolTransaction{
		{PoolId: pool.Id, Type: model.QuotaPoolTransactionInitialFund, Amount: 1000, CreatedAt: monthTimestamp(beijing, 2026, 1)},
		{PoolId: pool.Id, Type: model.QuotaPoolTransactionAllocateAuto, Amount: -200, CreatedAt: monthTimestamp(beijing, 2026, 2)},
	}).Error)

	context, recorder := quotaPoolBudgetContext(http.MethodGet, "/api/quota_pool/budget_stats?start_month=2026-01&end_month=2026-02", "")
	GetQuotaPoolBudgetStats(context)

	assert.Equal(t, http.StatusOK, recorder.Code)
	assert.Contains(t, recorder.Body.String(), `"start_month":"2026-01"`)
	assert.Contains(t, recorder.Body.String(), `"end_month":"2026-02"`)
	assert.Contains(t, recorder.Body.String(), `"net_recharge":1000`)
	assert.Contains(t, recorder.Body.String(), `"net_consumption":200`)
}

func TestGetQuotaPoolBudgetStatsRejectsMoreThanTwentyFourMonths(t *testing.T) {
	setupQuotaPoolBudgetControllerTest(t)
	context, recorder := quotaPoolBudgetContext(http.MethodGet, "/api/quota_pool/budget_stats?start_month=2024-01&end_month=2026-01", "")

	GetQuotaPoolBudgetStats(context)

	assert.Equal(t, http.StatusBadRequest, recorder.Code)
	assert.Contains(t, recorder.Body.String(), `"code":"QUOTA_POOL_BUDGET_MONTH_RANGE_INVALID"`)
}

func monthTimestamp(location *time.Location, year, month int) int64 {
	return time.Date(year, time.Month(month), 15, 12, 0, 0, 0, location).Unix()
}
