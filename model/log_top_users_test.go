package model

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

func setupTopUsersTestDB(t *testing.T) (*gorm.DB, func()) {
	return setupTopUsersTestDBWithLogger(t, gormlogger.Default.LogMode(gormlogger.Silent))
}

func setupTopUsersTestDBWithLogger(t *testing.T, logger gormlogger.Interface) (*gorm.DB, func()) {
	t.Helper()
	initCol()

	dsn := fmt.Sprintf("file:top_users_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{Logger: logger})
	if err != nil {
		t.Fatalf("open sqlite db failed: %v", err)
	}

	createTopUsersMainTables(t, db)
	createTopUsersLogsTable(t, db)

	oldDB := DB
	oldLogDB := LOG_DB
	DB = db
	LOG_DB = db

	cleanup := func() {
		DB = oldDB
		LOG_DB = oldLogDB
	}

	return db, cleanup
}

func setupTopUsersSeparateDBs(t *testing.T) (*gorm.DB, *gorm.DB, func()) {
	t.Helper()
	initCol()

	mainDSN := fmt.Sprintf("file:top_users_main_%d?mode=memory&cache=shared", time.Now().UnixNano())
	mainDB, err := gorm.Open(sqlite.Open(mainDSN), &gorm.Config{Logger: gormlogger.Default.LogMode(gormlogger.Silent)})
	if err != nil {
		t.Fatalf("open main sqlite db failed: %v", err)
	}

	logDSN := fmt.Sprintf("file:top_users_log_%d?mode=memory&cache=shared", time.Now().UnixNano())
	logDB, err := gorm.Open(sqlite.Open(logDSN), &gorm.Config{Logger: gormlogger.Default.LogMode(gormlogger.Silent)})
	if err != nil {
		t.Fatalf("open log sqlite db failed: %v", err)
	}

	createTopUsersMainTables(t, mainDB)
	createTopUsersLogsTable(t, logDB)

	oldDB := DB
	oldLogDB := LOG_DB
	DB = mainDB
	LOG_DB = logDB

	cleanup := func() {
		DB = oldDB
		LOG_DB = oldLogDB
	}

	return mainDB, logDB, cleanup
}

func createTopUsersMainTables(t *testing.T, db *gorm.DB) {
	t.Helper()

	createUsers := `CREATE TABLE users (
		id INTEGER PRIMARY KEY,
		username TEXT,
		quota INTEGER,
		used_quota INTEGER
	);`
	if err := db.Exec(createUsers).Error; err != nil {
		t.Fatalf("create users table failed: %v", err)
	}

	createQuotaData := `CREATE TABLE quota_data (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		user_id INTEGER,
		username TEXT,
		model_name TEXT,
		created_at BIGINT,
		token_used INTEGER,
		count INTEGER,
		quota INTEGER
	);`
	if err := db.Exec(createQuotaData).Error; err != nil {
		t.Fatalf("create quota_data table failed: %v", err)
	}
}

func createTopUsersLogsTable(t *testing.T, db *gorm.DB) {
	t.Helper()

	createLogs := `CREATE TABLE logs (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		user_id INTEGER,
		created_at BIGINT,
		type INTEGER,
		username TEXT,
		model_name TEXT,
		quota INTEGER,
		channel_id INTEGER,
		"group" TEXT
	);`
	if err := db.Exec(createLogs).Error; err != nil {
		t.Fatalf("create logs table failed: %v", err)
	}
}

func TestGetTopUsersMergesQuotaDataWithCurrentHourLogs(t *testing.T) {
	db, cleanup := setupTopUsersTestDB(t)
	defer cleanup()

	currentHourStart := time.Now().Unix()
	currentHourStart = currentHourStart - (currentHourStart % 3600)
	settledAt := currentHourStart - 3600
	currentAt := currentHourStart + 60

	if err := db.Exec(`INSERT INTO users (id, username, quota, used_quota) VALUES
		(1, 'alice', 1000, 600),
		(2, 'bob', 2000, 300)`).Error; err != nil {
		t.Fatalf("insert users failed: %v", err)
	}

	if err := db.Exec(`INSERT INTO quota_data (user_id, username, model_name, created_at, token_used, count, quota) VALUES
		(1, 'alice', 'gpt-4o', ?, 10, 1, 100),
		(1, 'alice', 'claude-3-5-sonnet', ?, 10, 1, 80),
		(1, 'alice', 'custom-model', ?, 10, 1, 20),
		(2, 'bob', 'qwen-max', ?, 10, 1, 120),
		(2, 'bob', 'gemini-2.0-flash', ?, 10, 1, 60),
		(1, 'alice', 'deepseek-chat', ?, 10, 1, 500)`,
		settledAt, settledAt, settledAt, settledAt, settledAt, currentAt,
	).Error; err != nil {
		t.Fatalf("insert quota_data failed: %v", err)
	}

	if err := db.Exec(`INSERT INTO logs (user_id, created_at, type, username, model_name, quota, channel_id, "group") VALUES
		(1, ?, ?, 'alice', 'deepseek-chat', 900, 1, 'default')`,
		currentAt, LogTypeConsume,
	).Error; err != nil {
		t.Fatalf("insert logs failed: %v", err)
	}

	results, err := GetTopUsers(settledAt-10, currentAt+10, "", 10)
	if err != nil {
		t.Fatalf("GetTopUsers returned error: %v", err)
	}

	if len(results) != 2 {
		t.Fatalf("unexpected result length, got %d want %d", len(results), 2)
	}

	if results[0].UserId != 1 || results[0].Username != "alice" {
		t.Fatalf("unexpected top1 user: %+v", results[0])
	}
	if results[0].UsedQuota != 1100 || results[0].GptQuota != 100 || results[0].ClaudeQuota != 80 || results[0].DeepSeekQuota != 900 || results[0].OtherQuota != 20 {
		t.Fatalf("unexpected top1 quota split: %+v", results[0])
	}

	if results[1].UserId != 2 || results[1].UsedQuota != 180 || results[1].QwenQuota != 120 || results[1].GeminiQuota != 60 {
		t.Fatalf("unexpected top2 quota split: %+v", results[1])
	}
}

func TestGetTopUsersUsesUnionQueryWithSQLLimit(t *testing.T) {
	capture := newSQLCaptureLogger()
	db, cleanup := setupTopUsersTestDBWithLogger(t, capture)
	defer cleanup()

	currentHourStart := time.Now().Unix()
	currentHourStart = currentHourStart - (currentHourStart % 3600)
	settledAt := currentHourStart - 3600
	currentAt := currentHourStart + 60

	if err := db.Exec(`INSERT INTO users (id, username, quota, used_quota) VALUES
		(1, 'alice', 1000, 600),
		(2, 'bob', 2000, 300)`).Error; err != nil {
		t.Fatalf("insert users failed: %v", err)
	}

	if err := db.Exec(`INSERT INTO quota_data (user_id, username, model_name, created_at, token_used, count, quota) VALUES
		(1, 'alice', 'gpt-4o', ?, 10, 1, 100),
		(2, 'bob', 'qwen-max', ?, 10, 1, 120)`,
		settledAt, settledAt,
	).Error; err != nil {
		t.Fatalf("insert quota_data failed: %v", err)
	}

	if err := db.Exec(`INSERT INTO logs (user_id, created_at, type, username, model_name, quota, channel_id, "group") VALUES
		(1, ?, ?, 'alice', 'claude-3-5-sonnet', 90, 1, 'default')`,
		currentAt, LogTypeConsume,
	).Error; err != nil {
		t.Fatalf("insert logs failed: %v", err)
	}

	results, err := GetTopUsers(settledAt-10, currentAt+10, "", 1)
	if err != nil {
		t.Fatalf("GetTopUsers returned error: %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("unexpected result length, got %d want %d", len(results), 1)
	}

	sqlText := strings.ToLower(capture.AllSQL())
	if !strings.Contains(sqlText, " union all ") {
		t.Fatalf("expected top users query to use UNION ALL, SQL:\n%s", capture.AllSQL())
	}
	if !strings.Contains(sqlText, " limit 1") {
		t.Fatalf("expected top users query to push limit into SQL, SQL:\n%s", capture.AllSQL())
	}
}

func TestGetTopUsersFallsBackToLogsWhenQuotaDataEmpty(t *testing.T) {
	db, cleanup := setupTopUsersTestDB(t)
	defer cleanup()

	currentHourStart := time.Now().Unix()
	currentHourStart = currentHourStart - (currentHourStart % 3600)
	oldAt := currentHourStart - 3600

	if err := db.Exec(`INSERT INTO users (id, username, quota, used_quota) VALUES
		(1, 'alice', 1000, 500),
		(2, 'bob', 2000, 400)`).Error; err != nil {
		t.Fatalf("insert users failed: %v", err)
	}

	if err := db.Exec(`INSERT INTO logs (user_id, created_at, type, username, model_name, quota, channel_id, "group") VALUES
		(1, ?, ?, 'alice', 'deepseek-chat', 70, 1, 'default'),
		(1, ?, ?, 'alice', 'Deep-Seek-R1', 30, 1, 'default'),
		(1, ?, ?, 'alice', 'o3-mini', 40, 1, 'default'),
		(2, ?, ?, 'bob', 'gemini-pro', 60, 1, 'default')`,
		oldAt, LogTypeConsume,
		oldAt, LogTypeConsume,
		oldAt, LogTypeConsume,
		oldAt, LogTypeConsume,
	).Error; err != nil {
		t.Fatalf("insert logs failed: %v", err)
	}

	results, err := GetTopUsers(oldAt-10, oldAt+10, "", 10)
	if err != nil {
		t.Fatalf("GetTopUsers returned error: %v", err)
	}

	if len(results) != 2 {
		t.Fatalf("unexpected result length, got %d want %d", len(results), 2)
	}
	if results[0].UserId != 1 || results[0].UsedQuota != 140 || results[0].DeepSeekQuota != 100 || results[0].GptQuota != 40 {
		t.Fatalf("unexpected top1 logs quota split: %+v", results[0])
	}
	if results[1].UserId != 2 || results[1].GeminiQuota != 60 {
		t.Fatalf("unexpected top2 logs quota split: %+v", results[1])
	}
}

func TestGetTopUsersMergedAcrossSeparateDBs(t *testing.T) {
	mainDB, logDB, cleanup := setupTopUsersSeparateDBs(t)
	defer cleanup()

	currentHourStart := time.Now().Unix()
	currentHourStart = currentHourStart - (currentHourStart % 3600)
	settledAt := currentHourStart - 3600
	currentAt := currentHourStart + 60

	if err := mainDB.Exec(`INSERT INTO users (id, username, quota, used_quota) VALUES
		(1, 'alice', 1000, 600),
		(2, 'bob', 2000, 300)`).Error; err != nil {
		t.Fatalf("insert users failed: %v", err)
	}

	if err := mainDB.Exec(`INSERT INTO quota_data (user_id, username, model_name, created_at, token_used, count, quota) VALUES
		(1, 'alice', 'gpt-4o', ?, 10, 1, 100),
		(2, 'bob', 'qwen-max', ?, 10, 1, 120)`,
		settledAt, settledAt,
	).Error; err != nil {
		t.Fatalf("insert quota_data failed: %v", err)
	}

	if err := logDB.Exec(`INSERT INTO logs (user_id, created_at, type, username, model_name, quota, channel_id, "group") VALUES
		(1, ?, ?, 'alice', 'claude-3-5-sonnet', 90, 1, 'default'),
		(2, ?, ?, 'bob', 'gemini-pro', 20, 1, 'default')`,
		currentAt, LogTypeConsume,
		currentAt, LogTypeConsume,
	).Error; err != nil {
		t.Fatalf("insert logs failed: %v", err)
	}

	results, err := GetTopUsers(settledAt-10, currentAt+10, "", 10)
	if err != nil {
		t.Fatalf("GetTopUsers returned error: %v", err)
	}

	if len(results) != 2 {
		t.Fatalf("unexpected result length, got %d want %d", len(results), 2)
	}
	if results[0].UserId != 1 || results[0].UsedQuota != 190 || results[0].GptQuota != 100 || results[0].ClaudeQuota != 90 {
		t.Fatalf("unexpected top1 separate db quota split: %+v", results[0])
	}
	if results[1].UserId != 2 || results[1].UsedQuota != 140 || results[1].QwenQuota != 120 || results[1].GeminiQuota != 20 {
		t.Fatalf("unexpected top2 separate db quota split: %+v", results[1])
	}
}

func TestGetTopUsersAcrossSeparateDBsFiltersMissingUsers(t *testing.T) {
	mainDB, logDB, cleanup := setupTopUsersSeparateDBs(t)
	defer cleanup()

	currentHourStart := time.Now().Unix()
	currentHourStart = currentHourStart - (currentHourStart % 3600)
	currentAt := currentHourStart + 60

	if err := mainDB.Exec(`INSERT INTO users (id, username, quota, used_quota) VALUES
		(1, 'alice', 1000, 600)`).Error; err != nil {
		t.Fatalf("insert users failed: %v", err)
	}

	if err := logDB.Exec(`INSERT INTO logs (user_id, created_at, type, username, model_name, quota, channel_id, "group") VALUES
		(999, ?, ?, 'deleted', 'gpt-4o', 900, 1, 'default'),
		(1, ?, ?, 'alice', 'claude-3-5-sonnet', 90, 1, 'default')`,
		currentAt, LogTypeConsume,
		currentAt, LogTypeConsume,
	).Error; err != nil {
		t.Fatalf("insert logs failed: %v", err)
	}

	results, err := GetTopUsers(currentHourStart, currentAt+10, "", 10)
	if err != nil {
		t.Fatalf("GetTopUsers returned error: %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("unexpected result length, got %d want %d: %+v", len(results), 1, results)
	}
	if results[0].UserId != 1 || results[0].Username != "alice" || results[0].ClaudeQuota != 90 {
		t.Fatalf("unexpected filtered user result: %+v", results[0])
	}
}

func TestGetTopUsersSettledRangeFallsBackToLogsWhenQuotaDataEmpty(t *testing.T) {
	db, cleanup := setupTopUsersTestDB(t)
	defer cleanup()

	currentHourStart := time.Now().Unix()
	currentHourStart = currentHourStart - (currentHourStart % 3600)
	settledAt := currentHourStart - 3600
	currentAt := currentHourStart + 60

	if err := db.Exec(`INSERT INTO users (id, username, quota, used_quota) VALUES
		(1, 'alice', 1000, 600),
		(2, 'bob', 2000, 300)`).Error; err != nil {
		t.Fatalf("insert users failed: %v", err)
	}

	if err := db.Exec(`INSERT INTO logs (user_id, created_at, type, username, model_name, quota, channel_id, "group") VALUES
		(1, ?, ?, 'alice', 'deepseek-chat', 70, 1, 'default'),
		(1, ?, ?, 'alice', 'claude-3-5-sonnet', 30, 1, 'default'),
		(2, ?, ?, 'bob', 'qwen-max', 80, 1, 'default')`,
		settledAt, LogTypeConsume,
		currentAt, LogTypeConsume,
		currentAt, LogTypeConsume,
	).Error; err != nil {
		t.Fatalf("insert logs failed: %v", err)
	}

	results, err := GetTopUsers(settledAt-10, currentAt+10, "", 10)
	if err != nil {
		t.Fatalf("GetTopUsers returned error: %v", err)
	}

	if len(results) != 2 {
		t.Fatalf("unexpected result length, got %d want %d", len(results), 2)
	}
	if results[0].UserId != 1 || results[0].UsedQuota != 100 || results[0].DeepSeekQuota != 70 || results[0].ClaudeQuota != 30 {
		t.Fatalf("unexpected top1 fallback quota split: %+v", results[0])
	}
	if results[1].UserId != 2 || results[1].UsedQuota != 80 || results[1].QwenQuota != 80 {
		t.Fatalf("unexpected top2 fallback quota split: %+v", results[1])
	}
}

func TestGetTopUsersFiltersModelNamePattern(t *testing.T) {
	db, cleanup := setupTopUsersTestDB(t)
	defer cleanup()

	currentHourStart := time.Now().Unix()
	currentHourStart = currentHourStart - (currentHourStart % 3600)
	settledAt := currentHourStart - 3600
	currentAt := currentHourStart + 60

	if err := db.Exec(`INSERT INTO users (id, username, quota, used_quota) VALUES
		(1, 'alice', 1000, 600),
		(2, 'bob', 2000, 300)`).Error; err != nil {
		t.Fatalf("insert users failed: %v", err)
	}

	if err := db.Exec(`INSERT INTO quota_data (user_id, username, model_name, created_at, token_used, count, quota) VALUES
		(1, 'alice', 'gpt-4o', ?, 10, 1, 100),
		(1, 'alice', 'claude-3-5-sonnet', ?, 10, 1, 80)`,
		settledAt, settledAt,
	).Error; err != nil {
		t.Fatalf("insert quota_data failed: %v", err)
	}

	if err := db.Exec(`INSERT INTO logs (user_id, created_at, type, username, model_name, quota, channel_id, "group") VALUES
		(1, ?, ?, 'alice', 'gpt-4o-mini', 40, 1, 'default'),
		(2, ?, ?, 'bob', 'claude-3-haiku', 90, 1, 'default')`,
		currentAt, LogTypeConsume,
		currentAt, LogTypeConsume,
	).Error; err != nil {
		t.Fatalf("insert logs failed: %v", err)
	}

	results, err := GetTopUsers(settledAt-10, currentAt+10, "%gpt%", 10)
	if err != nil {
		t.Fatalf("GetTopUsers returned error: %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("unexpected result length, got %d want %d", len(results), 1)
	}
	if results[0].UserId != 1 || results[0].UsedQuota != 140 || results[0].GptQuota != 140 {
		t.Fatalf("unexpected filtered quota split: %+v", results[0])
	}
}
