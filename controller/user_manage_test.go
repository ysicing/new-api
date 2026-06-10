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

	common.UsingSQLite = true
	common.UsingMySQL = false
	common.UsingPostgreSQL = false
	common.RedisEnabled = false

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open sqlite db: %v", err)
	}
	model.DB = db
	model.LOG_DB = db

	if err := db.AutoMigrate(&model.User{}); err != nil {
		t.Fatalf("failed to migrate user table: %v", err)
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
