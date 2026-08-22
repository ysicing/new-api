package controller

import (
	"net/http"
	"net/http/httptest"
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
