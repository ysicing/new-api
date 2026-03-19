package service

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

type autoRechargeSQLCaptureLogger struct {
	gormlogger.Interface
	mu   sync.Mutex
	sqls []string
}

func newAutoRechargeSQLCaptureLogger() *autoRechargeSQLCaptureLogger {
	return &autoRechargeSQLCaptureLogger{
		Interface: gormlogger.Default.LogMode(gormlogger.Silent),
	}
}

func (l *autoRechargeSQLCaptureLogger) LogMode(level gormlogger.LogLevel) gormlogger.Interface {
	l.Interface = l.Interface.LogMode(level)
	return l
}

func (l *autoRechargeSQLCaptureLogger) Trace(ctx context.Context, begin time.Time, fc func() (string, int64), err error) {
	sql, _ := fc()
	l.mu.Lock()
	l.sqls = append(l.sqls, sql)
	l.mu.Unlock()
	l.Interface.Trace(ctx, begin, fc, err)
}

func (l *autoRechargeSQLCaptureLogger) AllSQL() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return strings.Join(l.sqls, "\n")
}

func setupAutoRechargeTestDB(t *testing.T) (*gorm.DB, *autoRechargeSQLCaptureLogger, func()) {
	t.Helper()

	capture := newAutoRechargeSQLCaptureLogger()
	dsn := fmt.Sprintf("file:auto_recharge_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{Logger: capture})
	if err != nil {
		t.Fatalf("open sqlite db failed: %v", err)
	}

	oldDB := model.DB
	oldLogDB := model.LOG_DB
	model.DB = db
	model.LOG_DB = db

	if err := db.AutoMigrate(&model.User{}, &model.Log{}); err != nil {
		t.Fatalf("migrate test db failed: %v", err)
	}

	cleanup := func() {
		model.DB = oldDB
		model.LOG_DB = oldLogDB
	}
	return db, capture, cleanup
}

func TestStartAutoRechargeTask_OnlyStartsOnMasterNode(t *testing.T) {
	originalMaster := common.IsMasterNode
	originalRunner := autoRechargeRunner
	originalOnce := autoRechargeOnce
	defer func() {
		common.IsMasterNode = originalMaster
		autoRechargeRunner = originalRunner
		autoRechargeOnce = originalOnce
	}()

	var started atomic.Int32
	autoRechargeRunner = func() {
		started.Add(1)
	}

	autoRechargeOnce = sync.Once{}
	common.IsMasterNode = false
	StartAutoRechargeTask()
	time.Sleep(50 * time.Millisecond)
	if started.Load() != 0 {
		t.Fatalf("worker started on non-master node")
	}

	autoRechargeOnce = sync.Once{}
	common.IsMasterNode = true
	StartAutoRechargeTask()
	time.Sleep(50 * time.Millisecond)
	if started.Load() != 1 {
		t.Fatalf("worker should start exactly once on master node, got %d", started.Load())
	}
}

func TestCheckAndRechargeUsers_DoesNotRequeryUserQuota(t *testing.T) {
	db, capture, cleanup := setupAutoRechargeTestDB(t)
	defer cleanup()

	user := &model.User{
		Id:       1,
		Username: "quota_user",
		Password: "12345678",
		Status:   common.UserStatusEnabled,
		Quota:    10,
	}
	if err := db.Create(user).Error; err != nil {
		t.Fatalf("create user failed: %v", err)
	}

	cfg := &operation_setting.AutoRechargeSetting{
		Enabled:      true,
		Interval:     30,
		Threshold:    50,
		Amount:       20,
		WeeklyLimit:  0,
		MonthlyLimit: 0,
	}

	checkAndRechargeUsers(cfg, 50, 20)

	sqlText := strings.ToLower(capture.AllSQL())
	if strings.Contains(sqlText, "select `quota` from `users`") || strings.Contains(sqlText, `select "quota" from "users"`) {
		t.Fatalf("unexpected per-user quota re-query detected:\n%s", capture.AllSQL())
	}
}
