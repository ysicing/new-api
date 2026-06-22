package controller

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

type manageUserAPIResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

func setupUserManageTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	gin.SetMode(gin.TestMode)
	oldDB := model.DB
	oldLogDB := model.LOG_DB
	oldUsingSQLite := common.UsingSQLite
	oldUsingMySQL := common.UsingMySQL
	oldUsingPostgreSQL := common.UsingPostgreSQL
	oldRedisEnabled := common.RedisEnabled
	oldQuotaPoolEnabled := common.QuotaPoolEnabled

	common.UsingSQLite = true
	common.UsingMySQL = false
	common.UsingPostgreSQL = false
	common.RedisEnabled = false
	common.QuotaPoolEnabled = true

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open sqlite db: %v", err)
	}
	model.DB = db
	model.LOG_DB = db

	if err := db.AutoMigrate(&model.User{}, &model.QuotaPool{}, &model.QuotaPoolTransaction{}, &model.QuotaPoolAdmin{}); err != nil {
		t.Fatalf("failed to migrate user table: %v", err)
	}
	if err := db.AutoMigrate(&model.Log{}); err != nil {
		t.Fatalf("failed to migrate log table: %v", err)
	}

	t.Cleanup(func() {
		sqlDB, err := db.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
		model.DB = oldDB
		model.LOG_DB = oldLogDB
		common.UsingSQLite = oldUsingSQLite
		common.UsingMySQL = oldUsingMySQL
		common.UsingPostgreSQL = oldUsingPostgreSQL
		common.RedisEnabled = oldRedisEnabled
		common.QuotaPoolEnabled = oldQuotaPoolEnabled
	})

	return db
}

func newManageUserContext(t *testing.T, body any, role int, userID int) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()

	payload, err := common.Marshal(body)
	if err != nil {
		t.Fatalf("failed to marshal request body: %v", err)
	}

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/api/user/manage", bytes.NewReader(payload))
	ctx.Request.Header.Set("Content-Type", "application/json")
	ctx.Set("id", userID)
	ctx.Set("role", role)
	ctx.Set("username", "admin-user")
	return ctx, recorder
}

func decodeManageUserResponse(t *testing.T, recorder *httptest.ResponseRecorder) manageUserAPIResponse {
	t.Helper()

	var response manageUserAPIResponse
	if err := common.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to decode api response: %v", err)
	}
	return response
}

func assertLatestManageLogAdminInfo(t *testing.T, db *gorm.DB, userId int, adminId int, adminUsername string) {
	t.Helper()

	var log model.Log
	if err := db.Where("user_id = ? AND type = ?", userId, model.LogTypeManage).
		Order("id desc").
		First(&log).Error; err != nil {
		t.Fatalf("failed to load manage log: %v", err)
	}

	var other map[string]interface{}
	if err := common.UnmarshalJsonStr(log.Other, &other); err != nil {
		t.Fatalf("failed to decode log other: %v", err)
	}
	adminInfo, ok := other["admin_info"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected admin_info in log other, got %#v", other)
	}
	if got := int(adminInfo["admin_id"].(float64)); got != adminId {
		t.Fatalf("unexpected admin_id, got %d want %d", got, adminId)
	}
	if got := adminInfo["admin_username"]; got != adminUsername {
		t.Fatalf("unexpected admin_username, got %#v", got)
	}
}

func TestManageUserAdminCannotDisableAdmin(t *testing.T) {
	db := setupUserManageTestDB(t)
	target := &model.User{
		Username: "target-admin",
		Password: "password",
		Role:     common.RoleAdminUser,
		Status:   common.UserStatusEnabled,
		Group:    "default",
	}
	if err := db.Create(target).Error; err != nil {
		t.Fatalf("failed to create target admin: %v", err)
	}

	ctx, recorder := newManageUserContext(t, ManageRequest{
		Id:     target.Id,
		Action: "disable",
	}, common.RoleAdminUser, 99)
	ManageUser(ctx)

	response := decodeManageUserResponse(t, recorder)
	if response.Success {
		t.Fatalf("expected admin disabling admin to fail")
	}

	var updated model.User
	if err := db.First(&updated, target.Id).Error; err != nil {
		t.Fatalf("failed to reload target admin: %v", err)
	}
	if updated.Status != common.UserStatusEnabled {
		t.Fatalf("expected target admin to remain enabled, got status %d", updated.Status)
	}
}

func TestAdminClearLDAPBinding(t *testing.T) {
	db := setupUserManageTestDB(t)
	user := model.User{
		Username:  "ldap-user",
		Password:  "password",
		Email:     "ldap@example.com",
		LDAPId:    "ldap-user",
		Role:      common.RoleCommonUser,
		Status:    common.UserStatusEnabled,
		AffCode:   "ldap-code",
		CreatedAt: 1,
	}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("failed to create user: %v", err)
	}

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/api/user/%d/bindings/ldap", user.Id), nil)
	ctx.Params = gin.Params{
		{Key: "id", Value: fmt.Sprint(user.Id)},
		{Key: "binding_type", Value: "ldap"},
	}
	ctx.Set("id", 999)
	ctx.Set("role", common.RoleAdminUser)
	ctx.Set("username", "admin-user")

	AdminClearUserBinding(ctx)
	response := decodeManageUserResponse(t, recorder)
	if !response.Success {
		t.Fatalf("expected success, got %+v", response)
	}

	var updated model.User
	if err := db.First(&updated, user.Id).Error; err != nil {
		t.Fatalf("failed to load updated user: %v", err)
	}
	if updated.LDAPId != "" {
		t.Fatalf("expected ldap binding cleared, got %q", updated.LDAPId)
	}
}

func TestSetupLoginReturnsQuotaPoolAdminSummary(t *testing.T) {
	db := setupUserManageTestDB(t)
	pool := &model.QuotaPool{Name: "team", Enabled: true, BaseQuota: 100, Quota: 100, MonthlyRefillDay: 1}
	if err := db.Create(pool).Error; err != nil {
		t.Fatalf("failed to create pool: %v", err)
	}
	user := &model.User{
		Username:    "pool-admin",
		Password:    "password",
		Role:        common.RoleCommonUser,
		Status:      common.UserStatusEnabled,
		QuotaPoolId: pool.Id,
		Group:       "default",
	}
	if err := db.Create(user).Error; err != nil {
		t.Fatalf("failed to create user: %v", err)
	}
	if err := db.Create(&model.QuotaPoolAdmin{PoolId: pool.Id, UserId: user.Id, Level: model.QuotaPoolAdminLevelV2}).Error; err != nil {
		t.Fatalf("failed to create pool admin: %v", err)
	}

	router := gin.New()
	router.Use(sessions.Sessions("test-session", cookie.NewStore([]byte("secret"))))
	router.GET("/login-test", func(c *gin.Context) {
		setupLogin(user, c)
	})
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/login-test", nil))

	var response struct {
		Success bool `json:"success"`
		Data    struct {
			QuotaPoolAdmin *model.QuotaPoolAdminSummary `json:"quota_pool_admin"`
		} `json:"data"`
	}
	if err := common.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response failed: %v", err)
	}
	if !response.Success || response.Data.QuotaPoolAdmin == nil {
		t.Fatalf("expected quota_pool_admin in login response, got %s", recorder.Body.String())
	}
	if response.Data.QuotaPoolAdmin.PoolId != pool.Id || response.Data.QuotaPoolAdmin.Level != model.QuotaPoolAdminLevelV2 {
		t.Fatalf("unexpected quota_pool_admin: %+v", response.Data.QuotaPoolAdmin)
	}
}

func TestUpdateUserAdminCannotEditQuota(t *testing.T) {
	db := setupUserManageTestDB(t)
	target := &model.User{
		Username: "target-user",
		Password: "password",
		Role:     common.RoleCommonUser,
		Status:   common.UserStatusEnabled,
		Quota:    100,
		Group:    "default",
	}
	if err := db.Create(target).Error; err != nil {
		t.Fatalf("failed to create target user: %v", err)
	}

	ctx, recorder := newManageUserContext(t, model.User{
		Id:       target.Id,
		Username: target.Username,
		Role:     common.RoleCommonUser,
		Status:   common.UserStatusEnabled,
		Quota:    200,
		Group:    "default",
	}, common.RoleAdminUser, 99)
	UpdateUser(ctx)

	response := decodeManageUserResponse(t, recorder)
	if response.Success {
		t.Fatalf("expected admin editing quota to fail")
	}

	var updated model.User
	if err := db.First(&updated, target.Id).Error; err != nil {
		t.Fatalf("failed to reload target user: %v", err)
	}
	if updated.Quota != target.Quota {
		t.Fatalf("expected target quota to remain %d, got %d", target.Quota, updated.Quota)
	}
}

func TestUpdateUserRootCannotEditQuota(t *testing.T) {
	db := setupUserManageTestDB(t)
	target := &model.User{
		Username: "target-user",
		Password: "password",
		Role:     common.RoleCommonUser,
		Status:   common.UserStatusEnabled,
		Quota:    100,
		Group:    "default",
	}
	if err := db.Create(target).Error; err != nil {
		t.Fatalf("failed to create target user: %v", err)
	}

	ctx, recorder := newManageUserContext(t, model.User{
		Id:       target.Id,
		Username: target.Username,
		Role:     common.RoleCommonUser,
		Status:   common.UserStatusEnabled,
		Quota:    200,
		Group:    "default",
	}, common.RoleRootUser, 1)
	UpdateUser(ctx)

	response := decodeManageUserResponse(t, recorder)
	if response.Success {
		t.Fatalf("expected root editing quota to fail")
	}

	var updated model.User
	if err := db.First(&updated, target.Id).Error; err != nil {
		t.Fatalf("failed to reload target user: %v", err)
	}
	if updated.Quota != target.Quota {
		t.Fatalf("expected target quota to remain %d, got %d", target.Quota, updated.Quota)
	}
}

func TestManageUserRechargeAutoRecordsAdminInfo(t *testing.T) {
	db := setupUserManageTestDB(t)
	cfg := operation_setting.GetAutoRechargeSetting()
	originalAmount := cfg.Amount
	defer func() {
		cfg.Amount = originalAmount
	}()
	cfg.Amount = 2

	target := &model.User{
		Username: "target-user",
		Password: "password",
		Role:     common.RoleCommonUser,
		Status:   common.UserStatusEnabled,
		Quota:    100,
		Group:    "default",
	}
	if err := db.Create(target).Error; err != nil {
		t.Fatalf("failed to create target user: %v", err)
	}

	ctx, recorder := newManageUserContext(t, ManageRequest{
		Id:     target.Id,
		Action: "recharge_auto",
	}, common.RoleAdminUser, 99)
	ManageUser(ctx)

	response := decodeManageUserResponse(t, recorder)
	if !response.Success {
		t.Fatalf("expected recharge_auto to succeed, got message %q", response.Message)
	}

	var log model.Log
	if err := db.Where("user_id = ? AND type = ?", target.Id, model.LogTypeManage).
		Order("id desc").
		First(&log).Error; err != nil {
		t.Fatalf("failed to load manage log: %v", err)
	}

	var other map[string]interface{}
	if err := common.UnmarshalJsonStr(log.Other, &other); err != nil {
		t.Fatalf("failed to decode log other: %v", err)
	}
	adminInfo, ok := other["admin_info"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected admin_info in log other, got %#v", other)
	}
	if got := int(adminInfo["admin_id"].(float64)); got != 99 {
		t.Fatalf("unexpected admin_id, got %d want %d", got, 99)
	}
	if got := adminInfo["admin_username"]; got != "admin-user" {
		t.Fatalf("unexpected admin_username, got %#v", got)
	}
}

func TestManageUserRootCanAdjustDefaultPoolUserQuota(t *testing.T) {
	db := setupUserManageTestDB(t)
	target := &model.User{
		Username: "target-user",
		Password: "password",
		Role:     common.RoleCommonUser,
		Status:   common.UserStatusEnabled,
		Quota:    100,
		Group:    "default",
	}
	if err := db.Create(target).Error; err != nil {
		t.Fatalf("failed to create target user: %v", err)
	}

	ctx, recorder := newManageUserContext(t, ManageRequest{
		Id:     target.Id,
		Action: "add_quota",
		Mode:   "add",
		Value:  100,
	}, common.RoleRootUser, 99)
	ManageUser(ctx)

	response := decodeManageUserResponse(t, recorder)
	if !response.Success {
		t.Fatalf("expected root add_quota to succeed, got %q", response.Message)
	}
	var updated model.User
	_ = db.First(&updated, target.Id).Error
	if updated.Quota != 200 {
		t.Fatalf("quota = %d, want 200", updated.Quota)
	}
	assertLatestManageLogAdminInfo(t, db, target.Id, 99, "admin-user")
}

func TestManageUserRootCanOverrideDefaultPoolUserQuota(t *testing.T) {
	db := setupUserManageTestDB(t)
	target := &model.User{
		Username: "target-user",
		Password: "password",
		Role:     common.RoleCommonUser,
		Status:   common.UserStatusEnabled,
		Quota:    100,
		Group:    "default",
	}
	if err := db.Create(target).Error; err != nil {
		t.Fatalf("failed to create target user: %v", err)
	}

	ctx, recorder := newManageUserContext(t, ManageRequest{
		Id:     target.Id,
		Action: "add_quota",
		Mode:   "override",
		Value:  12,
	}, common.RoleRootUser, 99)
	ManageUser(ctx)

	response := decodeManageUserResponse(t, recorder)
	if !response.Success {
		t.Fatalf("expected root override add_quota to succeed, got %q", response.Message)
	}
	var updated model.User
	_ = db.First(&updated, target.Id).Error
	if updated.Quota != 12 {
		t.Fatalf("quota = %d, want 12", updated.Quota)
	}
	assertLatestManageLogAdminInfo(t, db, target.Id, 99, "admin-user")
}

func TestManageUserRootCanSubtractDefaultPoolUserQuota(t *testing.T) {
	db := setupUserManageTestDB(t)
	target := &model.User{
		Username: "target-user",
		Password: "password",
		Role:     common.RoleCommonUser,
		Status:   common.UserStatusEnabled,
		Quota:    100,
		Group:    "default",
	}
	if err := db.Create(target).Error; err != nil {
		t.Fatalf("failed to create target user: %v", err)
	}

	ctx, recorder := newManageUserContext(t, ManageRequest{
		Id:     target.Id,
		Action: "add_quota",
		Mode:   "subtract",
		Value:  40,
	}, common.RoleRootUser, 99)
	ManageUser(ctx)

	response := decodeManageUserResponse(t, recorder)
	if !response.Success {
		t.Fatalf("expected root subtract add_quota to succeed, got %q", response.Message)
	}
	var updated model.User
	_ = db.First(&updated, target.Id).Error
	if updated.Quota != 60 {
		t.Fatalf("quota = %d, want 60", updated.Quota)
	}
	assertLatestManageLogAdminInfo(t, db, target.Id, 99, "admin-user")
}

func TestManageUserAdminCannotDirectlyAdjustQuota(t *testing.T) {
	db := setupUserManageTestDB(t)
	target := &model.User{
		Username: "target-user",
		Password: "password",
		Role:     common.RoleCommonUser,
		Status:   common.UserStatusEnabled,
		Quota:    100,
		Group:    "default",
	}
	if err := db.Create(target).Error; err != nil {
		t.Fatalf("failed to create target user: %v", err)
	}

	ctx, recorder := newManageUserContext(t, ManageRequest{
		Id:     target.Id,
		Action: "add_quota",
		Mode:   "add",
		Value:  100,
	}, common.RoleAdminUser, 99)
	ManageUser(ctx)

	response := decodeManageUserResponse(t, recorder)
	if response.Success {
		t.Fatalf("expected admin add_quota to be rejected")
	}
	var updated model.User
	_ = db.First(&updated, target.Id).Error
	if updated.Quota != 100 {
		t.Fatalf("quota changed unexpectedly: %d", updated.Quota)
	}
}

func TestManageUserRootCannotDirectlyAdjustNonDefaultPoolUserQuota(t *testing.T) {
	db := setupUserManageTestDB(t)
	pool := &model.QuotaPool{Name: "team", Enabled: true, BaseQuota: 1000, Quota: 1000, MonthlyRefillDay: 1}
	if err := db.Create(pool).Error; err != nil {
		t.Fatalf("failed to create pool: %v", err)
	}
	target := &model.User{
		Username:    "target-user",
		Password:    "password",
		Role:        common.RoleCommonUser,
		Status:      common.UserStatusEnabled,
		Quota:       100,
		QuotaPoolId: pool.Id,
		Group:       "default",
	}
	if err := db.Create(target).Error; err != nil {
		t.Fatalf("failed to create target user: %v", err)
	}

	ctx, recorder := newManageUserContext(t, ManageRequest{
		Id:     target.Id,
		Action: "add_quota",
		Mode:   "add",
		Value:  100,
	}, common.RoleRootUser, 99)
	ManageUser(ctx)

	response := decodeManageUserResponse(t, recorder)
	if response.Success {
		t.Fatalf("expected non-default pool add_quota to be rejected")
	}
	var updated model.User
	_ = db.First(&updated, target.Id).Error
	if updated.Quota != 100 {
		t.Fatalf("quota changed unexpectedly: %d", updated.Quota)
	}
}

func TestManageUserRechargeAutoDebitsNonDefaultQuotaPool(t *testing.T) {
	db := setupUserManageTestDB(t)
	cfg := operation_setting.GetAutoRechargeSetting()
	originalAmount := cfg.Amount
	defer func() {
		cfg.Amount = originalAmount
	}()
	cfg.Amount = 2

	pool := &model.QuotaPool{
		Name:               "team",
		Enabled:            true,
		BaseQuota:          int(10 * common.QuotaPerUnit),
		Quota:              int(10 * common.QuotaPerUnit),
		AutoRechargeAmount: model.QuotaPoolAutoRechargeInherit,
		WeeklyLimit:        model.QuotaPoolAutoRechargeInherit,
		MonthlyLimit:       model.QuotaPoolAutoRechargeInherit,
		MonthlyRefillDay:   1,
	}
	if err := db.Create(pool).Error; err != nil {
		t.Fatalf("failed to create quota pool: %v", err)
	}
	target := &model.User{
		Username:    "target-user",
		Password:    "password",
		Role:        common.RoleCommonUser,
		Status:      common.UserStatusEnabled,
		Quota:       100,
		QuotaPoolId: pool.Id,
		Group:       "default",
	}
	if err := db.Create(target).Error; err != nil {
		t.Fatalf("failed to create target user: %v", err)
	}

	ctx, recorder := newManageUserContext(t, ManageRequest{
		Id:     target.Id,
		Action: "recharge_auto",
	}, common.RoleAdminUser, 99)
	ManageUser(ctx)

	response := decodeManageUserResponse(t, recorder)
	if !response.Success {
		t.Fatalf("expected recharge_auto to succeed, got %q", response.Message)
	}
	amountQuota := int(float64(cfg.Amount) * common.QuotaPerUnit)
	var updatedPool model.QuotaPool
	_ = db.First(&updatedPool, pool.Id).Error
	if updatedPool.Quota != pool.Quota-amountQuota {
		t.Fatalf("pool quota = %d, want %d", updatedPool.Quota, pool.Quota-amountQuota)
	}
	var tx model.QuotaPoolTransaction
	if err := db.First(&tx, "pool_id = ? AND type = ?", pool.Id, model.QuotaPoolTransactionAllocateManual).Error; err != nil {
		t.Fatalf("expected pool transaction: %v", err)
	}
	if tx.Amount != -amountQuota || tx.OperatorId != 99 || tx.UserId != target.Id {
		t.Fatalf("unexpected transaction: %+v", tx)
	}
}

func TestManageUserRechargeAutoWhenQuotaPoolFeatureDisabledUsesOldPath(t *testing.T) {
	db := setupUserManageTestDB(t)
	common.QuotaPoolEnabled = false
	cfg := operation_setting.GetAutoRechargeSetting()
	originalAmount := cfg.Amount
	defer func() {
		cfg.Amount = originalAmount
	}()
	cfg.Amount = 2

	pool := &model.QuotaPool{
		Name:               "team",
		Enabled:            true,
		BaseQuota:          int(10 * common.QuotaPerUnit),
		Quota:              int(10 * common.QuotaPerUnit),
		AutoRechargeAmount: model.QuotaPoolAutoRechargeInherit,
		WeeklyLimit:        model.QuotaPoolAutoRechargeInherit,
		MonthlyLimit:       model.QuotaPoolAutoRechargeInherit,
		MonthlyRefillDay:   1,
	}
	if err := db.Create(pool).Error; err != nil {
		t.Fatalf("failed to create quota pool: %v", err)
	}
	target := &model.User{
		Username:    "target-user",
		Password:    "password",
		Role:        common.RoleCommonUser,
		Status:      common.UserStatusEnabled,
		Quota:       100,
		QuotaPoolId: pool.Id,
		Group:       "default",
	}
	if err := db.Create(target).Error; err != nil {
		t.Fatalf("failed to create target user: %v", err)
	}

	ctx, recorder := newManageUserContext(t, ManageRequest{
		Id:     target.Id,
		Action: "recharge_auto",
	}, common.RoleAdminUser, 99)
	ManageUser(ctx)

	response := decodeManageUserResponse(t, recorder)
	if !response.Success {
		t.Fatalf("expected recharge_auto to succeed, got %q", response.Message)
	}
	amountQuota := int(float64(cfg.Amount) * common.QuotaPerUnit)
	var updatedPool model.QuotaPool
	_ = db.First(&updatedPool, pool.Id).Error
	if updatedPool.Quota != pool.Quota {
		t.Fatalf("pool quota = %d, want unchanged %d", updatedPool.Quota, pool.Quota)
	}
	var updatedUser model.User
	_ = db.First(&updatedUser, target.Id).Error
	if updatedUser.Quota != target.Quota+amountQuota {
		t.Fatalf("user quota = %d, want %d", updatedUser.Quota, target.Quota+amountQuota)
	}
	var txCount int64
	_ = db.Model(&model.QuotaPoolTransaction{}).Count(&txCount).Error
	if txCount != 0 {
		t.Fatalf("quota pool disabled should not write transactions, got %d", txCount)
	}
}
