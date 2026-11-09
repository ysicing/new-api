package model

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

type statsSQLCapture struct {
	gormlogger.Interface
	mu         sync.Mutex
	statements []string
}

func (capture *statsSQLCapture) Trace(
	_ context.Context,
	_ time.Time,
	query func() (string, int64),
	_ error,
) {
	sql, _ := query()
	capture.mu.Lock()
	capture.statements = append(capture.statements, strings.ToLower(sql))
	capture.mu.Unlock()
}

func (capture *statsSQLCapture) reset() {
	capture.mu.Lock()
	capture.statements = nil
	capture.mu.Unlock()
}

func (capture *statsSQLCapture) matching(fragment string) []string {
	capture.mu.Lock()
	defer capture.mu.Unlock()
	items := make([]string, 0)
	for _, statement := range capture.statements {
		if strings.Contains(statement, fragment) {
			items = append(items, statement)
		}
	}
	return items
}

func captureStatsLogDB(t *testing.T, logDB *gorm.DB) *statsSQLCapture {
	t.Helper()
	capture := &statsSQLCapture{Interface: logDB.Logger}
	LOG_DB = logDB.Session(&gorm.Session{Logger: capture})
	return capture
}

func TestGetTopUsersDoesNotQueryLogs(t *testing.T) {
	mainDB, logDB := setupICodeStatsTest(t)
	now := time.Date(2026, time.August, 27, 12, 30, 0, 0, time.Local)
	start := now.AddDate(0, 0, -3)
	require.NoError(t, mainDB.Create(&User{
		Id: 1, Username: "alice", Password: "password", AffCode: "top-sql-alice",
	}).Error)
	require.NoError(t, mainDB.Create(&QuotaData{
		UserID: 1, Username: "alice", ModelName: "gpt-5",
		CreatedAt: now.Add(-24 * time.Hour).Truncate(time.Hour).Unix(), Quota: 30,
	}).Error)
	require.NoError(t, logDB.Create(&Log{
		UserId: 1, Type: LogTypeConsume, CreatedAt: now.Add(-30 * time.Minute).Unix(), ModelName: "claude-4", Quota: 20,
	}).Error)
	capture := captureStatsLogDB(t, logDB)
	capture.reset()

	_, err := GetTopUsers(start.Unix(), now.Unix(), "", 10)

	require.NoError(t, err)
	queries := capture.matching("from `logs`")
	assert.Empty(t, queries)
}

func TestGetRechargeLeaderboardDoesNotQueryLogs(t *testing.T) {
	_, logDB := setupICodeStatsTest(t)
	capture := captureStatsLogDB(t, logDB)
	capture.reset()

	_, err := GetRechargeLeaderboard(10)

	require.NoError(t, err)
	queries := capture.matching("from `logs`")
	assert.Empty(t, queries)
}
