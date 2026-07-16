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

func assertQuotaPoolManageLogContains(t *testing.T, db *gorm.DB, userId int, content string) {
	t.Helper()
	var count int64
	if err := db.Model(&model.Log{}).
		Where("user_id = ? AND type = ? AND content LIKE ?", userId, model.LogTypeManage, "%"+content+"%").
		Count(&count).Error; err != nil {
		t.Fatalf("count manage log failed: %v", err)
	}
	if count == 0 {
		t.Fatalf("expected manage log for user %d containing %q", userId, content)
	}
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

func TestSyncDefaultQuotaPoolCreatesDefaultOnce(t *testing.T) {
	db := setupQuotaPoolControllerTestDB(t)
	ctx, recorder := quotaPoolTestContext(t, http.MethodPost, "/api/quota_pool/sync_default", nil, common.RoleRootUser, 99)

	SyncDefaultQuotaPool(ctx)

	response := decodeQuotaPoolResponse(t, recorder)
	if response["success"] != true {
		t.Fatalf("expected success response, got %#v", response)
	}
	var count int64
	if err := db.Model(&model.QuotaPool{}).Where("is_default = ?", true).Count(&count).Error; err != nil {
		t.Fatalf("count default pool failed: %v", err)
	}
	if count != 1 {
		t.Fatalf("default pool count = %d, want 1", count)
	}
	assertQuotaPoolManageLogContains(t, db, 99, "同步默认额度池")

	ctx, recorder = quotaPoolTestContext(t, http.MethodPost, "/api/quota_pool/sync_default", nil, common.RoleRootUser, 99)
	SyncDefaultQuotaPool(ctx)
	response = decodeQuotaPoolResponse(t, recorder)
	if response["success"] != true {
		t.Fatalf("expected second sync success response, got %#v", response)
	}
	if err := db.Model(&model.QuotaPool{}).Where("is_default = ?", true).Count(&count).Error; err != nil {
		t.Fatalf("count default pool after second sync failed: %v", err)
	}
	if count != 1 {
		t.Fatalf("default pool count after second sync = %d, want 1", count)
	}
}

func TestGrantSelfQuotaPoolAdminRejectsAdminGrant(t *testing.T) {
	db := setupQuotaPoolControllerTestDB(t)
	pool := &model.QuotaPool{Name: "team", Enabled: true, BaseQuota: 100, Quota: 100, MonthlyRefillDay: 1}
	if err := db.Create(pool).Error; err != nil {
		t.Fatalf("create pool failed: %v", err)
	}
	operator := &model.User{Id: 1, Username: "operator", Password: "password", Role: common.RoleCommonUser, Status: common.UserStatusEnabled, Email: "operator@example.com", QuotaPoolId: pool.Id, AffCode: "operator-code"}
	target := &model.User{Id: 2, Username: "target", Password: "password", Role: common.RoleCommonUser, Status: common.UserStatusEnabled, QuotaPoolId: pool.Id, AffCode: "target-code"}
	if err := db.Create(operator).Error; err != nil {
		t.Fatalf("create operator failed: %v", err)
	}
	if err := db.Create(target).Error; err != nil {
		t.Fatalf("create target failed: %v", err)
	}
	if err := db.Create(&model.QuotaPoolAdmin{PoolId: pool.Id, UserId: operator.Id, Level: model.QuotaPoolAdminLevelV1}).Error; err != nil {
		t.Fatalf("create admin failed: %v", err)
	}

	ctx, recorder := quotaPoolTestContext(t, http.MethodPost, "/api/quota_pool/self/admins", quotaPoolAdminRequest{
		UserId: target.Id,
		Level:  model.QuotaPoolAdminLevelV1,
	}, common.RoleCommonUser, operator.Id)

	GrantSelfQuotaPoolAdmin(ctx)

	response := decodeQuotaPoolResponse(t, recorder)
	if response["success"] == true {
		t.Fatalf("expected self grant admin to fail")
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
	defaultMember := &model.User{Id: 3, Username: "default-member", Password: "password", Role: common.RoleCommonUser, Status: common.UserStatusEnabled, QuotaPoolId: model.QuotaPoolDefaultUserPoolId, AffCode: "default-member-code"}
	if err := db.Create(operator).Error; err != nil {
		t.Fatalf("create operator failed: %v", err)
	}
	if err := db.Create(member).Error; err != nil {
		t.Fatalf("create member failed: %v", err)
	}
	if err := db.Create(defaultMember).Error; err != nil {
		t.Fatalf("create default member failed: %v", err)
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

	memberCtx, memberRecorder := quotaPoolTestContext(t, http.MethodGet, "/api/quota_pool/self", nil, common.RoleCommonUser, member.Id)
	GetSelfQuotaPool(memberCtx)
	memberResponse := decodeQuotaPoolResponse(t, memberRecorder)
	if memberResponse["success"] != true {
		t.Fatalf("expected member success response, got %#v", memberResponse)
	}
	memberData := memberResponse["data"].(map[string]interface{})
	memberPoolData := memberData["pool"].(map[string]interface{})
	if int(memberPoolData["id"].(float64)) != pool.Id {
		t.Fatalf("member pool id = %v, want %d", memberPoolData["id"], pool.Id)
	}
	if memberData["admin"] != nil {
		t.Fatalf("member should not receive admin summary, got %#v", memberData["admin"])
	}
	adminContacts := memberData["admin_contacts"].([]interface{})
	if len(adminContacts) != 1 {
		t.Fatalf("expected one admin contact, got %#v", adminContacts)
	}
	adminContact := adminContacts[0].(map[string]interface{})
	if adminContact["username"] != operator.Username || adminContact["email"] != operator.Email {
		t.Fatalf("unexpected admin contact: %#v", adminContact)
	}

	defaultCtx, defaultRecorder := quotaPoolTestContext(t, http.MethodGet, "/api/quota_pool/self", nil, common.RoleCommonUser, defaultMember.Id)
	GetSelfQuotaPool(defaultCtx)
	defaultResponse := decodeQuotaPoolResponse(t, defaultRecorder)
	if defaultResponse["success"] != true {
		t.Fatalf("expected default member success response, got %#v", defaultResponse)
	}
	defaultData := defaultResponse["data"].(map[string]interface{})
	if defaultData["pool"] != nil {
		t.Fatalf("default pool member should not receive pool info, got %#v", defaultData["pool"])
	}
}

func TestUpdateSelfQuotaPoolOnlyUpdatesRechargeRules(t *testing.T) {
	db := setupQuotaPoolControllerTestDB(t)
	cfg := operation_setting.GetAutoRechargeSetting()
	originalAmount := cfg.Amount
	originalThreshold := cfg.Threshold
	defer func() {
		cfg.Amount = originalAmount
		cfg.Threshold = originalThreshold
	}()
	cfg.Amount = 200
	cfg.Threshold = 50

	unit := int(common.QuotaPerUnit)
	pool := &model.QuotaPool{
		Name:                 "team",
		Enabled:              true,
		BaseQuota:            1000,
		Quota:                1000,
		AutoRechargeAmount:   model.QuotaPoolAutoRechargeInherit,
		WeeklyLimit:          model.QuotaPoolAutoRechargeInherit,
		MonthlyLimit:         model.QuotaPoolAutoRechargeInherit,
		MonthlyRefillEnabled: false,
		MonthlyRefillAmount:  100,
		MonthlyRefillDay:     1,
	}
	if err := db.Create(pool).Error; err != nil {
		t.Fatalf("create pool failed: %v", err)
	}
	operator := &model.User{Id: 1, Username: "operator", Password: "password", Role: common.RoleCommonUser, Status: common.UserStatusEnabled, QuotaPoolId: pool.Id, AffCode: "operator-code"}
	if err := db.Create(operator).Error; err != nil {
		t.Fatalf("create operator failed: %v", err)
	}
	if err := db.Create(&model.QuotaPoolAdmin{PoolId: pool.Id, UserId: operator.Id, Level: model.QuotaPoolAdminLevelV1}).Error; err != nil {
		t.Fatalf("create operator admin failed: %v", err)
	}
	name := "renamed"
	autoRechargeAmount := float64(300)
	baseQuota := float64(9)
	weeklyLimit := 3
	monthlyLimit := 4
	monthlyRefillEnabled := true
	monthlyRefillAmount := float64(9)
	monthlyRefillDay := 7
	ctx, recorder := quotaPoolTestContext(t, http.MethodPut, "/api/quota_pool/self", quotaPoolUpdateRequest{
		Name:                 &name,
		BaseQuota:            &baseQuota,
		AutoRechargeAmount:   &autoRechargeAmount,
		WeeklyLimit:          &weeklyLimit,
		MonthlyLimit:         &monthlyLimit,
		MonthlyRefillEnabled: &monthlyRefillEnabled,
		MonthlyRefillAmount:  &monthlyRefillAmount,
		MonthlyRefillDay:     &monthlyRefillDay,
	}, common.RoleCommonUser, operator.Id)

	UpdateSelfQuotaPool(ctx)

	response := decodeQuotaPoolResponse(t, recorder)
	if response["success"] != true {
		t.Fatalf("expected success response, got %#v", response)
	}
	if response["message"] != "" {
		t.Fatalf("expected no warning for amount within risk limit, got %q", response["message"])
	}
	var got model.QuotaPool
	if err := db.First(&got, pool.Id).Error; err != nil {
		t.Fatalf("load pool failed: %v", err)
	}
	if got.AutoRechargeAmount != 300*unit || got.WeeklyLimit != 3 || got.MonthlyLimit != 4 {
		t.Fatalf("recharge rules not updated: %+v", got)
	}
	if got.Name != "team" || got.BaseQuota != 1000 || got.Quota != 1000 || got.MonthlyRefillEnabled || got.MonthlyRefillAmount != 100 || got.MonthlyRefillDay != 1 {
		t.Fatalf("self update should not change protected fields: %+v", got)
	}
}

func TestUpdateSelfQuotaPoolRejectsAutoRechargeAmountNotAboveThreshold(t *testing.T) {
	db := setupQuotaPoolControllerTestDB(t)
	cfg := operation_setting.GetAutoRechargeSetting()
	originalAmount := cfg.Amount
	originalThreshold := cfg.Threshold
	defer func() {
		cfg.Amount = originalAmount
		cfg.Threshold = originalThreshold
	}()
	cfg.Amount = 200
	cfg.Threshold = 50

	pool := &model.QuotaPool{Name: "team", Enabled: true, BaseQuota: 1000, Quota: 1000, AutoRechargeAmount: model.QuotaPoolAutoRechargeInherit, MonthlyRefillDay: 1}
	if err := db.Create(pool).Error; err != nil {
		t.Fatalf("create pool failed: %v", err)
	}
	operator := &model.User{Id: 1, Username: "operator", Password: "password", Role: common.RoleCommonUser, Status: common.UserStatusEnabled, QuotaPoolId: pool.Id, AffCode: "operator-code"}
	if err := db.Create(operator).Error; err != nil {
		t.Fatalf("create operator failed: %v", err)
	}
	if err := db.Create(&model.QuotaPoolAdmin{PoolId: pool.Id, UserId: operator.Id, Level: model.QuotaPoolAdminLevelV1}).Error; err != nil {
		t.Fatalf("create operator admin failed: %v", err)
	}
	autoRechargeAmount := float64(50)
	ctx, recorder := quotaPoolTestContext(t, http.MethodPut, "/api/quota_pool/self", quotaPoolUpdateRequest{
		AutoRechargeAmount: &autoRechargeAmount,
	}, common.RoleCommonUser, operator.Id)

	UpdateSelfQuotaPool(ctx)

	response := decodeQuotaPoolResponse(t, recorder)
	if response["success"] == true {
		t.Fatalf("expected update to be rejected")
	}
	if !strings.Contains(response["message"].(string), "自动充值金额必须大于触发充值金额") {
		t.Fatalf("unexpected message: %s", response["message"].(string))
	}
	var got model.QuotaPool
	if err := db.First(&got, pool.Id).Error; err != nil {
		t.Fatalf("load pool failed: %v", err)
	}
	if got.AutoRechargeAmount != model.QuotaPoolAutoRechargeInherit {
		t.Fatalf("auto recharge amount should be unchanged, got %d", got.AutoRechargeAmount)
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

func TestRevokeSelfQuotaPoolAdminRejectsAdminRevoke(t *testing.T) {
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
	if err := db.Create(&model.QuotaPoolAdmin{PoolId: pool.Id, UserId: operator.Id, Level: model.QuotaPoolAdminLevelV1}).Error; err != nil {
		t.Fatalf("create operator admin failed: %v", err)
	}
	if err := db.Create(&model.QuotaPoolAdmin{PoolId: pool.Id, UserId: target.Id, Level: model.QuotaPoolAdminLevelV1}).Error; err != nil {
		t.Fatalf("create target admin failed: %v", err)
	}

	ctx, recorder := quotaPoolTestContext(t, http.MethodDelete, fmt.Sprintf("/api/quota_pool/self/admins/%d", target.Id), nil, common.RoleCommonUser, operator.Id)
	ctx.Params = gin.Params{{Key: "user_id", Value: fmt.Sprintf("%d", target.Id)}}

	RevokeSelfQuotaPoolAdmin(ctx)

	response := decodeQuotaPoolResponse(t, recorder)
	if response["success"] == true {
		t.Fatalf("expected self revoke admin to fail")
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
		Level:  model.QuotaPoolAdminLevelV1,
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
	if !strings.Contains(logs[0].Content, "任命") || !strings.Contains(logs[0].Content, "池管理员") {
		t.Fatalf("unexpected grant log: %q", logs[0].Content)
	}
	if !strings.Contains(logs[1].Content, "撤销额度池管理员") {
		t.Fatalf("unexpected revoke log: %q", logs[1].Content)
	}
}

func TestQuotaPoolSuperAdminGrantMovesDefaultUserToPool(t *testing.T) {
	db := setupQuotaPoolControllerTestDB(t)
	cfg := operation_setting.GetAutoRechargeSetting()
	originalAmount := cfg.Amount
	defer func() {
		cfg.Amount = originalAmount
	}()
	cfg.Amount = 0
	pool := &model.QuotaPool{Name: "team", Enabled: true, BaseQuota: 100, Quota: 100, MonthlyRefillDay: 1}
	if err := db.Create(pool).Error; err != nil {
		t.Fatalf("create pool failed: %v", err)
	}
	target := &model.User{Id: 2, Username: "target", Password: "password", Role: common.RoleCommonUser, Status: common.UserStatusEnabled, Quota: 123, QuotaPoolId: model.QuotaPoolDefaultUserPoolId, AffCode: "target-code"}
	if err := db.Create(target).Error; err != nil {
		t.Fatalf("create target failed: %v", err)
	}

	ctx, recorder := quotaPoolTestContext(t, http.MethodPost, fmt.Sprintf("/api/quota_pool/%d/admins", pool.Id), quotaPoolAdminRequest{
		UserId: target.Id,
		Level:  model.QuotaPoolAdminLevelV1,
	}, common.RoleQuotaPoolSuperAdmin, 99)
	ctx.Params = gin.Params{{Key: "id", Value: fmt.Sprintf("%d", pool.Id)}}
	GrantQuotaPoolAdmin(ctx)

	if response := decodeQuotaPoolResponse(t, recorder); response["success"] != true {
		t.Fatalf("expected quota pool super admin grant success, got %#v", response)
	}
	var gotUser model.User
	_ = db.First(&gotUser, target.Id).Error
	if gotUser.QuotaPoolId != pool.Id {
		t.Fatalf("user quota_pool_id = %d, want %d", gotUser.QuotaPoolId, pool.Id)
	}
	var admin model.QuotaPoolAdmin
	if err := db.First(&admin, "user_id = ?", target.Id).Error; err != nil {
		t.Fatalf("load quota pool admin failed: %v", err)
	}
	if admin.Level != model.QuotaPoolAdminLevelV1 {
		t.Fatalf("admin level = %d, want v1", admin.Level)
	}
}

func TestQuotaPoolSuperAdminCanAddMemberButCannotRefillPool(t *testing.T) {
	db := setupQuotaPoolControllerTestDB(t)
	cfg := operation_setting.GetAutoRechargeSetting()
	originalAmount := cfg.Amount
	defer func() {
		cfg.Amount = originalAmount
	}()
	cfg.Amount = 0

	pool := &model.QuotaPool{Name: "team", Enabled: true, BaseQuota: 100, Quota: 100, MonthlyRefillDay: 1}
	if err := db.Create(pool).Error; err != nil {
		t.Fatalf("create pool failed: %v", err)
	}
	target := &model.User{Id: 2, Username: "target", Password: "password", Role: common.RoleCommonUser, Status: common.UserStatusEnabled, QuotaPoolId: model.QuotaPoolDefaultUserPoolId, AffCode: "target-code"}
	if err := db.Create(target).Error; err != nil {
		t.Fatalf("create target failed: %v", err)
	}

	addCtx, addRecorder := quotaPoolTestContext(t, http.MethodPost, fmt.Sprintf("/api/quota_pool/%d/members", pool.Id), quotaPoolMemberRequest{
		UserId: target.Id,
	}, common.RoleQuotaPoolSuperAdmin, 99)
	addCtx.Params = gin.Params{{Key: "id", Value: fmt.Sprintf("%d", pool.Id)}}
	AddQuotaPoolMember(addCtx)
	if response := decodeQuotaPoolResponse(t, addRecorder); response["success"] != true {
		t.Fatalf("expected quota pool super admin add member success, got %#v", response)
	}
	var gotUser model.User
	if err := db.First(&gotUser, target.Id).Error; err != nil {
		t.Fatalf("load target failed: %v", err)
	}
	if gotUser.QuotaPoolId != pool.Id {
		t.Fatalf("target quota_pool_id = %d, want %d", gotUser.QuotaPoolId, pool.Id)
	}

	refillCtx, refillRecorder := quotaPoolTestContext(t, http.MethodPost, fmt.Sprintf("/api/quota_pool/%d/refill", pool.Id), quotaPoolRefillRequest{
		Amount: 1,
	}, common.RoleQuotaPoolSuperAdmin, 99)
	refillCtx.Params = gin.Params{{Key: "id", Value: fmt.Sprintf("%d", pool.Id)}}
	RefillQuotaPool(refillCtx)
	if response := decodeQuotaPoolResponse(t, refillRecorder); response["success"] == true {
		t.Fatalf("expected quota pool super admin refill to fail")
	}
}

func TestQuotaPoolSuperAdminCanRechargeAndReclaimMembers(t *testing.T) {
	db := setupQuotaPoolControllerTestDB(t)
	cfg := operation_setting.GetAutoRechargeSetting()
	originalEnabled := cfg.Enabled
	originalThreshold := cfg.Threshold
	defer func() {
		cfg.Enabled = originalEnabled
		cfg.Threshold = originalThreshold
	}()
	cfg.Enabled = false
	cfg.Threshold = -1

	unit := int(common.QuotaPerUnit)
	pool := &model.QuotaPool{Name: "team", Enabled: true, BaseQuota: 10 * unit, Quota: 10 * unit, AutoRechargeAmount: unit, MonthlyRefillDay: 1}
	if err := db.Create(pool).Error; err != nil {
		t.Fatalf("create pool failed: %v", err)
	}
	user := &model.User{Id: 2, Username: "member", Password: "password", Role: common.RoleCommonUser, Status: common.UserStatusEnabled, QuotaPoolId: pool.Id, AffCode: "member-code"}
	if err := db.Create(user).Error; err != nil {
		t.Fatalf("create user failed: %v", err)
	}

	rechargeCtx, rechargeRecorder := quotaPoolTestContext(t, http.MethodPost, fmt.Sprintf("/api/quota_pool/%d/members/%d/recharge", pool.Id, user.Id), nil, common.RoleQuotaPoolSuperAdmin, 99)
	rechargeCtx.Params = gin.Params{{Key: "id", Value: fmt.Sprintf("%d", pool.Id)}, {Key: "user_id", Value: fmt.Sprintf("%d", user.Id)}}
	RechargeQuotaPoolMember(rechargeCtx)
	if response := decodeQuotaPoolResponse(t, rechargeRecorder); response["success"] != true {
		t.Fatalf("expected quota pool super admin recharge success, got %#v", response)
	}

	reclaimCtx, reclaimRecorder := quotaPoolTestContext(t, http.MethodPost, fmt.Sprintf("/api/quota_pool/%d/members/%d/reclaim", pool.Id, user.Id), nil, common.RoleQuotaPoolSuperAdmin, 99)
	reclaimCtx.Params = gin.Params{{Key: "id", Value: fmt.Sprintf("%d", pool.Id)}, {Key: "user_id", Value: fmt.Sprintf("%d", user.Id)}}
	ReclaimQuotaPoolMember(reclaimCtx)
	if response := decodeQuotaPoolResponse(t, reclaimRecorder); response["success"] != true {
		t.Fatalf("expected quota pool super admin reclaim success, got %#v", response)
	}
	var logs []model.Log
	if err := db.Where("user_id = ? AND type = ?", user.Id, model.LogTypeManage).Order("id asc").Find(&logs).Error; err != nil {
		t.Fatalf("load quota pool recharge logs failed: %v", err)
	}
	if len(logs) < 2 {
		t.Fatalf("expected recharge and reclaim logs, got %d", len(logs))
	}
	if !strings.Contains(logs[0].Content, "池管理员(ID:99)添加") {
		t.Fatalf("unexpected recharge log content: %q", logs[0].Content)
	}
	if strings.Contains(logs[0].Content, "额度池管理员") {
		t.Fatalf("recharge log should use short pool admin label: %q", logs[0].Content)
	}
	if !strings.Contains(logs[1].Content, "池管理员(ID:99)减少") {
		t.Fatalf("unexpected reclaim log content: %q", logs[1].Content)
	}
	if strings.Contains(logs[1].Content, "额度池管理员") {
		t.Fatalf("reclaim log should use short pool admin label: %q", logs[1].Content)
	}
	operationLogs, total, err := model.ListQuotaPoolOperationLogs(pool.Id, &common.PageInfo{Page: 1, PageSize: 10}, nil)
	if err != nil {
		t.Fatalf("list quota pool operation logs failed: %v", err)
	}
	if total < 2 || len(operationLogs) < 2 {
		t.Fatalf("expected quota pool operation logs, got total=%d len=%d", total, len(operationLogs))
	}
	if !strings.Contains(operationLogs[1].Content, "池管理员(ID:99)添加") {
		t.Fatalf("unexpected operation recharge log content: %q", operationLogs[1].Content)
	}
	if strings.Contains(operationLogs[1].Content, "额度池管理员") {
		t.Fatalf("operation recharge log should use short pool admin label: %q", operationLogs[1].Content)
	}
	if !strings.Contains(operationLogs[0].Content, "池管理员(ID:99)减少") {
		t.Fatalf("unexpected operation reclaim log content: %q", operationLogs[0].Content)
	}
	if strings.Contains(operationLogs[0].Content, "额度池管理员") {
		t.Fatalf("operation reclaim log should use short pool admin label: %q", operationLogs[0].Content)
	}

}

func TestQuotaPoolSuperAdminCanMoveUserToNormalPool(t *testing.T) {
	db := setupQuotaPoolControllerTestDB(t)
	sourcePool := &model.QuotaPool{Name: "source", Enabled: true, BaseQuota: 1000, Quota: 1000, MonthlyRefillDay: 1}
	targetPool := &model.QuotaPool{Name: "target", Enabled: true, BaseQuota: 1000, Quota: 1000, MonthlyRefillDay: 1}
	if err := db.Create(sourcePool).Error; err != nil {
		t.Fatalf("create source pool failed: %v", err)
	}
	if err := db.Create(targetPool).Error; err != nil {
		t.Fatalf("create target pool failed: %v", err)
	}
	user := &model.User{Id: 2, Username: "member", Password: "password", Role: common.RoleCommonUser, Status: common.UserStatusEnabled, Quota: 120, QuotaPoolId: sourcePool.Id, AffCode: "member-code"}
	if err := db.Create(user).Error; err != nil {
		t.Fatalf("create user failed: %v", err)
	}

	ctx, recorder := quotaPoolTestContext(t, http.MethodPut, fmt.Sprintf("/api/quota_pool/users/%d", user.Id), quotaPoolMoveRequest{
		PoolId: targetPool.Id,
	}, common.RoleQuotaPoolSuperAdmin, 99)
	ctx.Params = gin.Params{{Key: "user_id", Value: fmt.Sprintf("%d", user.Id)}}
	MoveUserQuotaPool(ctx)

	response := decodeQuotaPoolResponse(t, recorder)
	if response["success"] != true {
		t.Fatalf("expected quota pool super admin move success, got %#v", response)
	}
	var gotUser model.User
	if err := db.First(&gotUser, user.Id).Error; err != nil {
		t.Fatalf("load user failed: %v", err)
	}
	if gotUser.QuotaPoolId != targetPool.Id || gotUser.Quota != 0 {
		t.Fatalf("unexpected user after move: quota=%d pool=%d", gotUser.Quota, gotUser.QuotaPoolId)
	}
	newUserPool, err := model.SyncNewUserQuotaPool()
	if err != nil {
		t.Fatalf("sync new user pool failed: %v", err)
	}
	ctx, recorder = quotaPoolTestContext(t, http.MethodPut, fmt.Sprintf("/api/quota_pool/users/%d", user.Id), quotaPoolMoveRequest{
		PoolId: newUserPool.Id,
	}, common.RoleQuotaPoolSuperAdmin, 99)
	ctx.Params = gin.Params{{Key: "user_id", Value: fmt.Sprintf("%d", user.Id)}}
	MoveUserQuotaPool(ctx)
	if response := decodeQuotaPoolResponse(t, recorder); response["success"] != true {
		t.Fatalf("expected quota pool super admin move to default pool success, got %#v", response)
	}
	if err := db.First(&gotUser, user.Id).Error; err != nil {
		t.Fatalf("reload user failed: %v", err)
	}
	if gotUser.QuotaPoolId != newUserPool.Id {
		t.Fatalf("user quota_pool_id = %d, want %d", gotUser.QuotaPoolId, newUserPool.Id)
	}
}

func TestQuotaPoolSuperAdminCannotMoveToLegacyDefaultPoolAlias(t *testing.T) {
	db := setupQuotaPoolControllerTestDB(t)
	sourcePool := &model.QuotaPool{Name: "source", Enabled: true, BaseQuota: 1000, Quota: 1000, MonthlyRefillDay: 1}
	if err := db.Create(sourcePool).Error; err != nil {
		t.Fatalf("create source pool failed: %v", err)
	}
	legacyDefaultPool, err := model.SyncDefaultQuotaPool()
	if err != nil {
		t.Fatalf("sync legacy default pool failed: %v", err)
	}
	user := &model.User{Id: 2, Username: "member", Password: "password", Role: common.RoleCommonUser, Status: common.UserStatusEnabled, Quota: 120, QuotaPoolId: sourcePool.Id, AffCode: "member-code"}
	if err := db.Create(user).Error; err != nil {
		t.Fatalf("create user failed: %v", err)
	}

	for _, targetPoolId := range []int{legacyDefaultPool.Id, -1} {
		ctx, recorder := quotaPoolTestContext(t, http.MethodPut, fmt.Sprintf("/api/quota_pool/users/%d", user.Id), quotaPoolMoveRequest{
			PoolId: targetPoolId,
		}, common.RoleQuotaPoolSuperAdmin, 99)
		ctx.Params = gin.Params{{Key: "user_id", Value: fmt.Sprintf("%d", user.Id)}}
		MoveUserQuotaPool(ctx)
		if response := decodeQuotaPoolResponse(t, recorder); response["success"] == true {
			t.Fatalf("expected legacy default target %d to be rejected", targetPoolId)
		}
	}
	var gotUser model.User
	if err := db.First(&gotUser, user.Id).Error; err != nil {
		t.Fatalf("load user failed: %v", err)
	}
	if gotUser.QuotaPoolId != sourcePool.Id || gotUser.Quota != 120 {
		t.Fatalf("user changed after rejected moves: quota=%d pool=%d", gotUser.Quota, gotUser.QuotaPoolId)
	}
}

func TestPoolAdminCanMoveMemberToDefaultPool(t *testing.T) {
	db := setupQuotaPoolControllerTestDB(t)
	sourcePool := &model.QuotaPool{Name: "source", Enabled: true, BaseQuota: 1000, Quota: 1000, MonthlyRefillDay: 1}
	if err := db.Create(sourcePool).Error; err != nil {
		t.Fatalf("create source pool failed: %v", err)
	}
	newUserPool, err := model.SyncNewUserQuotaPool()
	if err != nil {
		t.Fatalf("sync new user pool failed: %v", err)
	}
	admin := &model.User{Id: 1, Username: "pool-admin", Password: "password", Role: common.RoleCommonUser, Status: common.UserStatusEnabled, QuotaPoolId: sourcePool.Id, AffCode: "pool-admin-code"}
	user := &model.User{Id: 2, Username: "member", Password: "password", Role: common.RoleCommonUser, Status: common.UserStatusEnabled, Quota: 120, QuotaPoolId: sourcePool.Id, AffCode: "member-code"}
	if err := db.Create(admin).Error; err != nil {
		t.Fatalf("create pool admin failed: %v", err)
	}
	if err := db.Create(user).Error; err != nil {
		t.Fatalf("create member failed: %v", err)
	}
	if err := db.Create(&model.QuotaPoolAdmin{PoolId: sourcePool.Id, UserId: admin.Id, Level: model.QuotaPoolAdminLevelV1}).Error; err != nil {
		t.Fatalf("create pool admin relation failed: %v", err)
	}

	ctx, recorder := quotaPoolTestContext(t, http.MethodPut, fmt.Sprintf("/api/quota_pool/self/members/%d", user.Id), quotaPoolMoveRequest{
		PoolId: newUserPool.Id,
	}, common.RoleCommonUser, admin.Id)
	ctx.Params = gin.Params{{Key: "user_id", Value: fmt.Sprintf("%d", user.Id)}}
	MoveSelfQuotaPoolMember(ctx)

	response := decodeQuotaPoolResponse(t, recorder)
	if response["success"] != true {
		t.Fatalf("expected pool admin default-pool move success, got %#v", response)
	}
	var gotUser model.User
	if err := db.First(&gotUser, user.Id).Error; err != nil {
		t.Fatalf("load member failed: %v", err)
	}
	if gotUser.QuotaPoolId != newUserPool.Id || gotUser.Quota != 0 {
		t.Fatalf("unexpected member after default-pool move: quota=%d pool=%d", gotUser.Quota, gotUser.QuotaPoolId)
	}
	targetPool := &model.QuotaPool{Name: "target", Enabled: true, BaseQuota: 1000, Quota: 1000, MonthlyRefillDay: 1}
	if err := db.Create(targetPool).Error; err != nil {
		t.Fatalf("create target pool failed: %v", err)
	}
	secondUser := &model.User{Id: 3, Username: "second-member", Password: "password", Role: common.RoleCommonUser, Status: common.UserStatusEnabled, Quota: 80, QuotaPoolId: sourcePool.Id, AffCode: "second-member-code"}
	if err := db.Create(secondUser).Error; err != nil {
		t.Fatalf("create second member failed: %v", err)
	}
	ctx, recorder = quotaPoolTestContext(t, http.MethodPut, fmt.Sprintf("/api/quota_pool/self/members/%d", secondUser.Id), quotaPoolMoveRequest{
		PoolId: targetPool.Id,
	}, common.RoleCommonUser, admin.Id)
	ctx.Params = gin.Params{{Key: "user_id", Value: fmt.Sprintf("%d", secondUser.Id)}}
	MoveSelfQuotaPoolMember(ctx)
	if response := decodeQuotaPoolResponse(t, recorder); response["success"] == true {
		t.Fatalf("expected pool admin move to normal pool to be rejected")
	}
}

func TestSystemAdminCanMoveAnyUserToNewUserPool(t *testing.T) {
	db := setupQuotaPoolControllerTestDB(t)
	oldPool := &model.QuotaPool{Name: "source", Enabled: true, BaseQuota: 1000, Quota: 500, MonthlyRefillDay: 1}
	if err := db.Create(oldPool).Error; err != nil {
		t.Fatalf("create source pool failed: %v", err)
	}
	newUserPool, err := model.SyncNewUserQuotaPool()
	if err != nil {
		t.Fatalf("sync new user pool failed: %v", err)
	}
	target := &model.User{
		Id:          2,
		Username:    "root-target",
		Password:    "password",
		Role:        common.RoleRootUser,
		Status:      common.UserStatusDisabled,
		Quota:       120,
		QuotaPoolId: oldPool.Id,
		AffCode:     "root-target-code",
	}
	if err := db.Create(target).Error; err != nil {
		t.Fatalf("create target user failed: %v", err)
	}

	ctx, recorder := quotaPoolTestContext(t, http.MethodPut, fmt.Sprintf("/api/quota_pool/users/%d", target.Id), quotaPoolMoveRequest{
		PoolId: newUserPool.Id,
	}, common.RoleAdminUser, 99)
	ctx.Params = gin.Params{{Key: "user_id", Value: fmt.Sprintf("%d", target.Id)}}
	MoveUserQuotaPool(ctx)

	response := decodeQuotaPoolResponse(t, recorder)
	if response["success"] != true {
		t.Fatalf("expected system admin move success, got %#v", response)
	}
	var gotUser model.User
	if err := db.First(&gotUser, target.Id).Error; err != nil {
		t.Fatalf("load target user failed: %v", err)
	}
	if gotUser.Quota != 0 || gotUser.QuotaPoolId != newUserPool.Id {
		t.Fatalf("unexpected target user after move: quota=%d pool=%d", gotUser.Quota, gotUser.QuotaPoolId)
	}
}

func TestQuotaPoolSuperAdminGrantAndRevokeWriteManageLogs(t *testing.T) {
	db := setupQuotaPoolControllerTestDB(t)
	pool := &model.QuotaPool{Name: "team", Enabled: true, BaseQuota: 100, Quota: 100, MonthlyRefillDay: 1}
	if err := db.Create(pool).Error; err != nil {
		t.Fatalf("create pool failed: %v", err)
	}
	operator := &model.User{Id: 1, Username: "operator", Password: "password", Role: common.RoleQuotaPoolSuperAdmin, Status: common.UserStatusEnabled, QuotaPoolId: model.QuotaPoolDefaultUserPoolId, AffCode: "operator-code"}
	target := &model.User{Id: 2, Username: "target", Password: "password", Role: common.RoleCommonUser, Status: common.UserStatusEnabled, QuotaPoolId: pool.Id, AffCode: "target-code"}
	if err := db.Create(operator).Error; err != nil {
		t.Fatalf("create operator failed: %v", err)
	}
	if err := db.Create(target).Error; err != nil {
		t.Fatalf("create target failed: %v", err)
	}

	grantCtx, grantRecorder := quotaPoolTestContext(t, http.MethodPost, fmt.Sprintf("/api/quota_pool/%d/admins", pool.Id), quotaPoolAdminRequest{
		UserId: target.Id,
		Level:  model.QuotaPoolAdminLevelV1,
	}, common.RoleQuotaPoolSuperAdmin, operator.Id)
	grantCtx.Params = gin.Params{{Key: "id", Value: fmt.Sprintf("%d", pool.Id)}}
	GrantQuotaPoolAdmin(grantCtx)
	if response := decodeQuotaPoolResponse(t, grantRecorder); response["success"] != true {
		t.Fatalf("expected quota pool super admin grant success, got %#v", response)
	}

	revokeCtx, revokeRecorder := quotaPoolTestContext(t, http.MethodDelete, fmt.Sprintf("/api/quota_pool/%d/admins/%d", pool.Id, target.Id), nil, common.RoleQuotaPoolSuperAdmin, operator.Id)
	revokeCtx.Params = gin.Params{{Key: "id", Value: fmt.Sprintf("%d", pool.Id)}, {Key: "user_id", Value: fmt.Sprintf("%d", target.Id)}}
	RevokeQuotaPoolAdmin(revokeCtx)
	if response := decodeQuotaPoolResponse(t, revokeRecorder); response["success"] != true {
		t.Fatalf("expected quota pool super admin revoke success, got %#v", response)
	}

	var logs []model.Log
	if err := db.Where("user_id = ? AND type = ?", target.Id, model.LogTypeManage).Order("id asc").Find(&logs).Error; err != nil {
		t.Fatalf("load manage logs failed: %v", err)
	}
	if len(logs) != 2 {
		t.Fatalf("expected two manage logs, got %d", len(logs))
	}
	if !strings.Contains(logs[0].Content, "任命") || !strings.Contains(logs[0].Content, "池管理员") {
		t.Fatalf("unexpected grant log: %q", logs[0].Content)
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

func TestAddQuotaPoolMemberDefaultsSystemAdminToV1(t *testing.T) {
	db := setupQuotaPoolControllerTestDB(t)
	pool := &model.QuotaPool{Name: "team-admin-member", Enabled: true, BaseQuota: 1000, Quota: 1000, MonthlyRefillDay: 1}
	if err := db.Create(pool).Error; err != nil {
		t.Fatalf("create pool failed: %v", err)
	}
	user := &model.User{
		Username: "system-admin-member",
		Password: "password",
		Role:     common.RoleAdminUser,
		Status:   common.UserStatusEnabled,
		Quota:    123,
		AffCode:  "system-admin-member-code",
	}
	if err := db.Create(user).Error; err != nil {
		t.Fatalf("create system admin user failed: %v", err)
	}

	addCtx, addRecorder := quotaPoolTestContext(t, http.MethodPost, fmt.Sprintf("/api/quota_pool/%d/members", pool.Id), quotaPoolMemberRequest{
		UserId: user.Id,
	}, common.RoleAdminUser, 99)
	addCtx.Params = gin.Params{{Key: "id", Value: fmt.Sprintf("%d", pool.Id)}}
	AddQuotaPoolMember(addCtx)
	if response := decodeQuotaPoolResponse(t, addRecorder); response["success"] != true {
		t.Fatalf("expected add system admin success, got %#v", response)
	}

	var gotUser model.User
	_ = db.First(&gotUser, user.Id).Error
	if gotUser.QuotaPoolId != pool.Id {
		t.Fatalf("user quota_pool_id = %d, want %d", gotUser.QuotaPoolId, pool.Id)
	}
	var admin model.QuotaPoolAdmin
	if err := db.First(&admin, "user_id = ?", user.Id).Error; err != nil {
		t.Fatalf("load quota pool admin failed: %v", err)
	}
	if admin.Level != model.QuotaPoolAdminLevelV1 {
		t.Fatalf("admin level = %d, want v1", admin.Level)
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

func TestReclaimQuotaPoolMemberCreditsPoolAndDebitsUser(t *testing.T) {
	db := setupQuotaPoolControllerTestDB(t)
	cfg := operation_setting.GetAutoRechargeSetting()
	originalEnabled := cfg.Enabled
	originalThreshold := cfg.Threshold
	defer func() {
		cfg.Enabled = originalEnabled
		cfg.Threshold = originalThreshold
	}()
	cfg.Enabled = true
	cfg.Threshold = 1

	unit := int(common.QuotaPerUnit)
	pool := &model.QuotaPool{Name: "team", Enabled: true, BaseQuota: 10 * unit, Quota: 8 * unit, AutoRechargeAmount: 2 * unit, MonthlyRefillDay: 1}
	if err := db.Create(pool).Error; err != nil {
		t.Fatalf("create pool failed: %v", err)
	}
	user := &model.User{Username: "member", Password: "password", Role: common.RoleCommonUser, Status: common.UserStatusEnabled, Quota: 5 * unit, QuotaPoolId: pool.Id, AffCode: "member-code"}
	if err := db.Create(user).Error; err != nil {
		t.Fatalf("create user failed: %v", err)
	}
	ctx, recorder := quotaPoolTestContext(t, http.MethodPost, fmt.Sprintf("/api/quota_pool/%d/members/%d/reclaim", pool.Id, user.Id), nil, common.RoleAdminUser, 99)
	ctx.Params = gin.Params{{Key: "id", Value: fmt.Sprintf("%d", pool.Id)}, {Key: "user_id", Value: fmt.Sprintf("%d", user.Id)}}

	ReclaimQuotaPoolMember(ctx)

	response := decodeQuotaPoolResponse(t, recorder)
	if response["success"] != true {
		t.Fatalf("expected reclaim to succeed, got %#v", response)
	}
	var gotPool model.QuotaPool
	_ = db.First(&gotPool, pool.Id).Error
	if gotPool.Quota != 10*unit {
		t.Fatalf("pool quota = %d, want %d", gotPool.Quota, 10*unit)
	}
	var gotUser model.User
	_ = db.First(&gotUser, user.Id).Error
	if gotUser.Quota != 3*unit {
		t.Fatalf("user quota = %d, want %d", gotUser.Quota, 3*unit)
	}
	var tx model.QuotaPoolTransaction
	if err := db.First(&tx, "pool_id = ? AND type = ?", pool.Id, model.QuotaPoolTransactionReclaimUser).Error; err != nil {
		t.Fatalf("load reclaim transaction failed: %v", err)
	}
	if tx.Amount != 2*unit || tx.QuotaBefore != 8*unit || tx.QuotaAfter != 10*unit || tx.UserId != user.Id || tx.OperatorId != 99 {
		t.Fatalf("unexpected reclaim transaction: %+v", tx)
	}
}

func TestReclaimQuotaPoolMemberRejectsWhenResultWouldTriggerAutoRecharge(t *testing.T) {
	db := setupQuotaPoolControllerTestDB(t)
	cfg := operation_setting.GetAutoRechargeSetting()
	originalEnabled := cfg.Enabled
	originalThreshold := cfg.Threshold
	defer func() {
		cfg.Enabled = originalEnabled
		cfg.Threshold = originalThreshold
	}()
	cfg.Enabled = true
	cfg.Threshold = 3

	unit := int(common.QuotaPerUnit)
	pool := &model.QuotaPool{Name: "team", Enabled: true, BaseQuota: 10 * unit, Quota: 8 * unit, AutoRechargeAmount: 2 * unit, MonthlyRefillDay: 1}
	if err := db.Create(pool).Error; err != nil {
		t.Fatalf("create pool failed: %v", err)
	}
	user := &model.User{Username: "member", Password: "password", Role: common.RoleCommonUser, Status: common.UserStatusEnabled, Quota: 5 * unit, QuotaPoolId: pool.Id, AffCode: "member-code"}
	if err := db.Create(user).Error; err != nil {
		t.Fatalf("create user failed: %v", err)
	}
	ctx, recorder := quotaPoolTestContext(t, http.MethodPost, fmt.Sprintf("/api/quota_pool/%d/members/%d/reclaim", pool.Id, user.Id), nil, common.RoleAdminUser, 99)
	ctx.Params = gin.Params{{Key: "id", Value: fmt.Sprintf("%d", pool.Id)}, {Key: "user_id", Value: fmt.Sprintf("%d", user.Id)}}

	ReclaimQuotaPoolMember(ctx)

	response := decodeQuotaPoolResponse(t, recorder)
	if response["success"] == true {
		t.Fatalf("expected reclaim to be rejected")
	}
	if !strings.Contains(response["message"].(string), "扣减后会触发自动充值") {
		t.Fatalf("unexpected response: %#v", response)
	}
	var gotPool model.QuotaPool
	_ = db.First(&gotPool, pool.Id).Error
	if gotPool.Quota != 8*unit {
		t.Fatalf("pool quota = %d, want %d", gotPool.Quota, 8*unit)
	}
	var gotUser model.User
	_ = db.First(&gotUser, user.Id).Error
	if gotUser.Quota != 5*unit {
		t.Fatalf("user quota = %d, want %d", gotUser.Quota, 5*unit)
	}
}

func TestUpdateQuotaPoolCanAdjustBaseQuota(t *testing.T) {
	db := setupQuotaPoolControllerTestDB(t)
	unit := int(common.QuotaPerUnit)
	pool := &model.QuotaPool{Name: "team", Enabled: true, BaseQuota: 10 * unit, Quota: 4 * unit, MonthlyRefillDay: 1}
	if err := db.Create(pool).Error; err != nil {
		t.Fatalf("create pool failed: %v", err)
	}
	baseQuota := float64(12)
	ctx, recorder := quotaPoolTestContext(t, http.MethodPut, fmt.Sprintf("/api/quota_pool/%d", pool.Id), quotaPoolUpdateRequest{
		BaseQuota: &baseQuota,
	}, common.RoleRootUser, 99)
	ctx.Params = gin.Params{{Key: "id", Value: fmt.Sprintf("%d", pool.Id)}}

	UpdateQuotaPool(ctx)

	response := decodeQuotaPoolResponse(t, recorder)
	if response["success"] != true {
		t.Fatalf("expected success response, got %#v", response)
	}
	var got model.QuotaPool
	if err := db.First(&got, pool.Id).Error; err != nil {
		t.Fatalf("load pool failed: %v", err)
	}
	if got.BaseQuota != 12*unit || got.Quota != 6*unit {
		t.Fatalf("pool quota = %d/%d, want %d/%d", got.Quota, got.BaseQuota, 6*unit, 12*unit)
	}
	var tx model.QuotaPoolTransaction
	if err := db.First(&tx, "pool_id = ? AND type = ?", pool.Id, model.QuotaPoolTransactionAdjustBase).Error; err != nil {
		t.Fatalf("load adjust transaction failed: %v", err)
	}
	if tx.Amount != 2*unit || tx.QuotaBefore != 4*unit || tx.QuotaAfter != 6*unit || tx.OperatorId != 99 {
		t.Fatalf("unexpected adjust transaction: %+v", tx)
	}
}

func TestUpdateQuotaPoolRejectsBaseQuotaBelowAllocatedQuota(t *testing.T) {
	db := setupQuotaPoolControllerTestDB(t)
	unit := int(common.QuotaPerUnit)
	pool := &model.QuotaPool{Name: "team", Enabled: true, BaseQuota: 1000 * unit, Quota: 300 * unit, MonthlyRefillDay: 1}
	if err := db.Create(pool).Error; err != nil {
		t.Fatalf("create pool failed: %v", err)
	}
	baseQuota := float64(600)
	ctx, recorder := quotaPoolTestContext(t, http.MethodPut, fmt.Sprintf("/api/quota_pool/%d", pool.Id), quotaPoolUpdateRequest{
		BaseQuota: &baseQuota,
	}, common.RoleRootUser, 99)
	ctx.Params = gin.Params{{Key: "id", Value: fmt.Sprintf("%d", pool.Id)}}

	UpdateQuotaPool(ctx)

	response := decodeQuotaPoolResponse(t, recorder)
	if response["success"] == true {
		t.Fatalf("expected base quota decrease to fail")
	}
	if !strings.Contains(response["message"].(string), "最多减少") || !strings.Contains(response["message"].(string), "300.000000") {
		t.Fatalf("unexpected message: %s", response["message"].(string))
	}
	var got model.QuotaPool
	if err := db.First(&got, pool.Id).Error; err != nil {
		t.Fatalf("load pool failed: %v", err)
	}
	if got.BaseQuota != 1000*unit || got.Quota != 300*unit {
		t.Fatalf("pool should be unchanged, got %d/%d", got.Quota, got.BaseQuota)
	}
}

func TestUpdateQuotaPoolAllowsRiskyAutoRechargeAmountWithWarning(t *testing.T) {
	db := setupQuotaPoolControllerTestDB(t)
	cfg := operation_setting.GetAutoRechargeSetting()
	originalAmount := cfg.Amount
	originalThreshold := cfg.Threshold
	defer func() {
		cfg.Amount = originalAmount
		cfg.Threshold = originalThreshold
	}()
	cfg.Amount = 200
	cfg.Threshold = 50

	pool := &model.QuotaPool{Name: "team", Enabled: true, BaseQuota: 1000, Quota: 1000, AutoRechargeAmount: model.QuotaPoolAutoRechargeInherit, MonthlyRefillDay: 1}
	if err := db.Create(pool).Error; err != nil {
		t.Fatalf("create pool failed: %v", err)
	}
	autoRechargeAmount := float64(601)
	ctx, recorder := quotaPoolTestContext(t, http.MethodPut, fmt.Sprintf("/api/quota_pool/%d", pool.Id), quotaPoolUpdateRequest{
		AutoRechargeAmount: &autoRechargeAmount,
	}, common.RoleRootUser, 99)
	ctx.Params = gin.Params{{Key: "id", Value: fmt.Sprintf("%d", pool.Id)}}

	UpdateQuotaPool(ctx)

	response := decodeQuotaPoolResponse(t, recorder)
	if response["success"] != true {
		t.Fatalf("expected update to succeed, got %#v", response)
	}
	if !strings.Contains(response["message"].(string), "可能存在较大风险") {
		t.Fatalf("unexpected message: %s", response["message"].(string))
	}
	var got model.QuotaPool
	if err := db.First(&got, pool.Id).Error; err != nil {
		t.Fatalf("load pool failed: %v", err)
	}
	if got.AutoRechargeAmount != int(601*common.QuotaPerUnit) {
		t.Fatalf("auto recharge amount should be updated, got %d", got.AutoRechargeAmount)
	}
}

func TestQuotaPoolSuperAdminCanUpdateRechargeRulesOnly(t *testing.T) {
	db := setupQuotaPoolControllerTestDB(t)
	cfg := operation_setting.GetAutoRechargeSetting()
	originalAmount := cfg.Amount
	originalThreshold := cfg.Threshold
	defer func() {
		cfg.Amount = originalAmount
		cfg.Threshold = originalThreshold
	}()
	cfg.Amount = 200
	cfg.Threshold = 50

	unit := int(common.QuotaPerUnit)
	pool := &model.QuotaPool{Name: "team", Enabled: true, BaseQuota: 10 * unit, Quota: 10 * unit, AutoRechargeAmount: model.QuotaPoolAutoRechargeInherit, WeeklyLimit: model.QuotaPoolAutoRechargeInherit, MonthlyLimit: model.QuotaPoolAutoRechargeInherit, MonthlyRefillDay: 1}
	if err := db.Create(pool).Error; err != nil {
		t.Fatalf("create pool failed: %v", err)
	}
	autoRechargeAmount := float64(100)
	weeklyLimit := 3
	monthlyLimit := 8
	ctx, recorder := quotaPoolTestContext(t, http.MethodPut, fmt.Sprintf("/api/quota_pool/%d", pool.Id), quotaPoolUpdateRequest{
		AutoRechargeAmount: &autoRechargeAmount,
		WeeklyLimit:        &weeklyLimit,
		MonthlyLimit:       &monthlyLimit,
	}, common.RoleQuotaPoolSuperAdmin, 99)
	ctx.Params = gin.Params{{Key: "id", Value: fmt.Sprintf("%d", pool.Id)}}

	UpdateQuotaPool(ctx)

	response := decodeQuotaPoolResponse(t, recorder)
	if response["success"] != true {
		t.Fatalf("expected quota pool super admin update success, got %#v", response)
	}
	var got model.QuotaPool
	if err := db.First(&got, pool.Id).Error; err != nil {
		t.Fatalf("load pool failed: %v", err)
	}
	if got.AutoRechargeAmount != 100*unit || got.WeeklyLimit != weeklyLimit || got.MonthlyLimit != monthlyLimit {
		t.Fatalf("unexpected recharge rules: amount=%d weekly=%d monthly=%d", got.AutoRechargeAmount, got.WeeklyLimit, got.MonthlyLimit)
	}
	assertQuotaPoolManageLogContains(t, db, 99, "修改额度池")
	assertQuotaPoolManageLogContains(t, db, 99, "充值金额")
	assertQuotaPoolManageLogContains(t, db, 99, "周自动充值次数")
	assertQuotaPoolManageLogContains(t, db, 99, "月自动充值次数")

	baseQuota := float64(20)
	rejectCtx, rejectRecorder := quotaPoolTestContext(t, http.MethodPut, fmt.Sprintf("/api/quota_pool/%d", pool.Id), quotaPoolUpdateRequest{
		BaseQuota: &baseQuota,
	}, common.RoleQuotaPoolSuperAdmin, 99)
	rejectCtx.Params = gin.Params{{Key: "id", Value: fmt.Sprintf("%d", pool.Id)}}
	UpdateQuotaPool(rejectCtx)
	rejectResponse := decodeQuotaPoolResponse(t, rejectRecorder)
	if rejectResponse["success"] == true {
		t.Fatalf("expected quota pool super admin base quota update to fail")
	}
	if !strings.Contains(rejectResponse["message"].(string), "无权限调整额度池总额度") {
		t.Fatalf("unexpected message: %s", rejectResponse["message"].(string))
	}
}

func TestSelfQuotaPoolAdminUpdateWritesManageLog(t *testing.T) {
	db := setupQuotaPoolControllerTestDB(t)
	cfg := operation_setting.GetAutoRechargeSetting()
	originalAmount := cfg.Amount
	originalThreshold := cfg.Threshold
	defer func() {
		cfg.Amount = originalAmount
		cfg.Threshold = originalThreshold
	}()
	cfg.Amount = 200
	cfg.Threshold = 50

	pool := &model.QuotaPool{Name: "self-config", Enabled: true, BaseQuota: 1000, Quota: 1000, AutoRechargeAmount: model.QuotaPoolAutoRechargeInherit, WeeklyLimit: model.QuotaPoolAutoRechargeInherit, MonthlyLimit: model.QuotaPoolAutoRechargeInherit, MonthlyRefillDay: 1}
	if err := db.Create(pool).Error; err != nil {
		t.Fatalf("create pool failed: %v", err)
	}
	operator := &model.User{Id: 7, Username: "pool-admin", Password: "password", Role: common.RoleCommonUser, Status: common.UserStatusEnabled, QuotaPoolId: pool.Id, AffCode: "admin-code"}
	if err := db.Create(operator).Error; err != nil {
		t.Fatalf("create operator failed: %v", err)
	}
	if err := db.Create(&model.QuotaPoolAdmin{PoolId: pool.Id, UserId: operator.Id, Level: model.QuotaPoolAdminLevelV1}).Error; err != nil {
		t.Fatalf("create admin failed: %v", err)
	}

	autoRechargeAmount := float64(100)
	weeklyLimit := 2
	ctx, recorder := quotaPoolTestContext(t, http.MethodPut, "/api/quota_pool/self", quotaPoolUpdateRequest{
		AutoRechargeAmount: &autoRechargeAmount,
		WeeklyLimit:        &weeklyLimit,
	}, common.RoleCommonUser, operator.Id)

	UpdateSelfQuotaPool(ctx)

	if response := decodeQuotaPoolResponse(t, recorder); response["success"] != true {
		t.Fatalf("expected self update success, got %#v", response)
	}
	assertQuotaPoolManageLogContains(t, db, operator.Id, "修改额度池")
	assertQuotaPoolManageLogContains(t, db, operator.Id, "充值金额")
}

func TestQuotaPoolLifecycleOperationsWriteManageLogs(t *testing.T) {
	db := setupQuotaPoolControllerTestDB(t)
	unit := int(common.QuotaPerUnit)
	pool := &model.QuotaPool{Name: "lifecycle", Enabled: false, BaseQuota: 10 * unit, Quota: 10 * unit, MonthlyRefillDay: 1}
	if err := db.Create(pool).Error; err != nil {
		t.Fatalf("create pool failed: %v", err)
	}

	enableCtx, enableRecorder := quotaPoolTestContext(t, http.MethodPost, fmt.Sprintf("/api/quota_pool/%d/enable", pool.Id), nil, common.RoleRootUser, 99)
	enableCtx.Params = gin.Params{{Key: "id", Value: fmt.Sprintf("%d", pool.Id)}}
	EnableQuotaPool(enableCtx)
	if response := decodeQuotaPoolResponse(t, enableRecorder); response["success"] != true {
		t.Fatalf("expected enable success, got %#v", response)
	}
	assertQuotaPoolManageLogContains(t, db, 99, "启用额度池")

	refillCtx, refillRecorder := quotaPoolTestContext(t, http.MethodPost, fmt.Sprintf("/api/quota_pool/%d/refill", pool.Id), quotaPoolRefillRequest{Amount: 1}, common.RoleAdminUser, 99)
	refillCtx.Params = gin.Params{{Key: "id", Value: fmt.Sprintf("%d", pool.Id)}}
	RefillQuotaPool(refillCtx)
	if response := decodeQuotaPoolResponse(t, refillRecorder); response["success"] != true {
		t.Fatalf("expected refill success, got %#v", response)
	}
	assertQuotaPoolManageLogContains(t, db, 99, "充值额度池")

	disableCtx, disableRecorder := quotaPoolTestContext(t, http.MethodPost, fmt.Sprintf("/api/quota_pool/%d/disable", pool.Id), nil, common.RoleRootUser, 99)
	disableCtx.Params = gin.Params{{Key: "id", Value: fmt.Sprintf("%d", pool.Id)}}
	DisableQuotaPool(disableCtx)
	if response := decodeQuotaPoolResponse(t, disableRecorder); response["success"] != true {
		t.Fatalf("expected disable success, got %#v", response)
	}
	assertQuotaPoolManageLogContains(t, db, 99, "禁用额度池")

	deleteCtx, deleteRecorder := quotaPoolTestContext(t, http.MethodDelete, fmt.Sprintf("/api/quota_pool/%d", pool.Id), nil, common.RoleRootUser, 99)
	deleteCtx.Params = gin.Params{{Key: "id", Value: fmt.Sprintf("%d", pool.Id)}}
	DeleteQuotaPool(deleteCtx)
	if response := decodeQuotaPoolResponse(t, deleteRecorder); response["success"] != true {
		t.Fatalf("expected delete success, got %#v", response)
	}
	assertQuotaPoolManageLogContains(t, db, 99, "删除额度池")
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
