package controller

import (
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/oauth"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func setupOAuthControllerTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	oldDB := model.DB
	oldLogDB := model.LOG_DB
	oldRegisterEnabled := common.RegisterEnabled
	oldRedisEnabled := common.RedisEnabled

	dsn := "file:" + strings.ReplaceAll(t.Name(), "/", "_") + "?mode=memory&cache=shared"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite failed: %v", err)
	}
	model.DB = db
	model.LOG_DB = db
	common.RegisterEnabled = false
	common.RedisEnabled = false

	if err := db.AutoMigrate(&model.User{}, &model.UserOAuthBinding{}); err != nil {
		t.Fatalf("migrate failed: %v", err)
	}

	t.Cleanup(func() {
		sqlDB, err := db.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
		model.DB = oldDB
		model.LOG_DB = oldLogDB
		common.RegisterEnabled = oldRegisterEnabled
		common.RedisEnabled = oldRedisEnabled
	})

	return db
}

func TestFindOrCreateOAuthUserReusesExistingLDAPUserByEmail(t *testing.T) {
	db := setupOAuthControllerTestDB(t)
	existing := model.User{
		Username:  "ldap-user",
		Email:     "alice@example.com",
		LDAPId:    "alice@example.com",
		Role:      common.RoleCommonUser,
		Status:    common.UserStatusEnabled,
		AffCode:   "ldap-user-code",
		CreatedAt: time.Now().Unix(),
	}
	if err := db.Create(&existing).Error; err != nil {
		t.Fatalf("create existing user failed: %v", err)
	}

	user, err := findOrCreateOAuthUser(nil, &oauth.OIDCProvider{}, &oauth.OAuthUser{
		ProviderUserID: "oidc-sub",
		Username:       "alice",
		DisplayName:    "Alice",
		Email:          "Alice@Example.com",
	}, nil)
	if err != nil {
		t.Fatalf("find or create oauth user failed: %v", err)
	}
	if user.Id != existing.Id || user.OidcId != "oidc-sub" {
		t.Fatalf("expected existing LDAP user with OIDC binding, got %+v", user)
	}

	var count int64
	if err := db.Model(&model.User{}).Count(&count).Error; err != nil {
		t.Fatalf("count users failed: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected one user, got %d", count)
	}
}

func TestFindOrCreateOAuthUserRejectsDifferentOIDCForSameEmail(t *testing.T) {
	db := setupOAuthControllerTestDB(t)
	existing := model.User{
		Username:  "oidc-user",
		Email:     "alice@example.com",
		OidcId:    "old-oidc-sub",
		Role:      common.RoleCommonUser,
		Status:    common.UserStatusEnabled,
		AffCode:   "oidc-user-code",
		CreatedAt: time.Now().Unix(),
	}
	if err := db.Create(&existing).Error; err != nil {
		t.Fatalf("create existing user failed: %v", err)
	}

	_, err := findOrCreateOAuthUser(nil, &oauth.OIDCProvider{}, &oauth.OAuthUser{
		ProviderUserID: "new-oidc-sub",
		Username:       "alice",
		DisplayName:    "Alice",
		Email:          "alice@example.com",
	}, nil)
	if err == nil || !strings.Contains(err.Error(), "已绑定其他 OAuth 账号") {
		t.Fatalf("expected existing OIDC binding error, got %v", err)
	}
}
