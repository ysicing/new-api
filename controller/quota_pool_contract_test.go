package controller

import (
	"fmt"
	"math"
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

func TestSystemAdminCanEditRechargePolicyButNotPoolFunding(t *testing.T) {
	weekly := 2
	updates, err := buildQuotaPoolUpdates(quotaPoolUpdateRequest{WeeklyLimit: &weekly}, common.RoleAdminUser)
	require.NoError(t, err)
	assert.Equal(t, 2, updates["weekly_limit"])
	base := float64(100)
	_, err = buildQuotaPoolUpdates(quotaPoolUpdateRequest{BaseQuota: &base}, common.RoleAdminUser)
	assert.ErrorIs(t, err, model.ErrQuotaPoolPermissionDenied)
}

func TestSystemAdminUpdatesLegacyPoolPolicyAndCanReadAudit(t *testing.T) {
	db := setupModelListControllerTestDB(t)
	require.NoError(t, db.AutoMigrate(&model.Log{}, &model.AuditLog{}, &model.QuotaPool{}, &model.QuotaPoolAdmin{}, &model.QuotaPoolTransaction{}))
	pool := model.QuotaPool{Name: model.QuotaPoolDefaultName, PoolType: model.QuotaPoolTypeNormal, LegacyDefault: true, Enabled: true, BaseQuota: 0, Quota: 0}
	require.NoError(t, db.Create(&pool).Error)
	previous := common.QuotaPoolEnabled
	common.QuotaPoolEnabled = true
	t.Cleanup(func() { common.QuotaPoolEnabled = previous })
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPut, "/api/quota_pool/1", strings.NewReader(`{"weekly_limit":2}`))
	c.Params = gin.Params{{Key: "id", Value: strconv.Itoa(pool.Id)}}
	c.Set("id", 8)
	c.Set("role", common.RoleAdminUser)
	UpdateQuotaPool(c)
	assert.Equal(t, http.StatusOK, recorder.Code)
	assert.Contains(t, recorder.Body.String(), `"success":true`)
	require.NoError(t, db.First(&pool, pool.Id).Error)
	assert.Equal(t, 2, pool.WeeklyLimit)
	logs, total, err := model.ListQuotaPoolOperationLogs(pool.Id, &common.PageInfo{Page: 1, PageSize: 10}, "quota_pool.update")
	require.NoError(t, err)
	assert.EqualValues(t, 1, total)
	require.Len(t, logs, 1)
	other := lastQuotaPoolAudit(t, db)
	assert.Equal(t, "quota_pool.update", other.Op.Action)

	recorder = httptest.NewRecorder()
	c, _ = gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/quota_pool/1", nil)
	c.Params = gin.Params{{Key: "id", Value: strconv.Itoa(pool.Id)}}
	c.Set("role", common.RoleAdminUser)
	GetQuotaPool(c)
	assert.Contains(t, recorder.Body.String(), `"can_edit":true`)
	assert.Contains(t, recorder.Body.String(), `"can_edit_monthly_refill":false`)
}

func TestWriteQuotaPoolErrorReturnsStableCode(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)

	writeQuotaPoolError(c, model.ErrQuotaPoolInsufficientQuota)

	assert.Equal(t, http.StatusConflict, recorder.Code)
	assert.Contains(t, recorder.Body.String(), `"code":"QUOTA_POOL_INSUFFICIENT"`)
}

func TestDeductQuotaPoolReducesBalanceAndRecordsAudit(t *testing.T) {
	db := setupModelListControllerTestDB(t)
	require.NoError(t, db.AutoMigrate(&model.Log{}, &model.AuditLog{}, &model.QuotaPool{}, &model.QuotaPoolTransaction{}))
	pool := model.QuotaPool{
		Name: "deduct-handler-pool", PoolType: model.QuotaPoolTypeNormal, Enabled: true,
		BaseQuota: common.QuotaFromFloat(10 * common.QuotaPerUnit), Quota: common.QuotaFromFloat(10 * common.QuotaPerUnit),
	}
	require.NoError(t, db.Create(&pool).Error)
	previous := common.QuotaPoolEnabled
	common.QuotaPoolEnabled = true
	t.Cleanup(func() { common.QuotaPoolEnabled = previous })
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/quota_pool/1/deduct", strings.NewReader(`{"amount":2}`))
	c.Params = gin.Params{{Key: "id", Value: strconv.Itoa(pool.Id)}}
	c.Set("id", 8)
	c.Set("username", "system-admin")
	c.Set("role", common.RoleAdminUser)

	DeductQuotaPool(c)

	assert.Equal(t, http.StatusOK, recorder.Code)
	require.NoError(t, db.First(&pool, pool.Id).Error)
	assert.Equal(t, common.QuotaFromFloat(8*common.QuotaPerUnit), pool.Quota)
	assert.Equal(t, common.QuotaFromFloat(10*common.QuotaPerUnit), pool.BaseQuota)
	other := lastQuotaPoolAudit(t, db)
	assert.Equal(t, "quota_pool.deduct", other.Op.Action)
	assert.EqualValues(t, common.QuotaFromFloat(2*common.QuotaPerUnit), other.Op.Params["amount"])
}

func TestQuotaPoolFundingRequiresRefillCapability(t *testing.T) {
	db := setupModelListControllerTestDB(t)
	require.NoError(t, db.AutoMigrate(&model.Log{}, &model.QuotaPool{}, &model.QuotaPoolTransaction{}))
	pool := model.QuotaPool{
		Name: "deduct-permission-pool", PoolType: model.QuotaPoolTypeNormal, Enabled: true,
		BaseQuota: 1000, Quota: 1000,
	}
	require.NoError(t, db.Create(&pool).Error)
	previous := common.QuotaPoolEnabled
	common.QuotaPoolEnabled = true
	t.Cleanup(func() { common.QuotaPoolEnabled = previous })
	for _, handler := range []struct {
		name string
		run  gin.HandlerFunc
	}{
		{"deduct", DeductQuotaPool},
		{"refill", RefillQuotaPool},
	} {
		t.Run(handler.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(recorder)
			c.Request = httptest.NewRequest(http.MethodPost, "/api/quota_pool/1/"+handler.name, strings.NewReader(`{"amount":1}`))
			c.Params = gin.Params{{Key: "id", Value: strconv.Itoa(pool.Id)}}
			c.Set("id", 8)
			c.Set("role", common.RoleQuotaPoolSuperAdmin)

			handler.run(c)

			assert.Equal(t, http.StatusForbidden, recorder.Code)
			assert.Contains(t, recorder.Body.String(), `"code":"QUOTA_POOL_PERMISSION_DENIED"`)
			require.NoError(t, db.First(&pool, pool.Id).Error)
			assert.Equal(t, 1000, pool.Quota)
		})
	}
}

func TestQuotaAmountToInternalRejectsInvalidAmountsAndScale(t *testing.T) {
	previous := common.QuotaPerUnit
	t.Cleanup(func() { common.QuotaPerUnit = previous })
	for _, test := range []struct {
		name   string
		amount float64
		scale  float64
		want   int
		valid  bool
	}{
		{"zero", 0, 100, 0, true},
		{"fraction_truncated", 1.234, 100, 123, true},
		{"maximum_wallet_quota", float64(common.MaxWalletQuota), 1, common.MaxWalletQuota, true},
		{"above_wallet_quota", float64(common.MaxWalletQuota) + 1, 1, 0, false},
		{"negative", -1, 100, 0, false},
		{"nan", math.NaN(), 100, 0, false},
		{"positive_infinity", math.Inf(1), 100, 0, false},
		{"negative_infinity", math.Inf(-1), 100, 0, false},
		{"overflow", math.MaxFloat64, 100, 0, false},
		{"invalid_scale", 1, 0, 0, false},
		{"negative_scale", 1, -1, 0, false},
		{"nan_scale", 1, math.NaN(), 0, false},
		{"infinite_scale", 1, math.Inf(1), 0, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			common.QuotaPerUnit = test.scale
			quota, err := quotaAmountToInternal(test.amount)
			if test.valid {
				require.NoError(t, err)
				assert.Equal(t, test.want, quota)
			} else {
				assert.ErrorIs(t, err, model.ErrQuotaPoolInvalidAmount)
				assert.Zero(t, quota)
			}
		})
	}
}

func TestWriteQuotaPoolErrorReturnsCandidateValidationCode(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)

	writeQuotaPoolError(c, model.ErrQuotaPoolCandidateInvalid)

	assert.Equal(t, http.StatusBadRequest, recorder.Code)
	assert.Contains(t, recorder.Body.String(), `"code":"QUOTA_POOL_CANDIDATE_INVALID"`)
}

func TestWriteQuotaPoolErrorReturnsStatisticsTimezoneCode(t *testing.T) {
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)

	writeQuotaPoolError(c, model.ErrQuotaPoolStatsTimezoneUnsupported)

	assert.Equal(t, http.StatusInternalServerError, recorder.Code)
	assert.Contains(t, recorder.Body.String(), `"code":"QUOTA_POOL_STATS_TIMEZONE_UNSUPPORTED"`)
}

func TestSelfQuotaPoolManagementEndpointsRequirePoolManager(t *testing.T) {
	db := setupModelListControllerTestDB(t)
	require.NoError(t, db.AutoMigrate(
		&model.Log{},
		&model.AuditLog{},
		&model.QuotaData{},
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
		PoolId: pool.Id, UserId: user.Id, Level: model.QuotaPoolAdminLevel,
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

func TestBuildSelfUserDataOmitsPoolAdministratorLevel(t *testing.T) {
	db := setupModelListControllerTestDB(t)
	require.NoError(t, db.AutoMigrate(&model.QuotaPool{}, &model.QuotaPoolAdmin{}))
	pool := model.QuotaPool{Name: "self-data-admin-pool", PoolType: model.QuotaPoolTypeNormal, Enabled: true}
	require.NoError(t, db.Create(&pool).Error)
	user := model.User{Username: "self-data-admin", Password: "password", AffCode: "self-data-admin", Role: common.RoleCommonUser, Status: common.UserStatusEnabled, QuotaPoolId: pool.Id}
	require.NoError(t, db.Create(&user).Error)
	require.NoError(t, db.Create(&model.QuotaPoolAdmin{PoolId: pool.Id, UserId: user.Id, Level: 2}).Error)

	data := buildSelfUserData(&user)
	encoded, err := common.Marshal(data["quota_pool_admin"])
	require.NoError(t, err)

	assert.JSONEq(t, fmt.Sprintf(`{"pool_id":%d}`, pool.Id), string(encoded))
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

func TestResolveQuotaPoolRechargeAmountUsesDefaultAndValidatesCustomRange(t *testing.T) {
	minimum := common.QuotaRound(10 * common.QuotaPerUnit)
	defaultAmount := common.QuotaRound(50 * common.QuotaPerUnit)
	pool := &model.QuotaPool{
		PoolType: model.QuotaPoolTypeNormal,
		Quota:    defaultAmount * 2, AutoRechargeAmount: defaultAmount,
	}

	amount, err := resolveQuotaPoolRechargeAmount(pool, nil)
	require.NoError(t, err)
	assert.Equal(t, defaultAmount, amount)

	amount, err = resolveQuotaPoolRechargeAmount(pool, &minimum)
	require.NoError(t, err)
	assert.Equal(t, minimum, amount)

	belowMinimum := minimum - 1
	_, err = resolveQuotaPoolRechargeAmount(pool, &belowMinimum)
	assert.ErrorIs(t, err, model.ErrQuotaPoolInvalidAmount)

	aboveDefault := defaultAmount + 1
	_, err = resolveQuotaPoolRechargeAmount(pool, &aboveDefault)
	assert.ErrorIs(t, err, model.ErrQuotaPoolInvalidAmount)

	pool.Quota = minimum
	aboveAvailable := minimum + 1
	_, err = resolveQuotaPoolRechargeAmount(pool, &aboveAvailable)
	assert.ErrorIs(t, err, model.ErrQuotaPoolInsufficientQuota)
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

func TestWriteQuotaPoolRefillErrorExplainsSpecificLimit(t *testing.T) {
	for _, test := range []struct {
		name    string
		err     error
		message string
		key     string
	}{
		{"base", model.ErrQuotaPoolRefillBaseInvalid, "Quota pool base quota must be greater than zero for temporary refills", "Quota pool base quota must be greater than zero for temporary refills"},
		{"amount", model.ErrQuotaPoolRefillAmountLimited, "A temporary refill cannot exceed 50% of the current base quota", "A temporary refill cannot exceed 50% of the current base quota"},
		{"monthly", model.ErrQuotaPoolRefillMonthlyLimited, fmt.Sprintf("This quota pool has reached its monthly limit of %d temporary refills", model.QuotaPoolMonthlyManualRefillLimit), "This quota pool has reached its monthly limit of {{limit}} temporary refills"},
	} {
		t.Run(test.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(recorder)
			writeQuotaPoolError(c, test.err)
			assert.Equal(t, http.StatusConflict, recorder.Code)
			var response map[string]any
			require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
			expected := map[string]any{"success": false, "code": "QUOTA_POOL_REFILL_LIMITED", "message": test.message, "message_key": test.key}
			if test.name == "monthly" {
				expected["message_params"] = map[string]any{"limit": float64(model.QuotaPoolMonthlyManualRefillLimit)}
			}
			assert.Equal(t, expected, response)
		})
	}
}

func TestWriteQuotaPoolErrorExplainsFrozenUser(t *testing.T) {
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	writeQuotaPoolError(c, model.ErrQuotaFrozen)
	assert.Equal(t, http.StatusConflict, recorder.Code)
	var response map[string]any
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	assert.Equal(t, "USER_QUOTA_FROZEN", response["code"])
	assert.Equal(t, "User quota is frozen", response["message"])
	assert.Equal(t, "User quota is frozen", response["message_key"])
}

func TestSubscriptionBalancePayRejectsFrozenUserWithStableMessage(t *testing.T) {
	db := setupModelListControllerTestDB(t)
	confirmPaymentComplianceForTest(t)
	require.NoError(t, db.AutoMigrate(&model.SubscriptionPlan{}, &model.SubscriptionOrder{}, &model.UserSubscription{}))
	user := model.User{
		Username: "frozen-balance-pay", Password: "password", AffCode: "frozen-balance-pay",
		Role: common.RoleCommonUser, Status: common.UserStatusEnabled, Quota: 500, QuotaFrozen: true,
	}
	require.NoError(t, db.Create(&user).Error)
	plan := model.SubscriptionPlan{
		Title: "Balance purchase", Enabled: true, PriceAmount: 1,
		DurationUnit: model.SubscriptionDurationMonth, DurationValue: 1,
		AllowBalancePay: common.GetPointer(true),
	}
	require.NoError(t, db.Create(&plan).Error)
	model.InvalidateSubscriptionPlanCache(plan.Id)
	t.Cleanup(func() { model.InvalidateSubscriptionPlanCache(plan.Id) })
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Set("id", user.Id)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/subscription/balance/pay", strings.NewReader(fmt.Sprintf(`{"plan_id":%d}`, plan.Id)))
	c.Request.Header.Set("Content-Type", "application/json")

	SubscriptionRequestBalancePay(c)

	assert.Equal(t, http.StatusConflict, recorder.Code)
	var response map[string]any
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	assert.Equal(t, map[string]any{
		"success": false, "code": "USER_QUOTA_FROZEN",
		"message": "User quota is frozen", "message_key": "User quota is frozen",
	}, response)
}
