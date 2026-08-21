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

func TestBuildQuotaPoolUpdatesPreservesAutoRechargeSentinels(t *testing.T) {
	inherit := float64(-1)
	off := float64(0)

	updates, err := buildQuotaPoolUpdates(quotaPoolUpdateRequest{AutoRechargeAmount: &inherit}, common.RoleQuotaPoolSuperAdmin)
	require.NoError(t, err)
	assert.Equal(t, model.QuotaPoolAutoRechargeInherit, updates["auto_recharge_amount"])
	updates, err = buildQuotaPoolUpdates(quotaPoolUpdateRequest{AutoRechargeAmount: &off}, common.RoleQuotaPoolSuperAdmin)
	require.NoError(t, err)
	assert.Equal(t, model.QuotaPoolAutoRechargeOff, updates["auto_recharge_amount"])
}

func TestWriteQuotaPoolErrorReturnsStableCode(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)

	writeQuotaPoolError(c, model.ErrQuotaPoolInsufficientQuota)

	assert.Equal(t, http.StatusConflict, recorder.Code)
	assert.Contains(t, recorder.Body.String(), `"code":"QUOTA_POOL_INSUFFICIENT"`)
}

func TestValidateQuotaPoolReclaimAmountAllowsOnlyConfiguredFractions(t *testing.T) {
	assert.NoError(t, validateQuotaPoolReclaimAmount(1000, 1000))
	assert.NoError(t, validateQuotaPoolReclaimAmount(1000, 500))
	assert.NoError(t, validateQuotaPoolReclaimAmount(1000, 100))
	assert.ErrorIs(t, validateQuotaPoolReclaimAmount(1000, 333), model.ErrQuotaPoolInvalidAmount)
}
