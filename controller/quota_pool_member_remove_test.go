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

func TestPoolAdministratorRemovesOrdinaryMemberToNewUserPool(t *testing.T) {
	db := setupModelListControllerTestDB(t)
	require.NoError(t, db.AutoMigrate(&model.Log{}, &model.QuotaPool{}, &model.QuotaPoolAdmin{}, &model.QuotaPoolTransaction{}))
	source := model.QuotaPool{Name: "self-remove-source", PoolType: model.QuotaPoolTypeNormal, Enabled: true, BaseQuota: 100, Quota: 100}
	target := model.QuotaPool{Name: model.QuotaPoolNewUserName, PoolType: model.QuotaPoolTypeNewUser, Enabled: true, BaseQuota: -1, Quota: -1}
	require.NoError(t, db.Create(&source).Error)
	require.NoError(t, db.Create(&target).Error)
	operator := model.User{Username: "self-remove-admin", Password: "password", AffCode: "self-remove-admin", Role: common.RoleCommonUser, Status: common.UserStatusEnabled, QuotaPoolId: source.Id}
	member := model.User{Username: "self-remove-member", Password: "password", AffCode: "self-remove-member", Role: common.RoleCommonUser, Status: common.UserStatusEnabled, QuotaPoolId: source.Id, Quota: 25}
	require.NoError(t, db.Create(&operator).Error)
	require.NoError(t, db.Create(&member).Error)
	require.NoError(t, db.Create(&model.QuotaPoolAdmin{PoolId: source.Id, UserId: operator.Id, Level: model.QuotaPoolAdminLevel}).Error)
	setQuotaPoolFeatureForTest(t)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodDelete, "/api/quota_pool/self/members/2", nil)
	c.Params = gin.Params{{Key: "user_id", Value: strconv.Itoa(member.Id)}}
	c.Set("id", operator.Id)
	c.Set("role", common.RoleCommonUser)

	RemoveSelfQuotaPoolMember(c)

	assert.Equal(t, http.StatusOK, recorder.Code)
	require.NoError(t, db.First(&source, source.Id).Error)
	require.NoError(t, db.First(&member, member.Id).Error)
	assert.Equal(t, 125, source.Quota)
	assert.Zero(t, member.Quota)
	assert.Equal(t, target.Id, member.QuotaPoolId)
	var audit model.Log
	require.NoError(t, db.Where("type = ?", model.LogTypeManage).Order("id DESC").First(&audit).Error)
	assert.Equal(t, "quota_pool.member_remove", audit.Content)
	assert.Contains(t, audit.Other, `"quota_pool_id":`+strconv.Itoa(source.Id))
	assert.Contains(t, audit.Other, `"target_pool_id":`+strconv.Itoa(target.Id))
	assert.Contains(t, audit.Other, `"user_name":"self-remove-member"`)
	assert.Contains(t, audit.Other, `"quota_pool_name":"self-remove-source"`)
	assert.Contains(t, audit.Other, `"target_pool_name":"`+model.QuotaPoolNewUserName+`"`)
	assert.Contains(t, audit.Other, `"amount":25`)
}

func TestPoolAdministratorCannotRemovePoolAdministrator(t *testing.T) {
	db := setupModelListControllerTestDB(t)
	require.NoError(t, db.AutoMigrate(&model.Log{}, &model.QuotaPool{}, &model.QuotaPoolAdmin{}, &model.QuotaPoolTransaction{}))
	source := model.QuotaPool{Name: "self-remove-admin-source", PoolType: model.QuotaPoolTypeNormal, Enabled: true, BaseQuota: 100, Quota: 100}
	require.NoError(t, db.Create(&source).Error)
	require.NoError(t, db.Create(&model.QuotaPool{Name: model.QuotaPoolNewUserName, PoolType: model.QuotaPoolTypeNewUser, Enabled: true, BaseQuota: -1, Quota: -1}).Error)
	operator := model.User{Username: "remove-admin-operator", Password: "password", AffCode: "remove-admin-operator", Role: common.RoleCommonUser, Status: common.UserStatusEnabled, QuotaPoolId: source.Id}
	target := model.User{Username: "remove-admin-target", Password: "password", AffCode: "remove-admin-target", Role: common.RoleCommonUser, Status: common.UserStatusEnabled, QuotaPoolId: source.Id, Quota: 25}
	require.NoError(t, db.Create(&operator).Error)
	require.NoError(t, db.Create(&target).Error)
	require.NoError(t, db.Create(&[]model.QuotaPoolAdmin{
		{PoolId: source.Id, UserId: operator.Id, Level: model.QuotaPoolAdminLevel},
		{PoolId: source.Id, UserId: target.Id, Level: model.QuotaPoolAdminLevel},
	}).Error)
	setQuotaPoolFeatureForTest(t)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodDelete, "/api/quota_pool/self/members/2", nil)
	c.Params = gin.Params{{Key: "user_id", Value: strconv.Itoa(target.Id)}}
	c.Set("id", operator.Id)
	c.Set("role", common.RoleCommonUser)

	RemoveSelfQuotaPoolMember(c)

	assert.Equal(t, http.StatusForbidden, recorder.Code)
	assert.Contains(t, recorder.Body.String(), `"code":"QUOTA_POOL_PERMISSION_DENIED"`)
}

func TestSystemAdministratorRemovesPoolAdministrator(t *testing.T) {
	db := setupModelListControllerTestDB(t)
	require.NoError(t, db.AutoMigrate(&model.Log{}, &model.QuotaPool{}, &model.QuotaPoolAdmin{}, &model.QuotaPoolTransaction{}))
	source := model.QuotaPool{Name: "global-remove-source", PoolType: model.QuotaPoolTypeNormal, Enabled: true, BaseQuota: 100, Quota: 100}
	targetPool := model.QuotaPool{Name: model.QuotaPoolNewUserName, PoolType: model.QuotaPoolTypeNewUser, Enabled: true, BaseQuota: -1, Quota: -1}
	require.NoError(t, db.Create(&source).Error)
	require.NoError(t, db.Create(&targetPool).Error)
	target := model.User{Username: "global-remove-target", Password: "password", AffCode: "global-remove-target", Role: common.RoleCommonUser, Status: common.UserStatusEnabled, QuotaPoolId: source.Id, Quota: 25}
	require.NoError(t, db.Create(&target).Error)
	require.NoError(t, db.Create(&model.QuotaPoolAdmin{PoolId: source.Id, UserId: target.Id, Level: model.QuotaPoolAdminLevel}).Error)
	setQuotaPoolFeatureForTest(t)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodDelete, "/api/quota_pool/1/members/1", nil)
	c.Params = gin.Params{{Key: "id", Value: strconv.Itoa(source.Id)}, {Key: "user_id", Value: strconv.Itoa(target.Id)}}
	c.Set("id", 99)
	c.Set("role", common.RoleAdminUser)

	RemoveQuotaPoolMember(c)

	assert.Equal(t, http.StatusOK, recorder.Code)
	var adminCount int64
	require.NoError(t, db.Model(&model.QuotaPoolAdmin{}).Where("user_id = ?", target.Id).Count(&adminCount).Error)
	assert.Zero(t, adminCount)
}

func TestLegacyMoveSelfQuotaPoolMemberAllowsOnlyNewUserPool(t *testing.T) {
	db := setupModelListControllerTestDB(t)
	require.NoError(t, db.AutoMigrate(&model.Log{}, &model.QuotaPool{}, &model.QuotaPoolAdmin{}, &model.QuotaPoolTransaction{}))
	source := model.QuotaPool{Name: "legacy-remove-source", PoolType: model.QuotaPoolTypeNormal, Enabled: true, BaseQuota: 100, Quota: 100}
	target := model.QuotaPool{Name: model.QuotaPoolNewUserName, PoolType: model.QuotaPoolTypeNewUser, Enabled: true, BaseQuota: -1, Quota: -1}
	other := model.QuotaPool{Name: "legacy-remove-other", PoolType: model.QuotaPoolTypeNormal, Enabled: true, BaseQuota: 50, Quota: 50}
	require.NoError(t, db.Create(&source).Error)
	require.NoError(t, db.Create(&target).Error)
	require.NoError(t, db.Create(&other).Error)
	operator := model.User{Username: "legacy-remove-admin", Password: "password", AffCode: "legacy-remove-admin", Role: common.RoleCommonUser, Status: common.UserStatusEnabled, QuotaPoolId: source.Id}
	member := model.User{Username: "legacy-remove-member", Password: "password", AffCode: "legacy-remove-member", Role: common.RoleCommonUser, Status: common.UserStatusEnabled, QuotaPoolId: source.Id, Quota: 25}
	require.NoError(t, db.Create(&operator).Error)
	require.NoError(t, db.Create(&member).Error)
	require.NoError(t, db.Create(&model.QuotaPoolAdmin{PoolId: source.Id, UserId: operator.Id, Level: model.QuotaPoolAdminLevel}).Error)
	setQuotaPoolFeatureForTest(t)

	invalidRecorder := httptest.NewRecorder()
	invalidContext, _ := gin.CreateTestContext(invalidRecorder)
	invalidContext.Request = httptest.NewRequest(http.MethodPut, "/api/quota_pool/self/members/2", strings.NewReader(fmt.Sprintf(`{"pool_id":%d}`, other.Id)))
	invalidContext.Params = gin.Params{{Key: "user_id", Value: strconv.Itoa(member.Id)}}
	invalidContext.Set("id", operator.Id)
	invalidContext.Set("role", common.RoleCommonUser)
	MoveSelfQuotaPoolMember(invalidContext)
	assert.Equal(t, http.StatusForbidden, invalidRecorder.Code)

	validRecorder := httptest.NewRecorder()
	validContext, _ := gin.CreateTestContext(validRecorder)
	validContext.Request = httptest.NewRequest(http.MethodPut, "/api/quota_pool/self/members/2", strings.NewReader(fmt.Sprintf(`{"pool_id":%d}`, target.Id)))
	validContext.Params = gin.Params{{Key: "user_id", Value: strconv.Itoa(member.Id)}}
	validContext.Set("id", operator.Id)
	validContext.Set("role", common.RoleCommonUser)
	MoveSelfQuotaPoolMember(validContext)
	assert.Equal(t, http.StatusOK, validRecorder.Code)
	require.NoError(t, db.First(&member, member.Id).Error)
	assert.Equal(t, target.Id, member.QuotaPoolId)
}

func TestPoolAdministratorCannotManagePoolAdministrators(t *testing.T) {
	db := setupModelListControllerTestDB(t)
	require.NoError(t, db.AutoMigrate(&model.QuotaPool{}, &model.QuotaPoolAdmin{}))
	pool := model.QuotaPool{Name: "self-admin-denied", PoolType: model.QuotaPoolTypeNormal, Enabled: true}
	require.NoError(t, db.Create(&pool).Error)
	operator := model.User{Username: "self-admin-operator", Password: "password", AffCode: "self-admin-operator", Role: common.RoleCommonUser, Status: common.UserStatusEnabled, QuotaPoolId: pool.Id}
	require.NoError(t, db.Create(&operator).Error)
	require.NoError(t, db.Create(&model.QuotaPoolAdmin{PoolId: pool.Id, UserId: operator.Id, Level: model.QuotaPoolAdminLevel}).Error)
	setQuotaPoolFeatureForTest(t)

	handlers := []struct {
		name    string
		method  string
		handler gin.HandlerFunc
	}{
		{name: "grant", method: http.MethodPost, handler: GrantSelfQuotaPoolAdmin},
		{name: "revoke", method: http.MethodDelete, handler: RevokeSelfQuotaPoolAdmin},
	}
	for _, item := range handlers {
		t.Run(item.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(recorder)
			c.Request = httptest.NewRequest(item.method, "/api/quota_pool/self/admins", strings.NewReader(`{"user_id":99}`))
			c.Set("id", operator.Id)
			c.Set("role", common.RoleCommonUser)
			item.handler(c)
			assert.Equal(t, http.StatusForbidden, recorder.Code)
			assert.Contains(t, recorder.Body.String(), `"code":"QUOTA_POOL_PERMISSION_DENIED"`)
		})
	}
}

func setQuotaPoolFeatureForTest(t *testing.T) {
	t.Helper()
	previous := common.QuotaPoolEnabled
	common.QuotaPoolEnabled = true
	t.Cleanup(func() { common.QuotaPoolEnabled = previous })
}
