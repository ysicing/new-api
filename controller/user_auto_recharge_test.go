package controller

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetSelfAutoRechargeEligibilityUsesAuthenticatedUserOnly(t *testing.T) {
	db := setupQuotaPoolRechargeQueryControllerTest(t)
	amount := common.QuotaFromFloat(common.QuotaPerUnit)
	user := model.User{
		Username: "self-eligibility-user", Email: "self-eligibility@example.com",
		Password: "password", AffCode: "self-eligibility-aff",
		Role: common.RoleCommonUser, Status: common.UserStatusEnabled, Quota: amount * 3,
	}
	otherUser := model.User{
		Username: "other-eligibility-user", Email: "other-eligibility@example.com",
		Password: "password", AffCode: "other-eligibility-aff",
		Role: common.RoleCommonUser, Status: common.UserStatusEnabled,
	}
	require.NoError(t, db.Create(&user).Error)
	require.NoError(t, db.Create(&otherUser).Error)

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Set("id", user.Id)
	request := httptest.NewRequest(
		http.MethodGet,
		"/api/user/auto_recharge/eligibility?identifier="+otherUser.Email,
		strings.NewReader(`{"identifier":"other-eligibility@example.com"}`),
	)
	request.Header.Set("Content-Type", "application/json")
	c.Request = request

	GetSelfAutoRechargeEligibility(c)

	var response struct {
		Success bool                                `json:"success"`
		Data    service.SelfAutoRechargeEligibility `json:"data"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	assert.Equal(t, http.StatusOK, recorder.Code)
	assert.True(t, response.Success)
	assert.Equal(t, service.SelfAutoRechargeStatusNotNeeded, response.Data.Status)
	assert.False(t, response.Data.Eligible)
	assert.Equal(t, "quota_above_threshold", response.Data.Reason)
	assert.Equal(t, user.Quota, response.Data.UserQuota)
	assert.NotEqual(t, otherUser.Quota, response.Data.UserQuota)
	assert.NotContains(t, recorder.Body.String(), "user_id")
	assert.NotContains(t, recorder.Body.String(), "username")
	assert.NotContains(t, recorder.Body.String(), "email")
	assert.NotContains(t, recorder.Body.String(), "pool_id")
	assert.NotContains(t, recorder.Body.String(), "pool_quota")
}
