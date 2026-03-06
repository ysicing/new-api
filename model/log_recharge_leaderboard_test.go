package model

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

type sqlCaptureLogger struct {
	gormlogger.Interface
	mu   sync.Mutex
	sqls []string
}

func newSQLCaptureLogger() *sqlCaptureLogger {
	return &sqlCaptureLogger{Interface: gormlogger.Default.LogMode(gormlogger.Silent)}
}

func (l *sqlCaptureLogger) LogMode(level gormlogger.LogLevel) gormlogger.Interface {
	l.Interface = l.Interface.LogMode(level)
	return l
}

func (l *sqlCaptureLogger) Trace(ctx context.Context, begin time.Time, fc func() (string, int64), err error) {
	sql, _ := fc()
	l.mu.Lock()
	l.sqls = append(l.sqls, sql)
	l.mu.Unlock()
	l.Interface.Trace(ctx, begin, fc, err)
}

func (l *sqlCaptureLogger) AllSQL() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return strings.Join(l.sqls, "\n")
}

func setupRechargeLeaderboardTestDB(t *testing.T) (*gorm.DB, *sqlCaptureLogger, func()) {
	t.Helper()

	capture := newSQLCaptureLogger()
	dsn := fmt.Sprintf("file:recharge_leaderboard_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{Logger: capture})
	if err != nil {
		t.Fatalf("open sqlite db failed: %v", err)
	}

	createUsers := `CREATE TABLE users (
		id INTEGER PRIMARY KEY,
		username TEXT,
		quota INTEGER,
		used_quota INTEGER
	);`
	if err := db.Exec(createUsers).Error; err != nil {
		t.Fatalf("create users table failed: %v", err)
	}

	createLogs := `CREATE TABLE logs (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		user_id INTEGER,
		created_at BIGINT,
		type INTEGER,
		content TEXT,
		quota INTEGER
	);`
	if err := db.Exec(createLogs).Error; err != nil {
		t.Fatalf("create logs table failed: %v", err)
	}

	oldLogDB := LOG_DB
	LOG_DB = db

	cleanup := func() {
		LOG_DB = oldLogDB
	}

	return db, capture, cleanup
}

func TestGetRechargeLeaderboard_AvoidsSlowLegacyQueryPatterns(t *testing.T) {
	db, capture, cleanup := setupRechargeLeaderboardTestDB(t)
	defer cleanup()

	now := time.Now().Unix()

	if err := db.Exec(`INSERT INTO users (id, username, quota, used_quota) VALUES
		(1, 'alice', 1000, 100),
		(2, 'bob', 2000, 200)`).Error; err != nil {
		t.Fatalf("insert users failed: %v", err)
	}

	if err := db.Exec(`INSERT INTO logs (user_id, created_at, type, content, quota) VALUES
		(1, ?, ?, '系统自动赠送 100', 0),
		(1, ?, ?, '管理员(ID:9)添加100临时额度', 0),
		(1, ?, ?, 'consume', 30),
		(2, ?, ?, '系统自动赠送 100', 0),
		(2, ?, ?, '管理员(ID:9)添加100临时额度', 0),
		(2, ?, ?, 'consume', 20)`,
		now, LogTypeSystem,
		now, LogTypeManage,
		now, LogTypeConsume,
		now, LogTypeSystem,
		now, LogTypeManage,
		now, LogTypeConsume,
	).Error; err != nil {
		t.Fatalf("insert logs failed: %v", err)
	}

	_, err := GetRechargeLeaderboard(10)
	if err != nil {
		t.Fatalf("GetRechargeLeaderboard returned error: %v", err)
	}

	sqlText := strings.ToLower(capture.AllSQL())
	if strings.Contains(sqlText, "users.username in") {
		t.Fatalf("found legacy slow pattern 'users.username IN' in SQL:\n%s", capture.AllSQL())
	}

	legacyOrPattern := regexp.MustCompile(`\)\s+or\s+\(`)
	if legacyOrPattern.MatchString(sqlText) {
		t.Fatalf("found legacy OR predicate in SQL:\n%s", capture.AllSQL())
	}
}
func TestGetRechargeLeaderboard_ReturnsExpectedRankingAndQuota(t *testing.T) {
	db, _, cleanup := setupRechargeLeaderboardTestDB(t)
	defer cleanup()

	now := time.Now().Unix()
	oldTs := now - 10*24*60*60

	if err := db.Exec(`INSERT INTO users (id, username, quota, used_quota) VALUES
		(1, 'alice', 1000, 100),
		(2, 'bob', 2000, 200),
		(3, 'carol', 3000, 300)`).Error; err != nil {
		t.Fatalf("insert users failed: %v", err)
	}

	if err := db.Exec(`INSERT INTO logs (user_id, created_at, type, content, quota) VALUES
		(1, ?, ?, '系统自动赠送 100', 0),
		(1, ?, ?, '系统自动赠送 100', 0),
		(1, ?, ?, '管理员(ID:9)添加100临时额度', 0),
		(2, ?, ?, '系统自动赠送 100', 0),
		(2, ?, ?, '管理员(ID:9)添加100临时额度', 0),
		(2, ?, ?, '管理员(ID:9)添加100临时额度', 0),
		(2, ?, ?, '管理员(ID:9)添加100临时额度', 0),
		(3, ?, ?, '管理员(ID:9)添加100临时额度', 0),
		(1, ?, ?, 'consume', 20),
		(2, ?, ?, 'consume', 25),
		(3, ?, ?, 'consume', 5),
		(2, ?, ?, '系统自动赠送 100', 0),
		(1, ?, ?, '普通系统日志', 0)`,
		now, LogTypeSystem,
		now, LogTypeSystem,
		now, LogTypeManage,
		now, LogTypeSystem,
		now, LogTypeManage,
		now, LogTypeManage,
		now, LogTypeManage,
		now, LogTypeManage,
		now, LogTypeConsume,
		now, LogTypeConsume,
		now, LogTypeConsume,
		oldTs, LogTypeSystem,
		now, LogTypeSystem,
	).Error; err != nil {
		t.Fatalf("insert logs failed: %v", err)
	}

	results, err := GetRechargeLeaderboard(2)
	if err != nil {
		t.Fatalf("GetRechargeLeaderboard returned error: %v", err)
	}

	if len(results) != 2 {
		t.Fatalf("unexpected result length, got %d want %d", len(results), 2)
	}

	if results[0].UserId != 2 {
		t.Fatalf("unexpected top1 user_id, got %d want %d", results[0].UserId, 2)
	}
	if results[0].TotalCount != 4 || results[0].AutoRechargeCount != 1 || results[0].TempQuotaCount != 3 {
		t.Fatalf("unexpected top1 counts: %+v", results[0])
	}
	if results[0].UsedQuota != 25 {
		t.Fatalf("unexpected top1 used_quota, got %d want %d", results[0].UsedQuota, 25)
	}
	if results[0].Username != "bob" {
		t.Fatalf("unexpected top1 username, got %q want %q", results[0].Username, "bob")
	}

	if results[1].UserId != 1 {
		t.Fatalf("unexpected top2 user_id, got %d want %d", results[1].UserId, 1)
	}
	if results[1].TotalCount != 3 || results[1].AutoRechargeCount != 2 || results[1].TempQuotaCount != 1 {
		t.Fatalf("unexpected top2 counts: %+v", results[1])
	}
	if results[1].UsedQuota != 20 {
		t.Fatalf("unexpected top2 used_quota, got %d want %d", results[1].UsedQuota, 20)
	}
	if results[1].Username != "alice" {
		t.Fatalf("unexpected top2 username, got %q want %q", results[1].Username, "alice")
	}
}

func TestGetRechargeLeaderboard_DoesNotLoseSlotsWhenTopDeletedUserExists(t *testing.T) {
	db, _, cleanup := setupRechargeLeaderboardTestDB(t)
	defer cleanup()

	now := time.Now().Unix()

	if err := db.Exec(`INSERT INTO users (id, username, quota, used_quota) VALUES
		(1, 'alice', 1000, 100),
		(2, 'bob', 2000, 200),
		(3, 'carol', 3000, 300)`).Error; err != nil {
		t.Fatalf("insert users failed: %v", err)
	}

	if err := db.Exec(`INSERT INTO logs (user_id, created_at, type, content, quota) VALUES
		(999, ?, ?, '管理员(ID:9)添加100临时额度', 0),
		(999, ?, ?, '管理员(ID:9)添加100临时额度', 0),
		(999, ?, ?, '管理员(ID:9)添加100临时额度', 0),
		(999, ?, ?, '管理员(ID:9)添加100临时额度', 0),
		(999, ?, ?, '管理员(ID:9)添加100临时额度', 0),
		(1, ?, ?, '系统自动赠送 100', 0),
		(1, ?, ?, '系统自动赠送 100', 0),
		(1, ?, ?, '管理员(ID:9)添加100临时额度', 0),
		(1, ?, ?, '管理员(ID:9)添加100临时额度', 0),
		(2, ?, ?, '系统自动赠送 100', 0),
		(2, ?, ?, '管理员(ID:9)添加100临时额度', 0),
		(2, ?, ?, '管理员(ID:9)添加100临时额度', 0),
		(3, ?, ?, '管理员(ID:9)添加100临时额度', 0),
		(1, ?, ?, 'consume', 20),
		(2, ?, ?, 'consume', 10)`,
		now, LogTypeManage,
		now, LogTypeManage,
		now, LogTypeManage,
		now, LogTypeManage,
		now, LogTypeManage,
		now, LogTypeSystem,
		now, LogTypeSystem,
		now, LogTypeManage,
		now, LogTypeManage,
		now, LogTypeSystem,
		now, LogTypeManage,
		now, LogTypeManage,
		now, LogTypeManage,
		now, LogTypeConsume,
		now, LogTypeConsume,
	).Error; err != nil {
		t.Fatalf("insert logs failed: %v", err)
	}

	results, err := GetRechargeLeaderboard(2)
	if err != nil {
		t.Fatalf("GetRechargeLeaderboard returned error: %v", err)
	}

	if len(results) != 2 {
		t.Fatalf("unexpected result length, got %d want %d", len(results), 2)
	}

	if results[0].UserId != 1 || results[1].UserId != 2 {
		t.Fatalf("unexpected ranking when deleted user exists: got [%d,%d] want [1,2]", results[0].UserId, results[1].UserId)
	}
}

func TestGetRechargeLeaderboard_TieBreakByUserIDAscending(t *testing.T) {
	db, _, cleanup := setupRechargeLeaderboardTestDB(t)
	defer cleanup()

	now := time.Now().Unix()

	if err := db.Exec(`INSERT INTO users (id, username, quota, used_quota) VALUES
		(1, 'alice', 1000, 100),
		(2, 'bob', 2000, 200),
		(3, 'carol', 3000, 300)`).Error; err != nil {
		t.Fatalf("insert users failed: %v", err)
	}

	if err := db.Exec(`INSERT INTO logs (user_id, created_at, type, content, quota) VALUES
		(2, ?, ?, '系统自动赠送 100', 0),
		(1, ?, ?, '管理员(ID:9)添加100临时额度', 0),
		(3, ?, ?, '管理员(ID:9)添加100临时额度', 0)`,
		now, LogTypeSystem,
		now, LogTypeManage,
		now, LogTypeManage,
	).Error; err != nil {
		t.Fatalf("insert logs failed: %v", err)
	}

	results, err := GetRechargeLeaderboard(2)
	if err != nil {
		t.Fatalf("GetRechargeLeaderboard returned error: %v", err)
	}

	if len(results) != 2 {
		t.Fatalf("unexpected result length, got %d want %d", len(results), 2)
	}

	if results[0].UserId != 1 || results[1].UserId != 2 {
		t.Fatalf("unexpected tie-break order: got [%d,%d] want [1,2]", results[0].UserId, results[1].UserId)
	}
	if results[0].TotalCount != 1 || results[1].TotalCount != 1 {
		t.Fatalf("unexpected total_count in tie-break results: %+v", results)
	}
}
