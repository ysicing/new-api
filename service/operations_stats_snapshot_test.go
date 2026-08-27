package service

import (
	"context"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOperationsStatsSnapshotRefreshesMonthlyOnlyAfterDayChanges(t *testing.T) {
	truncate(t)
	now := time.Date(2026, time.August, 27, 12, 0, 0, 0, time.Local)
	user := model.User{
		Id: 1, Username: "alice", Password: "password", AffCode: "stats-snapshot-alice",
		Status: common.UserStatusEnabled,
	}
	require.NoError(t, model.DB.Create(&user).Error)
	require.NoError(t, model.DB.Create(&model.QuotaData{
		UserID: user.Id, Username: user.Username, ModelName: "gpt-5",
		CreatedAt: now.Add(-time.Hour).Truncate(time.Hour).Unix(), Quota: 30,
	}).Error)
	require.NoError(t, model.LOG_DB.Create(&[]model.Log{
		{UserId: user.Id, Type: model.LogTypeConsume, CreatedAt: now.Add(-time.Hour).Unix(), ModelName: "gpt-5", Quota: 30},
		{UserId: user.Id, Type: model.LogTypeSystem, CreatedAt: now.Add(-time.Hour).Unix(), Content: "系统自动赠送 100"},
	}).Error)
	previousMonthlyGeneratedAt := now.Add(-time.Hour).Unix()
	previous := &OperationsStatsSnapshot{
		Version: operationsStatsSnapshotVersion,
		MonthlyTopUsers: OperationsStatsUserSection{
			GeneratedAt: previousMonthlyGeneratedAt,
			Items:       []model.UserQuotaStat{{UserId: 99, Username: "preserved"}},
		},
	}

	sameDay, err := buildOperationsStatsSnapshot(context.Background(), now, previous)

	require.NoError(t, err)
	assert.Equal(t, previousMonthlyGeneratedAt, sameDay.MonthlyTopUsers.GeneratedAt)
	require.Len(t, sameDay.MonthlyTopUsers.Items, 1)
	assert.Equal(t, 99, sameDay.MonthlyTopUsers.Items[0].UserId)
	assert.Equal(t, now.Unix(), sameDay.WeeklyTopUsers.GeneratedAt)
	assert.Equal(t, now.Unix(), sameDay.RechargeLeaderboard.GeneratedAt)

	nextDay := now.AddDate(0, 0, 1)
	refreshed, err := buildOperationsStatsSnapshot(context.Background(), nextDay, sameDay)

	require.NoError(t, err)
	assert.Equal(t, nextDay.Unix(), refreshed.MonthlyTopUsers.GeneratedAt)
	require.Len(t, refreshed.MonthlyTopUsers.Items, 1)
	assert.Equal(t, user.Id, refreshed.MonthlyTopUsers.Items[0].UserId)
}

func TestOperationsStatsRefreshUsesThirtyMinuteSystemTask(t *testing.T) {
	handler := operationsStatsRefreshHandler{}

	assert.Equal(t, model.SystemTaskTypeOperationsStatsRefresh, handler.Type())
	assert.True(t, handler.Enabled())
	assert.Equal(t, 30*time.Minute, handler.Interval())
}
