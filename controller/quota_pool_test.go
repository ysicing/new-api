package controller

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func setupQuotaPoolControllerTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	gin.SetMode(gin.TestMode)

	oldDB := model.DB
	oldLogDB := model.LOG_DB
	oldRedisEnabled := common.RedisEnabled
	oldUsingSQLite := common.UsingSQLite
	oldUsingMySQL := common.UsingMySQL
	oldUsingPostgreSQL := common.UsingPostgreSQL
	oldQuotaPoolEnabled := common.QuotaPoolEnabled

	common.RedisEnabled = false
	common.UsingSQLite = true
	common.UsingMySQL = false
	common.UsingPostgreSQL = false
	common.QuotaPoolEnabled = true

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite failed: %v", err)
	}
	model.DB = db
	model.LOG_DB = db

	if err := db.AutoMigrate(&model.User{}, &model.Log{}, &model.QuotaPool{}, &model.QuotaPoolAdmin{}, &model.QuotaPoolTransaction{}); err != nil {
		t.Fatalf("migrate failed: %v", err)
	}

	t.Cleanup(func() {
		sqlDB, err := db.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
		model.DB = oldDB
		model.LOG_DB = oldLogDB
		common.RedisEnabled = oldRedisEnabled
		common.UsingSQLite = oldUsingSQLite
		common.UsingMySQL = oldUsingMySQL
		common.UsingPostgreSQL = oldUsingPostgreSQL
		common.QuotaPoolEnabled = oldQuotaPoolEnabled
	})
	return db
}

func quotaPoolTestContext(t *testing.T, method string, path string, body any, role int, userId int) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()
	var reader *bytes.Reader
	if body == nil {
		reader = bytes.NewReader(nil)
	} else {
		payload, err := common.Marshal(body)
		if err != nil {
			t.Fatalf("marshal body failed: %v", err)
		}
		reader = bytes.NewReader(payload)
	}
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(method, path, reader)
	ctx.Request.Header.Set("Content-Type", "application/json")
	ctx.Set("id", userId)
	ctx.Set("role", role)
	ctx.Set("username", "operator")
	return ctx, recorder
}

func TestQuotaPoolApiRejectsWhenFeatureDisabled(t *testing.T) {
	setupQuotaPoolControllerTestDB(t)
	common.QuotaPoolEnabled = false

	ctx, recorder := quotaPoolTestContext(t, http.MethodGet, "/api/quota_pool", nil, common.RoleAdminUser, 1)
	GetQuotaPools(ctx)

	response := decodeQuotaPoolResponse(t, recorder)
	if response["success"].(bool) {
		t.Fatalf("expected quota pool api to be rejected when feature disabled")
	}
	if !strings.Contains(response["message"].(string), "未启用") {
		t.Fatalf("unexpected message: %s", response["message"].(string))
	}
}

func decodeQuotaPoolResponse(t *testing.T, recorder *httptest.ResponseRecorder) map[string]interface{} {
	t.Helper()
	var response map[string]interface{}
	if err := common.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response failed: %v", err)
	}
	return response
}

func TestCreateQuotaPoolCreatesInitialFundTransaction(t *testing.T) {
	db := setupQuotaPoolControllerTestDB(t)
	ctx, recorder := quotaPoolTestContext(t, http.MethodPost, "/api/quota_pool", quotaPoolCreateRequest{
		Name:      "team-a",
		BaseQuota: 10,
	}, common.RoleRootUser, 99)

	CreateQuotaPool(ctx)

	response := decodeQuotaPoolResponse(t, recorder)
	if response["success"] != true {
		t.Fatalf("expected success response, got %#v", response)
	}
	var pool model.QuotaPool
	if err := db.First(&pool, "name = ?", "team-a").Error; err != nil {
		t.Fatalf("load pool failed: %v", err)
	}
	if pool.Quota != int(10*common.QuotaPerUnit) || pool.BaseQuota != int(10*common.QuotaPerUnit) {
		t.Fatalf("unexpected pool quota: %+v", pool)
	}
	if !pool.MonthlyRefillEnabled {
		t.Fatalf("expected monthly refill enabled by default")
	}
	if pool.MonthlyRefillAmount != pool.BaseQuota {
		t.Fatalf("monthly refill amount = %d, want base quota %d", pool.MonthlyRefillAmount, pool.BaseQuota)
	}
	today := time.Now().Day()
	if today > 28 {
		today = 28
	}
	if pool.MonthlyRefillDay != today {
		t.Fatalf("monthly refill day = %d, want %d", pool.MonthlyRefillDay, today)
	}
	var tx model.QuotaPoolTransaction
	if err := db.First(&tx, "pool_id = ? AND type = ?", pool.Id, model.QuotaPoolTransactionInitialFund).Error; err != nil {
		t.Fatalf("load initial fund tx failed: %v", err)
	}
	if tx.OperatorId != 99 {
		t.Fatalf("operator id = %d, want 99", tx.OperatorId)
	}
	var log model.Log
	if err := db.First(&log, "user_id = ? AND type = ?", 99, model.LogTypeManage).Error; err != nil {
		t.Fatalf("load create quota pool manage log failed: %v", err)
	}
	if !strings.Contains(log.Content, "创建额度池 team-a") || !strings.Contains(log.Content, "10.000000") {
		t.Fatalf("unexpected manage log content: %q", log.Content)
	}
}

func TestGrantSelfQuotaPoolAdminRejectsV2Grant(t *testing.T) {
	db := setupQuotaPoolControllerTestDB(t)
	pool := &model.QuotaPool{Name: "team", Enabled: true, BaseQuota: 100, Quota: 100, MonthlyRefillDay: 1}
	if err := db.Create(pool).Error; err != nil {
		t.Fatalf("create pool failed: %v", err)
	}
	v2 := &model.User{Id: 1, Username: "v2", Password: "password", Role: common.RoleCommonUser, Status: common.UserStatusEnabled, QuotaPoolId: pool.Id, AffCode: "v2-code"}
	target := &model.User{Id: 2, Username: "target", Password: "password", Role: common.RoleCommonUser, Status: common.UserStatusEnabled, QuotaPoolId: pool.Id, AffCode: "target-code"}
	if err := db.Create(v2).Error; err != nil {
		t.Fatalf("create v2 failed: %v", err)
	}
	if err := db.Create(target).Error; err != nil {
		t.Fatalf("create target failed: %v", err)
	}
	if err := db.Create(&model.QuotaPoolAdmin{PoolId: pool.Id, UserId: v2.Id, Level: model.QuotaPoolAdminLevelV2}).Error; err != nil {
		t.Fatalf("create admin failed: %v", err)
	}

	ctx, recorder := quotaPoolTestContext(t, http.MethodPost, "/api/quota_pool/self/admins", quotaPoolAdminRequest{
		UserId: target.Id,
		Level:  model.QuotaPoolAdminLevelV2,
	}, common.RoleCommonUser, v2.Id)

	GrantSelfQuotaPoolAdmin(ctx)

	response := decodeQuotaPoolResponse(t, recorder)
	if response["success"] == true {
		t.Fatalf("expected v2 self grant of v2 to fail")
	}
}

func TestGetSelfQuotaPoolIncludesCounts(t *testing.T) {
	db := setupQuotaPoolControllerTestDB(t)
	pool := &model.QuotaPool{Name: "team", Enabled: true, BaseQuota: 100, Quota: 100, MonthlyRefillDay: 1}
	if err := db.Create(pool).Error; err != nil {
		t.Fatalf("create pool failed: %v", err)
	}
	operator := &model.User{Id: 1, Username: "operator", Password: "password", Role: common.RoleCommonUser, Status: common.UserStatusEnabled, QuotaPoolId: pool.Id, AffCode: "operator-code"}
	member := &model.User{Id: 2, Username: "member", Password: "password", Role: common.RoleCommonUser, Status: common.UserStatusEnabled, QuotaPoolId: pool.Id, AffCode: "member-code"}
	if err := db.Create(operator).Error; err != nil {
		t.Fatalf("create operator failed: %v", err)
	}
	if err := db.Create(member).Error; err != nil {
		t.Fatalf("create member failed: %v", err)
	}
	if err := db.Create(&model.QuotaPoolAdmin{PoolId: pool.Id, UserId: operator.Id, Level: model.QuotaPoolAdminLevelV1}).Error; err != nil {
		t.Fatalf("create operator admin failed: %v", err)
	}

	ctx, recorder := quotaPoolTestContext(t, http.MethodGet, "/api/quota_pool/self", nil, common.RoleCommonUser, operator.Id)

	GetSelfQuotaPool(ctx)

	response := decodeQuotaPoolResponse(t, recorder)
	if response["success"] != true {
		t.Fatalf("expected success response, got %#v", response)
	}
	data := response["data"].(map[string]interface{})
	poolData := data["pool"].(map[string]interface{})
	if poolData["member_count"].(float64) != 2 {
		t.Fatalf("member_count = %v, want 2", poolData["member_count"])
	}
	if poolData["admin_count"].(float64) != 1 {
		t.Fatalf("admin_count = %v, want 1", poolData["admin_count"])
	}
}

func TestGrantQuotaPoolAdminRejectsInvalidLevelBeforeMovingDefaultUser(t *testing.T) {
	db := setupQuotaPoolControllerTestDB(t)
	pool := &model.QuotaPool{Name: "team", Enabled: true, BaseQuota: 1000, Quota: 1000, MonthlyRefillDay: 1}
	if err := db.Create(pool).Error; err != nil {
		t.Fatalf("create pool failed: %v", err)
	}
	user := &model.User{
		Username: "target",
		Password: "password",
		Role:     common.RoleCommonUser,
		Status:   common.UserStatusEnabled,
		Quota:    123,
		AffCode:  "target-code",
	}
	if err := db.Create(user).Error; err != nil {
		t.Fatalf("create user failed: %v", err)
	}

	ctx, recorder := quotaPoolTestContext(t, http.MethodPost, fmt.Sprintf("/api/quota_pool/%d/admins", pool.Id), quotaPoolAdminRequest{
		UserId: user.Id,
		Level:  99,
	}, common.RoleAdminUser, 99)
	ctx.Params = gin.Params{{Key: "id", Value: fmt.Sprintf("%d", pool.Id)}}

	GrantQuotaPoolAdmin(ctx)

	response := decodeQuotaPoolResponse(t, recorder)
	if response["success"] == true {
		t.Fatalf("expected invalid admin level to fail")
	}
	var got model.User
	_ = db.First(&got, user.Id).Error
	if got.Quota != 123 || got.QuotaPoolId != model.QuotaPoolDefaultUserPoolId {
		t.Fatalf("user should not be moved on invalid level: quota=%d pool=%d", got.Quota, got.QuotaPoolId)
	}
	var adminCount int64
	_ = db.Model(&model.QuotaPoolAdmin{}).Where("user_id = ?", user.Id).Count(&adminCount).Error
	if adminCount != 0 {
		t.Fatalf("expected no admin relation, got %d", adminCount)
	}
}

func TestRevokeSelfQuotaPoolAdminRejectsV2Target(t *testing.T) {
	db := setupQuotaPoolControllerTestDB(t)
	pool := &model.QuotaPool{Name: "team", Enabled: true, BaseQuota: 100, Quota: 100, MonthlyRefillDay: 1}
	if err := db.Create(pool).Error; err != nil {
		t.Fatalf("create pool failed: %v", err)
	}
	operator := &model.User{Id: 1, Username: "operator", Password: "password", Role: common.RoleCommonUser, Status: common.UserStatusEnabled, QuotaPoolId: pool.Id, AffCode: "operator-code"}
	target := &model.User{Id: 2, Username: "target", Password: "password", Role: common.RoleCommonUser, Status: common.UserStatusEnabled, QuotaPoolId: pool.Id, AffCode: "target-code"}
	if err := db.Create(operator).Error; err != nil {
		t.Fatalf("create operator failed: %v", err)
	}
	if err := db.Create(target).Error; err != nil {
		t.Fatalf("create target failed: %v", err)
	}
	if err := db.Create(&model.QuotaPoolAdmin{PoolId: pool.Id, UserId: operator.Id, Level: model.QuotaPoolAdminLevelV2}).Error; err != nil {
		t.Fatalf("create operator admin failed: %v", err)
	}
	if err := db.Create(&model.QuotaPoolAdmin{PoolId: pool.Id, UserId: target.Id, Level: model.QuotaPoolAdminLevelV2}).Error; err != nil {
		t.Fatalf("create target admin failed: %v", err)
	}

	ctx, recorder := quotaPoolTestContext(t, http.MethodDelete, fmt.Sprintf("/api/quota_pool/self/admins/%d", target.Id), nil, common.RoleCommonUser, operator.Id)
	ctx.Params = gin.Params{{Key: "user_id", Value: fmt.Sprintf("%d", target.Id)}}

	RevokeSelfQuotaPoolAdmin(ctx)

	response := decodeQuotaPoolResponse(t, recorder)
	if response["success"] == true {
		t.Fatalf("expected revoking v2 target to fail")
	}
	var count int64
	_ = db.Model(&model.QuotaPoolAdmin{}).Where("user_id = ?", target.Id).Count(&count)
	if count != 1 {
		t.Fatalf("expected target admin relation to remain, got %d", count)
	}
}

func TestGrantAndRevokeQuotaPoolAdminWriteManageLogs(t *testing.T) {
	db := setupQuotaPoolControllerTestDB(t)
	pool := &model.QuotaPool{Name: "team", Enabled: true, BaseQuota: 100, Quota: 100, MonthlyRefillDay: 1}
	if err := db.Create(pool).Error; err != nil {
		t.Fatalf("create pool failed: %v", err)
	}
	target := &model.User{Id: 2, Username: "target", Password: "password", Role: common.RoleCommonUser, Status: common.UserStatusEnabled, QuotaPoolId: pool.Id, AffCode: "target-code"}
	if err := db.Create(target).Error; err != nil {
		t.Fatalf("create target failed: %v", err)
	}

	grantCtx, grantRecorder := quotaPoolTestContext(t, http.MethodPost, fmt.Sprintf("/api/quota_pool/%d/admins", pool.Id), quotaPoolAdminRequest{
		UserId: target.Id,
		Level:  model.QuotaPoolAdminLevelV2,
	}, common.RoleAdminUser, 99)
	grantCtx.Params = gin.Params{{Key: "id", Value: fmt.Sprintf("%d", pool.Id)}}
	GrantQuotaPoolAdmin(grantCtx)
	if response := decodeQuotaPoolResponse(t, grantRecorder); response["success"] != true {
		t.Fatalf("expected grant success, got %#v", response)
	}

	revokeCtx, revokeRecorder := quotaPoolTestContext(t, http.MethodDelete, fmt.Sprintf("/api/quota_pool/%d/admins/%d", pool.Id, target.Id), nil, common.RoleAdminUser, 99)
	revokeCtx.Params = gin.Params{{Key: "id", Value: fmt.Sprintf("%d", pool.Id)}, {Key: "user_id", Value: fmt.Sprintf("%d", target.Id)}}
	RevokeQuotaPoolAdmin(revokeCtx)
	if response := decodeQuotaPoolResponse(t, revokeRecorder); response["success"] != true {
		t.Fatalf("expected revoke success, got %#v", response)
	}

	var logs []model.Log
	if err := db.Where("user_id = ? AND type = ?", target.Id, model.LogTypeManage).Order("id asc").Find(&logs).Error; err != nil {
		t.Fatalf("load manage logs failed: %v", err)
	}
	if len(logs) != 2 {
		t.Fatalf("expected two manage logs, got %d", len(logs))
	}
	if !strings.Contains(logs[0].Content, "任命") || !strings.Contains(logs[0].Content, "池超级管理员 v2") {
		t.Fatalf("unexpected grant log: %q", logs[0].Content)
	}
	if !strings.Contains(logs[1].Content, "撤销额度池管理员") {
		t.Fatalf("unexpected revoke log: %q", logs[1].Content)
	}
}

func TestSelfQuotaPoolAdminGrantAndRevokeWriteManageLogs(t *testing.T) {
	db := setupQuotaPoolControllerTestDB(t)
	pool := &model.QuotaPool{Name: "team", Enabled: true, BaseQuota: 100, Quota: 100, MonthlyRefillDay: 1}
	if err := db.Create(pool).Error; err != nil {
		t.Fatalf("create pool failed: %v", err)
	}
	operator := &model.User{Id: 1, Username: "operator", Password: "password", Role: common.RoleCommonUser, Status: common.UserStatusEnabled, QuotaPoolId: pool.Id, AffCode: "operator-code"}
	target := &model.User{Id: 2, Username: "target", Password: "password", Role: common.RoleCommonUser, Status: common.UserStatusEnabled, QuotaPoolId: pool.Id, AffCode: "target-code"}
	if err := db.Create(operator).Error; err != nil {
		t.Fatalf("create operator failed: %v", err)
	}
	if err := db.Create(target).Error; err != nil {
		t.Fatalf("create target failed: %v", err)
	}
	if err := db.Create(&model.QuotaPoolAdmin{PoolId: pool.Id, UserId: operator.Id, Level: model.QuotaPoolAdminLevelV2}).Error; err != nil {
		t.Fatalf("create operator admin failed: %v", err)
	}

	grantCtx, grantRecorder := quotaPoolTestContext(t, http.MethodPost, "/api/quota_pool/self/admins", quotaPoolAdminRequest{
		UserId: target.Id,
		Level:  model.QuotaPoolAdminLevelV1,
	}, common.RoleCommonUser, operator.Id)
	GrantSelfQuotaPoolAdmin(grantCtx)
	if response := decodeQuotaPoolResponse(t, grantRecorder); response["success"] != true {
		t.Fatalf("expected self grant success, got %#v", response)
	}

	revokeCtx, revokeRecorder := quotaPoolTestContext(t, http.MethodDelete, fmt.Sprintf("/api/quota_pool/self/admins/%d", target.Id), nil, common.RoleCommonUser, operator.Id)
	revokeCtx.Params = gin.Params{{Key: "user_id", Value: fmt.Sprintf("%d", target.Id)}}
	RevokeSelfQuotaPoolAdmin(revokeCtx)
	if response := decodeQuotaPoolResponse(t, revokeRecorder); response["success"] != true {
		t.Fatalf("expected self revoke success, got %#v", response)
	}

	var logs []model.Log
	if err := db.Where("user_id = ? AND type = ?", target.Id, model.LogTypeManage).Order("id asc").Find(&logs).Error; err != nil {
		t.Fatalf("load manage logs failed: %v", err)
	}
	if len(logs) != 2 {
		t.Fatalf("expected two manage logs, got %d", len(logs))
	}
	if !strings.Contains(logs[0].Content, "任命") || !strings.Contains(logs[0].Content, "池管理员 v1") {
		t.Fatalf("unexpected self grant log: %q", logs[0].Content)
	}
	if !strings.Contains(logs[1].Content, "撤销额度池管理员") {
		t.Fatalf("unexpected self revoke log: %q", logs[1].Content)
	}
}

func TestDeleteQuotaPoolRejectsPoolWithMembers(t *testing.T) {
	db := setupQuotaPoolControllerTestDB(t)
	pool := &model.QuotaPool{Name: "team", Enabled: true, BaseQuota: 100, Quota: 100, MonthlyRefillDay: 1}
	if err := db.Create(pool).Error; err != nil {
		t.Fatalf("create pool failed: %v", err)
	}
	if err := db.Create(&model.User{Username: "member", Password: "password", Role: common.RoleCommonUser, Status: common.UserStatusEnabled, QuotaPoolId: pool.Id, AffCode: "member-code"}).Error; err != nil {
		t.Fatalf("create member failed: %v", err)
	}
	ctx, recorder := quotaPoolTestContext(t, http.MethodDelete, fmt.Sprintf("/api/quota_pool/%d", pool.Id), nil, common.RoleRootUser, 99)
	ctx.Params = gin.Params{{Key: "id", Value: fmt.Sprintf("%d", pool.Id)}}

	DeleteQuotaPool(ctx)

	response := decodeQuotaPoolResponse(t, recorder)
	if response["success"] == true {
		t.Fatalf("expected delete pool with members to fail")
	}
}

func TestAddQuotaPoolMemberInitialRechargeUsesAutoRechargeLimits(t *testing.T) {
	db := setupQuotaPoolControllerTestDB(t)
	cfg := operation_setting.GetAutoRechargeSetting()
	originalAmount := cfg.Amount
	originalThreshold := cfg.Threshold
	originalWeeklyLimit := cfg.WeeklyLimit
	originalMonthlyLimit := cfg.MonthlyLimit
	defer func() {
		cfg.Amount = originalAmount
		cfg.Threshold = originalThreshold
		cfg.WeeklyLimit = originalWeeklyLimit
		cfg.MonthlyLimit = originalMonthlyLimit
	}()
	cfg.Amount = 2
	cfg.Threshold = 10
	cfg.WeeklyLimit = 0
	cfg.MonthlyLimit = 0

	pool := &model.QuotaPool{
		Name:               "team",
		Enabled:            true,
		BaseQuota:          int(100 * common.QuotaPerUnit),
		Quota:              int(100 * common.QuotaPerUnit),
		AutoRechargeAmount: model.QuotaPoolAutoRechargeInherit,
		WeeklyLimit:        1,
		MonthlyLimit:       0,
		MonthlyRefillDay:   1,
	}
	if err := db.Create(pool).Error; err != nil {
		t.Fatalf("create pool failed: %v", err)
	}
	user := &model.User{Username: "member", Password: "password", Role: common.RoleCommonUser, Status: common.UserStatusEnabled, Quota: int(20 * common.QuotaPerUnit), AffCode: "member-code"}
	if err := db.Create(user).Error; err != nil {
		t.Fatalf("create user failed: %v", err)
	}
	if err := db.Create(&model.Log{
		UserId:    user.Id,
		Type:      model.LogTypeSystem,
		Content:   "系统自动赠送 2",
		CreatedAt: common.GetTimestamp(),
	}).Error; err != nil {
		t.Fatalf("create auto recharge log failed: %v", err)
	}

	ctx, recorder := quotaPoolTestContext(t, http.MethodPost, fmt.Sprintf("/api/quota_pool/%d/members", pool.Id), quotaPoolMemberRequest{
		UserId: user.Id,
	}, common.RoleAdminUser, 99)
	ctx.Params = gin.Params{{Key: "id", Value: fmt.Sprintf("%d", pool.Id)}}

	AddQuotaPoolMember(ctx)

	response := decodeQuotaPoolResponse(t, recorder)
	if response["success"] != true {
		t.Fatalf("expected add member success, got %#v", response)
	}
	if !strings.Contains(fmt.Sprint(response["message"]), "weekly_limited") {
		t.Fatalf("expected threshold warning, got %#v", response)
	}
	var gotUser model.User
	_ = db.First(&gotUser, user.Id).Error
	if gotUser.Quota != 0 || gotUser.QuotaPoolId != pool.Id {
		t.Fatalf("unexpected user after add: quota=%d pool=%d", gotUser.Quota, gotUser.QuotaPoolId)
	}
	var gotPool model.QuotaPool
	_ = db.First(&gotPool, pool.Id).Error
	if gotPool.Quota != pool.Quota {
		t.Fatalf("pool quota = %d, want unchanged %d", gotPool.Quota, pool.Quota)
	}
	var txCount int64
	_ = db.Model(&model.QuotaPoolTransaction{}).Where("pool_id = ? AND type = ?", pool.Id, model.QuotaPoolTransactionAllocateManual).Count(&txCount).Error
	if txCount != 0 {
		t.Fatalf("initial recharge should not write manual allocation, got %d", txCount)
	}
}

func TestRechargeQuotaPoolMemberRejectsNonMember(t *testing.T) {
	db := setupQuotaPoolControllerTestDB(t)
	pool := &model.QuotaPool{Name: "team", Enabled: true, BaseQuota: 1000, Quota: 1000, AutoRechargeAmount: 100, MonthlyRefillDay: 1}
	otherPool := &model.QuotaPool{Name: "other", Enabled: true, BaseQuota: 1000, Quota: 1000, AutoRechargeAmount: 100, MonthlyRefillDay: 1}
	if err := db.Create(pool).Error; err != nil {
		t.Fatalf("create pool failed: %v", err)
	}
	if err := db.Create(otherPool).Error; err != nil {
		t.Fatalf("create other pool failed: %v", err)
	}
	user := &model.User{Username: "member", Password: "password", Role: common.RoleCommonUser, Status: common.UserStatusEnabled, QuotaPoolId: otherPool.Id, AffCode: "member-code"}
	if err := db.Create(user).Error; err != nil {
		t.Fatalf("create user failed: %v", err)
	}
	ctx, recorder := quotaPoolTestContext(t, http.MethodPost, fmt.Sprintf("/api/quota_pool/%d/members/%d/recharge", pool.Id, user.Id), nil, common.RoleAdminUser, 99)
	ctx.Params = gin.Params{{Key: "id", Value: fmt.Sprintf("%d", pool.Id)}, {Key: "user_id", Value: fmt.Sprintf("%d", user.Id)}}

	RechargeQuotaPoolMember(ctx)

	response := decodeQuotaPoolResponse(t, recorder)
	if response["success"] == true {
		t.Fatalf("expected non-member recharge to fail")
	}
	var gotPool model.QuotaPool
	_ = db.First(&gotPool, pool.Id).Error
	if gotPool.Quota != 1000 {
		t.Fatalf("pool quota = %d, want 1000", gotPool.Quota)
	}
}

func TestRechargeQuotaPoolMemberRejectsDefaultPool(t *testing.T) {
	db := setupQuotaPoolControllerTestDB(t)
	user := &model.User{Username: "default-member", Password: "password", Role: common.RoleCommonUser, Status: common.UserStatusEnabled, QuotaPoolId: model.QuotaPoolDefaultUserPoolId, AffCode: "default-code"}
	if err := db.Create(user).Error; err != nil {
		t.Fatalf("create user failed: %v", err)
	}
	ctx, recorder := quotaPoolTestContext(t, http.MethodPost, fmt.Sprintf("/api/quota_pool/0/members/%d/recharge", user.Id), nil, common.RoleAdminUser, 99)
	ctx.Params = gin.Params{{Key: "id", Value: "0"}, {Key: "user_id", Value: fmt.Sprintf("%d", user.Id)}}

	RechargeQuotaPoolMember(ctx)

	response := decodeQuotaPoolResponse(t, recorder)
	if response["success"] == true {
		t.Fatalf("expected default pool member recharge to fail")
	}
	var gotUser model.User
	_ = db.First(&gotUser, user.Id).Error
	if gotUser.Quota != 0 {
		t.Fatalf("user quota = %d, want 0", gotUser.Quota)
	}
}

func TestUpdateQuotaPoolRejectsEnabledMonthlyRefillWithoutAmount(t *testing.T) {
	db := setupQuotaPoolControllerTestDB(t)
	pool := &model.QuotaPool{Name: "team", Enabled: true, BaseQuota: 1000, Quota: 1000, MonthlyRefillDay: 1}
	if err := db.Create(pool).Error; err != nil {
		t.Fatalf("create pool failed: %v", err)
	}
	enabled := true
	ctx, recorder := quotaPoolTestContext(t, http.MethodPut, fmt.Sprintf("/api/quota_pool/%d", pool.Id), quotaPoolUpdateRequest{
		MonthlyRefillEnabled: &enabled,
	}, common.RoleRootUser, 99)
	ctx.Params = gin.Params{{Key: "id", Value: fmt.Sprintf("%d", pool.Id)}}

	UpdateQuotaPool(ctx)

	response := decodeQuotaPoolResponse(t, recorder)
	if response["success"] == true {
		t.Fatalf("expected enabled monthly refill without amount to fail")
	}
}
