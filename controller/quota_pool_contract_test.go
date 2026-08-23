package controller

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildQuotaPoolUpdatesPreservesAutoRechargeSentinels(t *testing.T) {
	inherit := float64(-1)
	off := float64(0)
	invalid := float64(-2)

	updates, err := buildQuotaPoolUpdates(quotaPoolUpdateRequest{AutoRechargeAmount: &inherit}, common.RoleQuotaPoolSuperAdmin)
	require.NoError(t, err)
	assert.Equal(t, model.QuotaPoolAutoRechargeInherit, updates["auto_recharge_amount"])
	updates, err = buildQuotaPoolUpdates(quotaPoolUpdateRequest{AutoRechargeAmount: &off}, common.RoleQuotaPoolSuperAdmin)
	require.NoError(t, err)
	assert.Equal(t, model.QuotaPoolAutoRechargeOff, updates["auto_recharge_amount"])
	_, err = buildQuotaPoolUpdates(quotaPoolUpdateRequest{AutoRechargeAmount: &invalid}, common.RoleQuotaPoolSuperAdmin)
	assert.ErrorIs(t, err, model.ErrQuotaPoolInvalidAmount)
}

func TestWriteQuotaPoolErrorReturnsStableCode(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)

	writeQuotaPoolError(c, model.ErrQuotaPoolInsufficientQuota)

	assert.Equal(t, http.StatusConflict, recorder.Code)
	assert.Contains(t, recorder.Body.String(), `"code":"QUOTA_POOL_INSUFFICIENT"`)
}

func TestWriteQuotaPoolErrorReturnsCandidateValidationCode(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)

	writeQuotaPoolError(c, model.ErrQuotaPoolCandidateInvalid)

	assert.Equal(t, http.StatusBadRequest, recorder.Code)
	assert.Contains(t, recorder.Body.String(), `"code":"QUOTA_POOL_CANDIDATE_INVALID"`)
}

func TestSelfQuotaPoolManagementEndpointsRequirePoolManager(t *testing.T) {
	db := setupModelListControllerTestDB(t)
	require.NoError(t, db.AutoMigrate(
		&model.Log{},
		&model.QuotaPool{},
		&model.QuotaPoolAdmin{},
		&model.QuotaPoolTransaction{},
	))
	pool := model.QuotaPool{
		Name: "management-endpoint-pool", PoolType: model.QuotaPoolTypeNormal,
		Enabled: true, BaseQuota: 100, Quota: 100,
	}
	require.NoError(t, db.Create(&pool).Error)
	user := model.User{
		Username: "ordinary-pool-member", Password: "password", AffCode: "ordinary-pool-member",
		Role: common.RoleCommonUser, Status: common.UserStatusEnabled, QuotaPoolId: pool.Id,
	}
	require.NoError(t, db.Create(&user).Error)
	previous := common.QuotaPoolEnabled
	common.QuotaPoolEnabled = true
	t.Cleanup(func() { common.QuotaPoolEnabled = previous })

	handlers := []struct {
		name    string
		handler gin.HandlerFunc
	}{
		{name: "members", handler: GetSelfQuotaPoolMembers},
		{name: "transactions", handler: GetSelfQuotaPoolTransactions},
		{name: "operation_logs", handler: GetSelfQuotaPoolOperationLogs},
		{name: "stats", handler: GetSelfQuotaPoolStats},
	}
	for _, item := range handlers {
		t.Run("ordinary_member_"+item.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(recorder)
			c.Request = httptest.NewRequest(http.MethodGet, "/api/quota_pool/self/"+item.name, nil)
			c.Set("id", user.Id)
			c.Set("role", common.RoleCommonUser)

			item.handler(c)

			assert.Equal(t, http.StatusForbidden, recorder.Code)
			assert.Contains(t, recorder.Body.String(), `"code":"QUOTA_POOL_PERMISSION_DENIED"`)
		})
	}

	require.NoError(t, db.Create(&model.QuotaPoolAdmin{
		PoolId: pool.Id, UserId: user.Id, Level: model.QuotaPoolAdminLevelV1,
	}).Error)
	for _, item := range handlers {
		t.Run("pool_manager_"+item.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(recorder)
			c.Request = httptest.NewRequest(http.MethodGet, "/api/quota_pool/self/"+item.name, nil)
			c.Set("id", user.Id)
			c.Set("role", common.RoleCommonUser)

			item.handler(c)

			assert.Equal(t, http.StatusOK, recorder.Code)
		})
	}
}

func TestGetUserIncludesCurrentQuotaPoolFeatureState(t *testing.T) {
	db := setupModelListControllerTestDB(t)
	target := model.User{
		Username: "user-info-pool", Password: "password", AffCode: "user-info-pool",
		Role: common.RoleCommonUser, Status: common.UserStatusEnabled,
	}
	require.NoError(t, db.Create(&target).Error)
	previous := common.QuotaPoolEnabled
	common.QuotaPoolEnabled = true
	t.Cleanup(func() { common.QuotaPoolEnabled = previous })
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Params = gin.Params{{Key: "id", Value: strconv.Itoa(target.Id)}}
	c.Set("role", common.RoleRootUser)

	GetUser(c)

	assert.Equal(t, http.StatusOK, recorder.Code)
	assert.Contains(t, recorder.Body.String(), `"quota_pool_enabled":true`)
}

func TestGetQuotaPoolsReturnsRequestedSearchPage(t *testing.T) {
	db := setupModelListControllerTestDB(t)
	require.NoError(t, db.AutoMigrate(&model.QuotaPool{}, &model.QuotaPoolAdmin{}))
	require.NoError(t, db.Create(&[]model.QuotaPool{
		{Name: "pool-alpha", PoolType: model.QuotaPoolTypeNormal, Enabled: true},
		{Name: "pool-beta", PoolType: model.QuotaPoolTypeNormal, Enabled: true},
	}).Error)
	previous := common.QuotaPoolEnabled
	common.QuotaPoolEnabled = true
	t.Cleanup(func() { common.QuotaPoolEnabled = previous })
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/quota_pool/?p=2&page_size=1&keyword=pool", nil)
	c.Set("role", common.RoleAdminUser)

	GetQuotaPools(c)

	assert.Equal(t, http.StatusOK, recorder.Code)
	assert.Contains(t, recorder.Body.String(), `"total":2`)
	assert.Contains(t, recorder.Body.String(), `"page":2`)
	assert.Contains(t, recorder.Body.String(), `"page_size":1`)
	assert.Contains(t, recorder.Body.String(), `"name":"pool-beta"`)
	assert.NotContains(t, recorder.Body.String(), `"name":"pool-alpha"`)
}

func TestGetQuotaPoolsClampsInvalidPagination(t *testing.T) {
	db := setupModelListControllerTestDB(t)
	require.NoError(t, db.AutoMigrate(&model.QuotaPool{}, &model.QuotaPoolAdmin{}))
	require.NoError(t, db.Create(&model.QuotaPool{
		Name: "pool-alpha", PoolType: model.QuotaPoolTypeNormal, Enabled: true,
	}).Error)
	previous := common.QuotaPoolEnabled
	common.QuotaPoolEnabled = true
	t.Cleanup(func() { common.QuotaPoolEnabled = previous })
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/quota_pool/?p=-2&page_size=-5", nil)
	c.Set("role", common.RoleAdminUser)

	GetQuotaPools(c)

	assert.Equal(t, http.StatusOK, recorder.Code)
	assert.Contains(t, recorder.Body.String(), `"page":1`)
	assert.Contains(t, recorder.Body.String(), fmt.Sprintf(`"page_size":%d`, common.ItemsPerPage))
}

func TestGetQuotaPoolsTreatsAnEmptyKeywordAsPaginatedRequest(t *testing.T) {
	db := setupModelListControllerTestDB(t)
	require.NoError(t, db.AutoMigrate(&model.QuotaPool{}, &model.QuotaPoolAdmin{}))
	require.NoError(t, db.Create(&model.QuotaPool{
		Name: "pool-alpha", PoolType: model.QuotaPoolTypeNormal, Enabled: true,
	}).Error)
	previous := common.QuotaPoolEnabled
	common.QuotaPoolEnabled = true
	t.Cleanup(func() { common.QuotaPoolEnabled = previous })
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/quota_pool/?keyword=", nil)
	c.Set("role", common.RoleAdminUser)

	GetQuotaPools(c)

	assert.Equal(t, http.StatusOK, recorder.Code)
	assert.Contains(t, recorder.Body.String(), `"total":1`)
	assert.Contains(t, recorder.Body.String(), `"page":1`)
	assert.Contains(t, recorder.Body.String(), fmt.Sprintf(`"page_size":%d`, common.ItemsPerPage))
}

func TestGetQuotaPoolsClampsAnOverflowingPageToTheLastPage(t *testing.T) {
	db := setupModelListControllerTestDB(t)
	require.NoError(t, db.AutoMigrate(&model.QuotaPool{}, &model.QuotaPoolAdmin{}))
	pools := make([]model.QuotaPool, 21)
	for index := range pools {
		pools[index] = model.QuotaPool{
			Name: fmt.Sprintf("pool-%02d", index+1), PoolType: model.QuotaPoolTypeNormal, Enabled: true,
		}
	}
	require.NoError(t, db.Create(&pools).Error)
	previous := common.QuotaPoolEnabled
	common.QuotaPoolEnabled = true
	t.Cleanup(func() { common.QuotaPoolEnabled = previous })
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	maxPage := int(^uint(0) >> 1)
	c.Request = httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/quota_pool/?p=%d&page_size=20", maxPage), nil)
	c.Set("role", common.RoleAdminUser)

	GetQuotaPools(c)

	assert.Equal(t, http.StatusOK, recorder.Code)
	assert.Contains(t, recorder.Body.String(), `"page":2`)
	assert.Contains(t, recorder.Body.String(), `"total":21`)
	assert.Contains(t, recorder.Body.String(), `"name":"pool-21"`)
	assert.NotContains(t, recorder.Body.String(), `"name":"pool-01"`)
}

func TestValidateQuotaPoolReclaimAmountAllowsOnlyCurrentMemberOptions(t *testing.T) {
	allowed := []int{500, 400, 300, 200, 100}
	assert.NoError(t, validateQuotaPoolReclaimAmount(allowed, 500))
	assert.ErrorIs(t, validateQuotaPoolReclaimAmount(allowed, 1000), model.ErrQuotaPoolInvalidAmount)
	assert.ErrorIs(t, validateQuotaPoolReclaimAmount(allowed, 333), model.ErrQuotaPoolInvalidAmount)
}

func TestBuildQuotaPoolReclaimAmountsPreservesThreshold(t *testing.T) {
	assert.Equal(t, []int{1000}, buildQuotaPoolReclaimAmounts(1000, 1200, 100))
	assert.Equal(t, []int{500, 400, 300, 200, 100}, buildQuotaPoolReclaimAmounts(1000, 900, 100))
	assert.Empty(t, buildQuotaPoolReclaimAmounts(1000, 100, 100))
}

func TestReclaimQuotaPoolMemberUsesRequestedAllowedAmount(t *testing.T) {
	db := setupModelListControllerTestDB(t)
	require.NoError(t, db.AutoMigrate(&model.QuotaPool{}, &model.QuotaPoolTransaction{}))
	pool := model.QuotaPool{
		Name: "回收池", PoolType: model.QuotaPoolTypeNormal, Enabled: true,
		BaseQuota: 1000, Quota: 1000, AutoRechargeAmount: 1000,
	}
	require.NoError(t, db.Create(&pool).Error)
	user := model.User{
		Username: "reclaim-user", Password: "password", AffCode: "reclaim-contract",
		Role: common.RoleCommonUser, Status: common.UserStatusEnabled, Quota: 900, QuotaPoolId: pool.Id,
	}
	require.NoError(t, db.Create(&user).Error)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/reclaim", strings.NewReader(`{"amount":400}`))
	c.Set("id", 7)

	reclaimQuotaPoolMember(c, &pool, user.Id)

	assert.Equal(t, http.StatusOK, recorder.Code)
	require.NoError(t, db.First(&user, user.Id).Error)
	assert.Equal(t, 500, user.Quota)
	var transaction model.QuotaPoolTransaction
	require.NoError(t, db.Where("user_id = ?", user.Id).First(&transaction).Error)
	assert.Equal(t, 400, transaction.Amount)
}
