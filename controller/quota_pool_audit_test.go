package controller

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

type quotaPoolAuditOther struct {
	Op struct {
		Action string         `json:"action"`
		Params map[string]any `json:"params"`
	} `json:"op"`
}

func setupQuotaPoolAuditTest(t *testing.T) (*gorm.DB, *gin.Context) {
	t.Helper()
	db := setupModelListControllerTestDB(t)
	require.NoError(t, db.AutoMigrate(&model.Log{}, &model.QuotaPool{}))
	operator := model.User{
		Username: "audit-operator", Password: "password", AffCode: "audit-operator",
		Role: common.RoleAdminUser, Status: common.UserStatusEnabled,
	}
	require.NoError(t, db.Create(&operator).Error)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/api/quota_pool/audit", nil)
	c.Set("id", operator.Id)
	c.Set("username", operator.Username)
	return db, c
}

func lastQuotaPoolAudit(t *testing.T, db *gorm.DB) quotaPoolAuditOther {
	t.Helper()
	var log model.Log
	require.NoError(t, db.Order("id DESC").First(&log).Error)
	var other quotaPoolAuditOther
	require.NoError(t, common.UnmarshalJsonStr(log.Other, &other))
	return other
}

func TestRecordQuotaPoolAuditSnapshotsNames(t *testing.T) {
	db, c := setupQuotaPoolAuditTest(t)
	source := model.QuotaPool{Name: "平台保障部", PoolType: model.QuotaPoolTypeNormal, Enabled: true}
	target := model.QuotaPool{Name: model.QuotaPoolNewUserName, PoolType: model.QuotaPoolTypeNewUser, Enabled: true}
	member := model.User{
		Username: "alice", DisplayName: "张三", Password: "password",
		AffCode: "audit-alice", Role: common.RoleCommonUser, Status: common.UserStatusEnabled,
	}
	require.NoError(t, db.Create(&source).Error)
	require.NoError(t, db.Create(&target).Error)
	require.NoError(t, db.Create(&member).Error)

	recordQuotaPoolAudit(c, source.Id, "quota_pool.member_remove", map[string]any{
		"user_id": member.Id, "target_pool_id": target.Id, "amount": 250,
	})

	other := lastQuotaPoolAudit(t, db)
	assert.Equal(t, "quota_pool.member_remove", other.Op.Action)
	assert.Equal(t, "张三", other.Op.Params["user_name"])
	assert.Equal(t, "平台保障部", other.Op.Params["quota_pool_name"])
	assert.Equal(t, model.QuotaPoolNewUserName, other.Op.Params["target_pool_name"])
	assert.EqualValues(t, 250, other.Op.Params["amount"])
}

func TestRecordQuotaPoolAuditUsesUsernameWhenDisplayNameEmpty(t *testing.T) {
	db, c := setupQuotaPoolAuditTest(t)
	pool := model.QuotaPool{Name: "研发池", PoolType: model.QuotaPoolTypeNormal, Enabled: true}
	member := model.User{
		Username: "bob", Password: "password", AffCode: "audit-bob",
		Role: common.RoleCommonUser, Status: common.UserStatusEnabled,
	}
	require.NoError(t, db.Create(&pool).Error)
	require.NoError(t, db.Create(&member).Error)

	recordQuotaPoolAudit(c, pool.Id, "quota_pool.member_add", map[string]any{"user_id": member.Id})

	assert.Equal(t, "bob", lastQuotaPoolAudit(t, db).Op.Params["user_name"])
}

func TestRecordQuotaPoolAuditWritesReadableFallbackContent(t *testing.T) {
	db, c := setupQuotaPoolAuditTest(t)
	pool := model.QuotaPool{Name: "研发池", PoolType: model.QuotaPoolTypeNormal, Enabled: true}
	member := model.User{
		Username: "bob", Password: "password", AffCode: "audit-readable-bob",
		Role: common.RoleCommonUser, Status: common.UserStatusEnabled,
	}
	require.NoError(t, db.Create(&pool).Error)
	require.NoError(t, db.Create(&member).Error)

	recordQuotaPoolAudit(c, pool.Id, "quota_pool.member_add", map[string]any{"user_id": member.Id})

	var log model.Log
	require.NoError(t, db.Order("id DESC").First(&log).Error)
	assert.Equal(t, fmt.Sprintf("Added member bob (ID: %d) to 研发池", member.Id), log.Content)
}

func TestRecordQuotaPoolAuditClassifiesMemberBalanceChangesAsTopup(t *testing.T) {
	tests := []struct {
		action   string
		wantType int
	}{
		{action: "quota_pool.member_recharge", wantType: model.LogTypeTopup},
		{action: "quota_pool.member_reclaim", wantType: model.LogTypeTopup},
		{action: "quota_pool.member_add", wantType: model.LogTypeManage},
	}
	for _, tt := range tests {
		t.Run(tt.action, func(t *testing.T) {
			db, c := setupQuotaPoolAuditTest(t)
			pool := model.QuotaPool{Name: "研发池", PoolType: model.QuotaPoolTypeNormal, Enabled: true}
			member := model.User{
				Username: "balance-member", Password: "password", AffCode: "balance-member-" + tt.action,
				Role: common.RoleCommonUser, Status: common.UserStatusEnabled,
			}
			require.NoError(t, db.Create(&pool).Error)
			require.NoError(t, db.Create(&member).Error)

			recordQuotaPoolAudit(c, pool.Id, tt.action, map[string]any{
				"user_id": member.Id, "amount": 250,
			})

			var log model.Log
			require.NoError(t, db.Order("id DESC").First(&log).Error)
			assert.Equal(t, tt.wantType, log.Type)
		})
	}
}

func TestRecordQuotaPoolAuditReadsSoftDeletedPoolName(t *testing.T) {
	db, c := setupQuotaPoolAuditTest(t)
	pool := model.QuotaPool{Name: "待删除池", PoolType: model.QuotaPoolTypeNormal, Enabled: true}
	require.NoError(t, db.Create(&pool).Error)
	require.NoError(t, db.Delete(&pool).Error)

	recordQuotaPoolAudit(c, pool.Id, "quota_pool.delete", nil)

	assert.Equal(t, "待删除池", lastQuotaPoolAudit(t, db).Op.Params["quota_pool_name"])
}

func TestRecordQuotaPoolAuditStillWritesWhenSnapshotsCannotBeLoaded(t *testing.T) {
	db, c := setupQuotaPoolAuditTest(t)

	recordQuotaPoolAudit(c, 999, "quota_pool.member_move", map[string]any{
		"user_id": 998, "target_pool_id": 997,
	})

	params := lastQuotaPoolAudit(t, db).Op.Params
	assert.EqualValues(t, 999, params["quota_pool_id"])
	assert.EqualValues(t, 998, params["user_id"])
	assert.EqualValues(t, 997, params["target_pool_id"])
	assert.NotContains(t, params, "user_name")
	assert.NotContains(t, params, "quota_pool_name")
	assert.NotContains(t, params, "target_pool_name")
}

func TestQuotaPoolConfigAuditRecordsBeforeAndAfter(t *testing.T) {
	for _, self := range []bool{false, true} {
		t.Run(fmt.Sprint(self), func(t *testing.T) {
			db, c := setupQuotaPoolAuditTest(t)
			require.NoError(t, db.AutoMigrate(&model.QuotaPoolAdmin{}))
			previous := common.QuotaPoolEnabled
			common.QuotaPoolEnabled = true
			t.Cleanup(func() { common.QuotaPoolEnabled = previous })
			pool := model.QuotaPool{Name: "配置审计池", PoolType: model.QuotaPoolTypeNormal, Enabled: true, WeeklyLimit: 2}
			require.NoError(t, db.Create(&pool).Error)
			c.Request = httptest.NewRequest(http.MethodPut, "/api/quota_pool/config", strings.NewReader(`{"weekly_limit":5}`))
			c.Set("role", common.RoleRootUser)
			c.Params = gin.Params{{Key: "id", Value: fmt.Sprint(pool.Id)}}
			if self {
				c.Set("role", common.RoleCommonUser)
				require.NoError(t, db.Create(&model.QuotaPoolAdmin{PoolId: pool.Id, UserId: c.GetInt("id"), Level: model.QuotaPoolAdminLevel}).Error)
				UpdateSelfQuotaPool(c)
			} else {
				UpdateQuotaPool(c)
			}
			other := lastQuotaPoolAudit(t, db)
			expectedAction := "quota_pool.update"
			if self {
				expectedAction = "quota_pool.self_update"
			}
			assert.Equal(t, expectedAction, other.Op.Action)
			assert.Equal(t, float64(1), other.Op.Params["fields"])
			assert.Equal(t, []any{map[string]any{"field": "weekly_limit", "before": float64(2), "after": float64(5)}}, other.Op.Params["changes"])
		})
	}
}

func TestQuotaPoolMiddlewareKeepsOneOperationPerRequest(t *testing.T) {
	db := setupManageUserTestDB(t)
	previous := common.QuotaPoolEnabled
	common.QuotaPoolEnabled = true
	t.Cleanup(func() { common.QuotaPoolEnabled = previous })
	operator := model.User{
		Username: "pool-audit-root", Role: common.RoleRootUser,
		Status: common.UserStatusEnabled, AuthVersion: 1,
	}
	pool := model.QuotaPool{Name: "审计池", PoolType: model.QuotaPoolTypeNormal, Enabled: true}
	require.NoError(t, db.Create(&operator).Error)
	require.NoError(t, db.Create(&pool).Error)
	require.NoError(t, model.PublishUserAuthCache(operator.Id))
	bundle, err := service.CreateLoginSession(operator.Id, "password", "127.0.0.1", "quota-pool-audit-test")
	require.NoError(t, err)
	router := gin.New()
	router.Use(middleware.RequestId(), middleware.AccessTokenAudit())
	router.PUT("/api/quota_pool/:id", middleware.RootAuth(), UpdateQuotaPool)
	request := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/api/quota_pool/%d", pool.Id), strings.NewReader(`{"weekly_limit":5}`))
	request.Header.Set("Authorization", "Bearer "+bundle.AccessToken)
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	require.Equal(t, http.StatusOK, recorder.Code)
	require.Contains(t, recorder.Body.String(), `"success":true`)
	requestID := recorder.Header().Get(common.RequestIdKey)
	require.NotEmpty(t, requestID)
	var audits []model.AuditLog
	require.NoError(t, model.LOG_DB.Where("request_id = ? AND category = ?", requestID, model.AuditCategoryOperation).Find(&audits).Error)
	require.Len(t, audits, 1, "专用操作审计应阻止鉴权中间件重复记录通用操作")
	assert.Equal(t, "quota_pool.update", audits[0].Action)
	assert.Equal(t, operator.Id, audits[0].UserId)
	logs, total, err := model.ListQuotaPoolOperationLogs(pool.Id, &common.PageInfo{Page: 1, PageSize: 10}, "quota_pool.update")
	require.NoError(t, err)
	assert.EqualValues(t, 1, total)
	assert.Len(t, logs, 1, "额度池历史仍保留原有操作日志")
}
