package controller

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupQuotaPoolRechargeQueryControllerTest(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:quota-pool-recharge-query-controller-%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.User{}, &model.Log{}, &model.QuotaPool{}, &model.QuotaPoolTransaction{}))
	previousDB, previousLogDB := model.DB, model.LOG_DB
	previousEnabled := common.QuotaPoolEnabled
	previousConfig := *operation_setting.GetAutoRechargeSetting()
	model.DB, model.LOG_DB = db, db
	common.QuotaPoolEnabled = true
	common.SetDatabaseTypes(common.DatabaseTypeSQLite, common.DatabaseTypeSQLite)
	*operation_setting.GetAutoRechargeSetting() = operation_setting.AutoRechargeSetting{Enabled: true, Interval: 30, Threshold: 2, Amount: 1}
	t.Cleanup(func() {
		model.DB, model.LOG_DB = previousDB, previousLogDB
		common.QuotaPoolEnabled = previousEnabled
		*operation_setting.GetAutoRechargeSetting() = previousConfig
	})
	return db
}

func TestGetQuotaPoolRechargeRecordsReturnsCurrentWeekPage(t *testing.T) {
	db := setupQuotaPoolRechargeQueryControllerTest(t)
	now := common.GetTimestamp()
	pool := model.QuotaPool{Name: "查询池", PoolType: model.QuotaPoolTypeNormal, Enabled: true, BaseQuota: 1000, Quota: 1000}
	user := model.User{Username: "records-user", Password: "password", AffCode: "records-user-aff", Role: common.RoleCommonUser, Status: common.UserStatusEnabled}
	require.NoError(t, db.Create(&pool).Error)
	require.NoError(t, db.Create(&user).Error)
	require.NoError(t, db.Create(&model.QuotaPoolTransaction{PoolId: pool.Id, UserId: user.Id, Type: model.QuotaPoolTransactionAllocateAuto, Amount: -100, CreatedAt: now}).Error)
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodGet, "/api/quota_pool/recharge_query/records?period=week&p=1&page_size=20", nil)

	GetQuotaPoolRechargeRecords(context)

	var response struct {
		Success bool `json:"success"`
		Data    struct {
			Total int                             `json:"total"`
			Items []model.QuotaPoolRechargeRecord `json:"items"`
		} `json:"data"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	assert.True(t, response.Success)
	assert.Equal(t, 1, response.Data.Total)
	require.Len(t, response.Data.Items, 1)
	assert.Equal(t, 100, response.Data.Items[0].Amount)
}

func TestGetQuotaPoolRechargeEligibilityReturnsReadOnlyDiagnosis(t *testing.T) {
	db := setupQuotaPoolRechargeQueryControllerTest(t)
	amount := common.QuotaFromFloat(common.QuotaPerUnit)
	pool := model.QuotaPool{
		Name: "诊断池", PoolType: model.QuotaPoolTypeNormal, Enabled: true,
		BaseQuota: amount * 2, Quota: amount * 2,
		AutoRechargeAmount: model.QuotaPoolAutoRechargeInherit,
		WeeklyLimit:        model.QuotaPoolAutoRechargeInherit,
		MonthlyLimit:       model.QuotaPoolAutoRechargeInherit,
	}
	require.NoError(t, db.Create(&pool).Error)
	user := model.User{Username: "diagnosis-user", Email: "diagnosis@example.com", Password: "password", AffCode: "diagnosis-user-aff", Role: common.RoleCommonUser, Status: common.UserStatusEnabled, QuotaPoolId: pool.Id}
	require.NoError(t, db.Create(&user).Error)
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodPost, "/api/quota_pool/recharge_query/eligibility", strings.NewReader(`{"identifier":"diagnosis@example.com"}`))

	GetQuotaPoolRechargeEligibility(context)

	var response struct {
		Success bool                            `json:"success"`
		Data    service.AutoRechargeEligibility `json:"data"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	assert.True(t, common.GetContextKeyBool(context, constant.ContextKeyAuditSkip))
	assert.True(t, response.Success)
	assert.True(t, response.Data.Eligible)
	assert.Equal(t, user.Id, response.Data.UserId)
	assert.Equal(t, amount, response.Data.Amount)
	var transactionCount int64
	require.NoError(t, db.Model(&model.QuotaPoolTransaction{}).Count(&transactionCount).Error)
	assert.Zero(t, transactionCount)
}

func TestQuotaPoolRechargeReadEndpointsRemainAvailableWhenFeatureIsDisabled(t *testing.T) {
	db := setupQuotaPoolRechargeQueryControllerTest(t)
	common.QuotaPoolEnabled = false
	now := common.GetTimestamp()
	pool := model.QuotaPool{Name: "历史池", PoolType: model.QuotaPoolTypeNormal, Enabled: true, BaseQuota: 1000, Quota: 1000}
	user := model.User{Username: "feature-off-user", Password: "password", AffCode: "feature-off-user-aff", Role: common.RoleCommonUser, Status: common.UserStatusEnabled}
	require.NoError(t, db.Create(&pool).Error)
	require.NoError(t, db.Create(&user).Error)
	require.NoError(t, db.Create(&model.QuotaPoolTransaction{PoolId: pool.Id, UserId: user.Id, Type: model.QuotaPoolTransactionAllocateManual, Amount: -100, CreatedAt: now}).Error)

	for _, endpoint := range []struct {
		method  string
		path    string
		body    string
		handler gin.HandlerFunc
	}{
		{method: http.MethodGet, path: "/api/quota_pool/recharge_query/records?period=week", handler: GetQuotaPoolRechargeRecords},
		{method: http.MethodPost, path: "/api/quota_pool/recharge_query/eligibility", body: `{"identifier":"feature-off-user"}`, handler: GetQuotaPoolRechargeEligibility},
	} {
		recorder := httptest.NewRecorder()
		context, _ := gin.CreateTestContext(recorder)
		context.Request = httptest.NewRequest(endpoint.method, endpoint.path, strings.NewReader(endpoint.body))

		endpoint.handler(context)

		assert.Equal(t, http.StatusOK, recorder.Code, endpoint.path)
	}
}

func TestQuotaPoolRechargeQueryErrorsPreserveCodesAndExposeTranslationKeys(t *testing.T) {
	for _, test := range []struct {
		name       string
		method     string
		path       string
		body       string
		handler    gin.HandlerFunc
		userId     int
		ambiguous  bool
		dropTable  any
		wantStatus int
		wantCode   string
		wantKey    string
	}{
		{
			name:   "invalid JSON returns the identifier translation key",
			method: http.MethodPost, path: "/api/quota_pool/recharge_query/eligibility",
			body: "{", handler: GetQuotaPoolRechargeEligibility,
			wantStatus: http.StatusBadRequest, wantCode: "RECHARGE_IDENTIFIER_REQUIRED",
			wantKey: "Enter a user ID, username, or email.",
		},
		{
			name:   "empty identifier returns the identifier translation key",
			method: http.MethodPost, path: "/api/quota_pool/recharge_query/eligibility",
			body: `{"identifier":"  "}`, handler: GetQuotaPoolRechargeEligibility,
			wantStatus: http.StatusBadRequest, wantCode: "RECHARGE_IDENTIFIER_REQUIRED",
			wantKey: "Enter a user ID, username, or email.",
		},
		{
			name:   "missing user returns the not-found translation key",
			method: http.MethodPost, path: "/api/quota_pool/recharge_query/eligibility",
			body: `{"identifier":"missing-user"}`, handler: GetQuotaPoolRechargeEligibility,
			wantStatus: http.StatusNotFound, wantCode: "RECHARGE_USER_NOT_FOUND",
			wantKey: "No matching user was found.",
		},
		{
			name:   "ambiguous user returns the conflict translation key",
			method: http.MethodPost, path: "/api/quota_pool/recharge_query/eligibility",
			body: `{"identifier":"ambiguous-user"}`, handler: GetQuotaPoolRechargeEligibility, ambiguous: true,
			wantStatus: http.StatusConflict, wantCode: "RECHARGE_USER_AMBIGUOUS",
			wantKey: "Multiple users matched. Use the user ID to query.",
		},
		{
			name:   "eligibility query failure returns a safe translation key",
			method: http.MethodPost, path: "/api/quota_pool/recharge_query/eligibility",
			body: `{"identifier":"query-user"}`, handler: GetQuotaPoolRechargeEligibility, dropTable: &model.User{},
			wantStatus: http.StatusInternalServerError, wantCode: "RECHARGE_QUERY_INTERNAL",
			wantKey: "Failed to query recharge eligibility",
		},
		{
			name:   "records query failure returns a safe translation key",
			method: http.MethodGet, path: "/api/quota_pool/recharge_query/records",
			handler: GetQuotaPoolRechargeRecords, dropTable: &model.QuotaPoolTransaction{},
			wantStatus: http.StatusInternalServerError, wantCode: "RECHARGE_RECORDS_QUERY_FAILED",
			wantKey: "Failed to query recharge records",
		},
		{
			name:   "missing authenticated user returns the self-service translation key",
			method: http.MethodGet, path: "/api/user/auto_recharge/eligibility",
			handler: GetSelfAutoRechargeEligibility, userId: 98765,
			wantStatus: http.StatusNotFound, wantCode: "SELF_AUTO_RECHARGE_USER_NOT_FOUND",
			wantKey: "User does not exist",
		},
		{
			name:   "self-service query failure returns a safe translation key",
			method: http.MethodGet, path: "/api/user/auto_recharge/eligibility",
			handler: GetSelfAutoRechargeEligibility, userId: 98765, dropTable: &model.User{},
			wantStatus: http.StatusInternalServerError, wantCode: "SELF_AUTO_RECHARGE_QUERY_FAILED",
			wantKey: "Failed to query automatic recharge eligibility",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			db := setupQuotaPoolRechargeQueryControllerTest(t)
			if test.ambiguous {
				users := []model.User{
					{Username: "ambiguous-user", Email: "first@example.com", Password: "password", AffCode: "ambiguous-first"},
					{Username: "second-user", Email: "ambiguous-user", Password: "password", AffCode: "ambiguous-second"},
				}
				require.NoError(t, db.Create(&users).Error)
			}
			if test.dropTable != nil {
				require.NoError(t, db.Migrator().DropTable(test.dropTable))
			}
			recorder := httptest.NewRecorder()
			context, _ := gin.CreateTestContext(recorder)
			context.Set("id", test.userId)
			context.Request = httptest.NewRequest(test.method, test.path, strings.NewReader(test.body))

			test.handler(context)

			var response map[string]any
			require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
			assert.Equal(t, test.wantStatus, recorder.Code)
			assert.True(t, context.IsAborted())
			assert.Equal(t, map[string]any{
				"success": false, "code": test.wantCode,
				"message": test.wantKey, "message_key": test.wantKey,
			}, response)
		})
	}
}
