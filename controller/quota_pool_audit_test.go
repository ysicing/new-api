package controller

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
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
