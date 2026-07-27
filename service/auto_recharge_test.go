package service

import (
	"context"
	"errors"
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
	oldQuotaPoolEnabled := common.QuotaPoolEnabled
	model.DB = db
	model.LOG_DB = db
	common.QuotaPoolEnabled = true

	if err := db.AutoMigrate(&model.User{}, &model.Log{}, &model.QuotaPool{}, &model.QuotaPoolTransaction{}); err != nil {
		t.Fatalf("migrate test db failed: %v", err)
	}

	cleanup := func() {
		model.DB = oldDB
		model.LOG_DB = oldLogDB
		common.QuotaPoolEnabled = oldQuotaPoolEnabled
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

func TestTryAutoRechargeUser_DefaultPoolKeepsOldPath(t *testing.T) {
	db, _, cleanup := setupAutoRechargeTestDB(t)
	defer cleanup()

	user := &model.User{
		Id:       1,
		Username: "default_pool_user",
		Password: "12345678",
		Status:   common.UserStatusEnabled,
		Quota:    10,
	}
	if err := db.Create(user).Error; err != nil {
		t.Fatalf("create user failed: %v", err)
	}
	cfg := &operation_setting.AutoRechargeSetting{Amount: 20}

	result := tryAutoRechargeUser(cfg, user, 50, 20)
	if !result.Recharged {
		t.Fatalf("expected recharge, reason=%s", result.Reason)
	}

	var got model.User
	_ = db.First(&got, user.Id).Error
	if got.Quota != 30 {
		t.Fatalf("user quota = %d, want 30", got.Quota)
	}
	var txCount int64
	_ = db.Model(&model.QuotaPoolTransaction{}).Count(&txCount).Error
	if txCount != 0 {
		t.Fatalf("default pool should not write quota pool transactions, got %d", txCount)
	}
	var log model.Log
	if err := db.First(&log, "user_id = ? AND type = ?", user.Id, model.LogTypeSystem).Error; err != nil {
		t.Fatalf("expected auto recharge log: %v", err)
	}
	if !strings.HasPrefix(log.Content, "系统自动赠送 ") {
		t.Fatalf("auto recharge log content = %q, want system prefix", log.Content)
	}
}

func TestTryAutoRechargeUser_NonDefaultPoolDebitsPool(t *testing.T) {
	db, _, cleanup := setupAutoRechargeTestDB(t)
	defer cleanup()

	pool := &model.QuotaPool{
		Name:               "team-a",
		Enabled:            true,
		BaseQuota:          100,
		Quota:              100,
		AutoRechargeAmount: 20,
		WeeklyLimit:        model.QuotaPoolAutoRechargeInherit,
		MonthlyLimit:       model.QuotaPoolAutoRechargeInherit,
		MonthlyRefillDay:   1,
	}
	if err := db.Create(pool).Error; err != nil {
		t.Fatalf("create pool failed: %v", err)
	}
	user := &model.User{
		Id:          1,
		Username:    "pool_user",
		Password:    "12345678",
		Status:      common.UserStatusEnabled,
		Quota:       10,
		QuotaPoolId: pool.Id,
	}
	if err := db.Create(user).Error; err != nil {
		t.Fatalf("create user failed: %v", err)
	}
	cfg := &operation_setting.AutoRechargeSetting{Amount: 50}

	result := tryAutoRechargeUser(cfg, user, 50, 50)
	if !result.Recharged {
		t.Fatalf("expected recharge, reason=%s", result.Reason)
	}

	var gotPool model.QuotaPool
	_ = db.First(&gotPool, pool.Id).Error
	if gotPool.Quota != 80 {
		t.Fatalf("pool quota = %d, want 80", gotPool.Quota)
	}
	var gotUser model.User
	_ = db.First(&gotUser, user.Id).Error
	if gotUser.Quota != 30 {
		t.Fatalf("user quota = %d, want 30", gotUser.Quota)
	}
	var tx model.QuotaPoolTransaction
	if err := db.First(&tx, "pool_id = ?", pool.Id).Error; err != nil {
		t.Fatalf("expected quota pool transaction: %v", err)
	}
	if tx.Type != model.QuotaPoolTransactionAllocateAuto || tx.Amount != -20 {
		t.Fatalf("unexpected transaction: %+v", tx)
	}
	var log model.Log
	if err := db.First(&log, "user_id = ? AND type = ?", user.Id, model.LogTypeSystem).Error; err != nil {
		t.Fatalf("expected auto recharge log: %v", err)
	}
	if !strings.Contains(log.Content, "额度池team-a自动赠送 ") {
		t.Fatalf("auto recharge log content = %q, want quota pool prefix", log.Content)
	}
	if strings.HasPrefix(log.Content, "系统自动赠送 ") {
		t.Fatalf("auto recharge log content = %q, should not use system prefix", log.Content)
	}
}

func TestTryAutoRechargeUser_QuotaPoolDisabledUsesOldPath(t *testing.T) {
	db, _, cleanup := setupAutoRechargeTestDB(t)
	defer cleanup()
	common.QuotaPoolEnabled = false

	pool := &model.QuotaPool{
		Name:               "team-disabled-feature",
		Enabled:            true,
		BaseQuota:          100,
		Quota:              100,
		AutoRechargeAmount: 20,
		WeeklyLimit:        model.QuotaPoolAutoRechargeInherit,
		MonthlyLimit:       model.QuotaPoolAutoRechargeInherit,
		MonthlyRefillDay:   1,
	}
	if err := db.Create(pool).Error; err != nil {
		t.Fatalf("create pool failed: %v", err)
	}
	user := &model.User{
		Id:          1,
		Username:    "pool_user",
		Password:    "12345678",
		Status:      common.UserStatusEnabled,
		Quota:       10,
		QuotaPoolId: pool.Id,
	}
	if err := db.Create(user).Error; err != nil {
		t.Fatalf("create user failed: %v", err)
	}

	result := tryAutoRechargeUser(&operation_setting.AutoRechargeSetting{Amount: 50}, user, 50, 50)
	if !result.Recharged {
		t.Fatalf("expected recharge via old path, reason=%s", result.Reason)
	}

	var gotPool model.QuotaPool
	_ = db.First(&gotPool, pool.Id).Error
	if gotPool.Quota != 100 {
		t.Fatalf("pool quota = %d, want unchanged 100", gotPool.Quota)
	}
	var gotUser model.User
	_ = db.First(&gotUser, user.Id).Error
	if gotUser.Quota != 60 {
		t.Fatalf("user quota = %d, want 60", gotUser.Quota)
	}
	var txCount int64
	_ = db.Model(&model.QuotaPoolTransaction{}).Count(&txCount).Error
	if txCount != 0 {
		t.Fatalf("quota pool disabled should not write transactions, got %d", txCount)
	}
}

func TestCheckAndRechargeUsers_UsesPoolAmountWhenSystemAmountIsZero(t *testing.T) {
	db, _, cleanup := setupAutoRechargeTestDB(t)
	defer cleanup()

	pool := &model.QuotaPool{
		Name:               "team-override",
		Enabled:            true,
		BaseQuota:          100,
		Quota:              100,
		AutoRechargeAmount: 20,
		WeeklyLimit:        model.QuotaPoolAutoRechargeInherit,
		MonthlyLimit:       model.QuotaPoolAutoRechargeInherit,
		MonthlyRefillDay:   1,
	}
	if err := db.Create(pool).Error; err != nil {
		t.Fatalf("create pool failed: %v", err)
	}
	user := &model.User{
		Id:          1,
		Username:    "override_user",
		Password:    "12345678",
		Status:      common.UserStatusEnabled,
		Quota:       10,
		QuotaPoolId: pool.Id,
	}
	if err := db.Create(user).Error; err != nil {
		t.Fatalf("create user failed: %v", err)
	}

	checkAndRechargeUsers(&operation_setting.AutoRechargeSetting{
		Amount:       0,
		WeeklyLimit:  0,
		MonthlyLimit: 0,
	}, 50, 0)

	var gotPool model.QuotaPool
	_ = db.First(&gotPool, pool.Id).Error
	if gotPool.Quota != 80 {
		t.Fatalf("pool quota = %d, want 80", gotPool.Quota)
	}
	var gotUser model.User
	_ = db.First(&gotUser, user.Id).Error
	if gotUser.Quota != 30 {
		t.Fatalf("user quota = %d, want 30", gotUser.Quota)
	}
}

func TestTryAutoRechargeUser_NonDefaultPoolInsufficientSkips(t *testing.T) {
	db, _, cleanup := setupAutoRechargeTestDB(t)
	defer cleanup()

	pool := &model.QuotaPool{
		Name:               "team-low",
		Enabled:            true,
		BaseQuota:          10,
		Quota:              10,
		AutoRechargeAmount: 20,
		WeeklyLimit:        model.QuotaPoolAutoRechargeInherit,
		MonthlyLimit:       model.QuotaPoolAutoRechargeInherit,
		MonthlyRefillDay:   1,
	}
	if err := db.Create(pool).Error; err != nil {
		t.Fatalf("create pool failed: %v", err)
	}
	user := &model.User{
		Id:          1,
		Username:    "low_pool_user",
		Password:    "12345678",
		Status:      common.UserStatusEnabled,
		Quota:       10,
		QuotaPoolId: pool.Id,
	}
	if err := db.Create(user).Error; err != nil {
		t.Fatalf("create user failed: %v", err)
	}

	result := tryAutoRechargeUser(&operation_setting.AutoRechargeSetting{}, user, 50, 50)
	if result.Recharged {
		t.Fatalf("expected recharge to be skipped")
	}
	var gotUser model.User
	_ = db.First(&gotUser, user.Id).Error
	if gotUser.Quota != 10 {
		t.Fatalf("user quota = %d, want unchanged 10", gotUser.Quota)
	}
}

func TestTryAutoRechargeUser_NewUserPoolSkips(t *testing.T) {
	db, _, cleanup := setupAutoRechargeTestDB(t)
	defer cleanup()

	pool := &model.QuotaPool{
		Name:               "new-user",
		PoolType:           model.QuotaPoolTypeNewUser,
		Enabled:            true,
		BaseQuota:          model.QuotaPoolUnlimitedQuota,
		Quota:              model.QuotaPoolUnlimitedQuota,
		AutoRechargeAmount: 20,
		WeeklyLimit:        model.QuotaPoolAutoRechargeInherit,
		MonthlyLimit:       model.QuotaPoolAutoRechargeInherit,
		MonthlyRefillDay:   1,
	}
	if err := db.Create(pool).Error; err != nil {
		t.Fatalf("create pool failed: %v", err)
	}
	user := &model.User{
		Id:          1,
		Username:    "new_user_pool_member",
		Password:    "12345678",
		Status:      common.UserStatusEnabled,
		Quota:       10,
		QuotaPoolId: pool.Id,
	}
	if err := db.Create(user).Error; err != nil {
		t.Fatalf("create user failed: %v", err)
	}

	result := tryAutoRechargeUser(&operation_setting.AutoRechargeSetting{}, user, 50, 50)
	if result.Recharged || result.Reason != "new_user_quota_pool_auto_recharge_disabled" {
		t.Fatalf("unexpected recharge result: %+v", result)
	}
	var gotUser model.User
	_ = db.First(&gotUser, user.Id).Error
	if gotUser.Quota != 10 {
		t.Fatalf("user quota = %d, want unchanged 10", gotUser.Quota)
	}
}

func TestRefillMonthlyQuotaPools(t *testing.T) {
	tests := []struct {
		name              string
		topUp             bool
		quota             int
		expectedQuota     int
		expectedBaseQuota int
		expectedAmount    int
		expectedTxCount   int64
	}{
		{name: "fixed refill adds configured amount", quota: 3000, expectedQuota: 9000, expectedBaseQuota: 9000, expectedAmount: 6000, expectedTxCount: 1},
		{name: "top up fills to target", topUp: true, quota: 3000, expectedQuota: 6000, expectedBaseQuota: 6000, expectedAmount: 3000, expectedTxCount: 1},
		{name: "top up at target only marks month", topUp: true, quota: 6000, expectedQuota: 6000, expectedBaseQuota: 6000, expectedTxCount: 0},
		{name: "top up above target only marks month", topUp: true, quota: 7000, expectedQuota: 7000, expectedBaseQuota: 7000, expectedTxCount: 0},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			db, _, cleanup := setupAutoRechargeTestDB(t)
			defer cleanup()

			now := time.Now()
			currentMonth := now.Year()*100 + int(now.Month())
			pool := &model.QuotaPool{
				Name:                 "monthly",
				Enabled:              true,
				BaseQuota:            test.quota,
				Quota:                test.quota,
				AutoRechargeAmount:   model.QuotaPoolAutoRechargeInherit,
				WeeklyLimit:          model.QuotaPoolAutoRechargeInherit,
				MonthlyLimit:         model.QuotaPoolAutoRechargeInherit,
				MonthlyRefillEnabled: true,
				MonthlyRefillTopUp:   test.topUp,
				MonthlyRefillAmount:  6000,
				MonthlyRefillDay:     1,
				LastRefillMonth:      0,
			}
			if err := db.Create(pool).Error; err != nil {
				t.Fatalf("create pool failed: %v", err)
			}

			refillMonthlyQuotaPools()

			var got model.QuotaPool
			if err := db.First(&got, pool.Id).Error; err != nil {
				t.Fatalf("get pool failed: %v", err)
			}
			if got.Quota != test.expectedQuota {
				t.Fatalf("pool quota = %d, want %d", got.Quota, test.expectedQuota)
			}
			if got.BaseQuota != test.expectedBaseQuota {
				t.Fatalf("pool base quota = %d, want %d", got.BaseQuota, test.expectedBaseQuota)
			}
			if got.LastRefillMonth != currentMonth {
				t.Fatalf("last_refill_month = %d, want %d", got.LastRefillMonth, currentMonth)
			}

			var transactions []model.QuotaPoolTransaction
			if err := db.Where("pool_id = ? AND type = ?", pool.Id, model.QuotaPoolTransactionMonthlyRefill).Find(&transactions).Error; err != nil {
				t.Fatalf("list monthly refill transactions failed: %v", err)
			}
			if int64(len(transactions)) != test.expectedTxCount {
				t.Fatalf("monthly refill transaction count = %d, want %d", len(transactions), test.expectedTxCount)
			}
			if len(transactions) == 1 && transactions[0].Amount != test.expectedAmount {
				t.Fatalf("monthly refill amount = %d, want %d", transactions[0].Amount, test.expectedAmount)
			}

			if test.topUp && test.expectedAmount == 0 {
				loweredQuota := 5000
				if err := db.Model(&model.QuotaPool{}).Where("id = ?", pool.Id).Update("quota", loweredQuota).Error; err != nil {
					t.Fatalf("lower pool quota failed: %v", err)
				}
				refillMonthlyQuotaPools()
				if err := db.First(&got, pool.Id).Error; err != nil {
					t.Fatalf("get pool after second run failed: %v", err)
				}
				if got.Quota != loweredQuota {
					t.Fatalf("pool quota after second run = %d, want unchanged %d", got.Quota, loweredQuota)
				}
			}
		})
	}
}

func TestRefillMonthlyQuotaPoolsRollsBackWhenTransactionCreateFails(t *testing.T) {
	db, _, cleanup := setupAutoRechargeTestDB(t)
	defer cleanup()

	pool := &model.QuotaPool{
		Name:                 "monthly-rollback",
		Enabled:              true,
		BaseQuota:            3000,
		Quota:                3000,
		AutoRechargeAmount:   model.QuotaPoolAutoRechargeInherit,
		WeeklyLimit:          model.QuotaPoolAutoRechargeInherit,
		MonthlyLimit:         model.QuotaPoolAutoRechargeInherit,
		MonthlyRefillEnabled: true,
		MonthlyRefillTopUp:   true,
		MonthlyRefillAmount:  6000,
		MonthlyRefillDay:     1,
	}
	if err := db.Create(pool).Error; err != nil {
		t.Fatalf("create pool failed: %v", err)
	}

	callbackName := "test:fail_monthly_refill_transaction"
	if err := db.Callback().Create().Before("gorm:create").Register(callbackName, func(tx *gorm.DB) {
		if tx.Statement.Schema != nil && tx.Statement.Schema.Name == "QuotaPoolTransaction" {
			tx.AddError(errors.New("forced monthly refill transaction failure"))
		}
	}); err != nil {
		t.Fatalf("register create callback failed: %v", err)
	}
	defer func() {
		_ = db.Callback().Create().Remove(callbackName)
	}()

	refillMonthlyQuotaPools()

	var got model.QuotaPool
	if err := db.First(&got, pool.Id).Error; err != nil {
		t.Fatalf("get pool failed: %v", err)
	}
	if got.Quota != pool.Quota || got.BaseQuota != pool.BaseQuota || got.LastRefillMonth != 0 {
		t.Fatalf("pool should roll back after transaction failure, got quota=%d base=%d month=%d", got.Quota, got.BaseQuota, got.LastRefillMonth)
	}
	var count int64
	if err := db.Model(&model.QuotaPoolTransaction{}).Where("pool_id = ?", pool.Id).Count(&count).Error; err != nil {
		t.Fatalf("count transactions failed: %v", err)
	}
	if count != 0 {
		t.Fatalf("monthly refill transaction count = %d, want 0", count)
	}
}
