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

func TestGetTopUsersPushesRankingAndLimitIntoSQL(t *testing.T) {
	mainDB, logDB := setupICodeStatsTest(t)
	require.NoError(t, mainDB.Create(&User{
		Id: 1, Username: "alice", Password: "password", AffCode: "top-sql-alice",
	}).Error)
	require.NoError(t, logDB.Create(&[]Log{
		{UserId: 1, Type: LogTypeConsume, CreatedAt: 100, ModelName: "gpt-5", Quota: 30},
		{UserId: 1, Type: LogTypeConsume, CreatedAt: 101, ModelName: "claude-4", Quota: 20},
	}).Error)
	capture := captureStatsLogDB(t, logDB)
	capture.reset()

	_, err := GetTopUsers(90, 120, "", 10)

	require.NoError(t, err)
	queries := capture.matching("from `logs`")
	require.Len(t, queries, 1)
	assert.Contains(t, queries[0], "group by `user_id`")
	assert.Contains(t, queries[0], "order by used_quota desc")
	assert.Contains(t, queries[0], "limit 10")
	assert.NotContains(t, queries[0], "group by user_id, model_name")
}

func TestGetRechargeLeaderboardScansRechargeCountsOnce(t *testing.T) {
	_, logDB := setupICodeStatsTest(t)
	capture := captureStatsLogDB(t, logDB)
	capture.reset()

	_, err := GetRechargeLeaderboard(10)

	require.NoError(t, err)
	queries := capture.matching("from `logs`")
	require.Len(t, queries, 1)
	assert.Contains(t, queries[0], "group by `user_id`")
}
