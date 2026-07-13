package service

import (
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func setupBillingSessionTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	oldDB := model.DB
	oldLogDB := model.LOG_DB
	oldRedisEnabled := common.RedisEnabled

	dsn := "file:" + strings.ReplaceAll(t.Name(), "/", "_") + "?mode=memory&cache=shared"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite failed: %v", err)
	}
	model.DB = db
	model.LOG_DB = db
	common.RedisEnabled = false
	if err := db.AutoMigrate(&model.User{}); err != nil {
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
	})

	return db
}

func TestInsufficientUserQuotaErrorForQuotaPoolMember(t *testing.T) {
	db := setupBillingSessionTestDB(t)
	user := model.User{
		Username:    "pool-member",
		Password:    "password",
		Role:        common.RoleCommonUser,
		Status:      common.UserStatusEnabled,
		QuotaPoolId: 10,
		AffCode:     "pool-member-code",
		CreatedAt:   time.Now().Unix(),
	}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("create user failed: %v", err)
	}

	err := insufficientUserQuotaError(user.Id, 0)
	if err == nil || err.Error() != "额度不足，请联系池管理员充值" {
		t.Fatalf("expected pool admin contact hint, got %v", err)
	}
}

func TestInsufficientUserQuotaErrorForDefaultPoolMember(t *testing.T) {
	db := setupBillingSessionTestDB(t)
	user := model.User{
		Username:    "default-member",
		Password:    "password",
		Role:        common.RoleCommonUser,
		Status:      common.UserStatusEnabled,
		QuotaPoolId: model.QuotaPoolDefaultUserPoolId,
		AffCode:     "default-member-code",
		CreatedAt:   time.Now().Unix(),
	}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("create user failed: %v", err)
	}

	err := insufficientUserQuotaError(user.Id, 0)
	if err == nil || !strings.Contains(err.Error(), "用户额度不足") {
		t.Fatalf("expected original user quota hint, got %v", err)
	}
}
