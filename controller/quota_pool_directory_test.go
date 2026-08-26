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

type selfQuotaPoolDirectoryResponse struct {
	Success bool `json:"success"`
	Data    struct {
		AvailablePools []struct {
			Id            int                           `json:"id"`
			Name          string                        `json:"name"`
			AdminContacts []model.QuotaPoolAdminContact `json:"admin_contacts"`
		} `json:"available_pools"`
	} `json:"data"`
}

func TestGetSelfQuotaPoolLimitsAvailableDirectoryToNewUserMembers(t *testing.T) {
	db := setupModelListControllerTestDB(t)
	require.NoError(t, db.AutoMigrate(
		&model.Log{},
		&model.QuotaPool{},
		&model.QuotaPoolAdmin{},
		&model.QuotaPoolTransaction{},
	))
	newUserPool := model.QuotaPool{
		Name: "新用户额度池", PoolType: model.QuotaPoolTypeNewUser,
		Enabled: true, BaseQuota: 100, Quota: 100,
	}
	enabledPool := model.QuotaPool{
		Name: "研发部额度池", PoolType: model.QuotaPoolTypeNormal,
		Enabled: true, BaseQuota: 100, Quota: 100,
	}
	disabledPool := model.QuotaPool{
		Name: "停用额度池", PoolType: model.QuotaPoolTypeNormal,
		Enabled: true, BaseQuota: 100, Quota: 100,
	}
	defaultPool := model.QuotaPool{
		Name: "默认额度池", PoolType: model.QuotaPoolTypeDefault,
		Enabled: true, IsDefault: true, BaseQuota: -1, Quota: -1,
	}
	for _, pool := range []*model.QuotaPool{
		&newUserPool, &enabledPool, &disabledPool, &defaultPool,
	} {
		require.NoError(t, db.Create(pool).Error)
	}
	require.NoError(t, db.Model(&disabledPool).Update("enabled", false).Error)

	secretToken := "must-not-leak"
	poolAdmin := model.User{
		Username: "rd-owner", DisplayName: "研发负责人", Email: "rd@example.com",
		Password: "password", AffCode: "rd-owner-aff", Role: common.RoleCommonUser,
		Status: common.UserStatusEnabled, QuotaPoolId: enabledPool.Id,
		AccessToken: &secretToken,
	}
	newUser := model.User{
		Username: "new-member", Password: "password", AffCode: "new-member-aff",
		Role: common.RoleCommonUser, Status: common.UserStatusEnabled, QuotaPoolId: newUserPool.Id,
	}
	normalUser := model.User{
		Username: "normal-member", Password: "password", AffCode: "normal-member-aff",
		Role: common.RoleCommonUser, Status: common.UserStatusEnabled, QuotaPoolId: enabledPool.Id,
	}
	for _, user := range []*model.User{&poolAdmin, &newUser, &normalUser} {
		require.NoError(t, db.Create(user).Error)
	}
	require.NoError(t, db.Create(&model.QuotaPoolAdmin{
		PoolId: enabledPool.Id, UserId: poolAdmin.Id, Level: model.QuotaPoolAdminLevel,
	}).Error)

	previous := common.QuotaPoolEnabled
	common.QuotaPoolEnabled = true
	t.Cleanup(func() { common.QuotaPoolEnabled = previous })

	requestDirectory := func(userId int) (*httptest.ResponseRecorder, selfQuotaPoolDirectoryResponse) {
		recorder := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(recorder)
		c.Request = httptest.NewRequest(http.MethodGet, "/api/quota_pool/self/", nil)
		c.Set("id", userId)
		c.Set("role", common.RoleCommonUser)

		GetSelfQuotaPool(c)

		var response selfQuotaPoolDirectoryResponse
		require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
		return recorder, response
	}

	newUserRecorder, newUserResponse := requestDirectory(newUser.Id)
	assert.Equal(t, http.StatusOK, newUserRecorder.Code)
	require.Len(t, newUserResponse.Data.AvailablePools, 1)
	assert.Equal(t, enabledPool.Id, newUserResponse.Data.AvailablePools[0].Id)
	assert.Equal(t, "研发部额度池", newUserResponse.Data.AvailablePools[0].Name)
	require.Len(t, newUserResponse.Data.AvailablePools[0].AdminContacts, 1)
	assert.Equal(t, "研发负责人", newUserResponse.Data.AvailablePools[0].AdminContacts[0].DisplayName)
	assert.Equal(t, "rd@example.com", newUserResponse.Data.AvailablePools[0].AdminContacts[0].Email)
	assert.NotContains(t, newUserRecorder.Body.String(), "must-not-leak")
	assert.NotContains(t, newUserRecorder.Body.String(), "停用额度池")
	assert.NotContains(t, newUserRecorder.Body.String(), "默认额度池")

	normalUserRecorder, normalUserResponse := requestDirectory(normalUser.Id)
	assert.Equal(t, http.StatusOK, normalUserRecorder.Code)
	assert.Empty(t, normalUserResponse.Data.AvailablePools)
}
