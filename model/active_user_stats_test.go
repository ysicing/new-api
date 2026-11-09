package model

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetActiveUserStatsCountsDistinctUsersPerLocalDay(t *testing.T) {
	db, _ := setupICodeStatsTest(t)
	location := time.FixedZone("UTC+8", 8*60*60)
	start := time.Date(2026, time.September, 1, 0, 0, 0, 0, location)
	end := time.Date(2026, time.September, 3, 23, 59, 59, 0, location)
	require.NoError(t, db.Create(&[]QuotaData{
		{UserID: 1, Username: "alice", CreatedAt: time.Date(2026, time.September, 1, 1, 0, 0, 0, location).Unix(), Count: 2},
		{UserID: 1, Username: "alice", CreatedAt: time.Date(2026, time.September, 1, 9, 0, 0, 0, location).Unix(), Count: 3},
		{UserID: 2, Username: "bob", CreatedAt: time.Date(2026, time.September, 1, 23, 0, 0, 0, location).Unix(), Count: 1},
		{UserID: 1, Username: "alice", CreatedAt: time.Date(2026, time.September, 2, 2, 0, 0, 0, location).Unix(), Count: 1},
		{UserID: 3, Username: "ignored", CreatedAt: time.Date(2026, time.September, 2, 3, 0, 0, 0, location).Unix(), Count: 0},
	}).Error)

	stats, err := GetActiveUserStats(start.Unix(), end.Unix(), location)

	require.NoError(t, err)
	assert.Equal(t, 2, stats.TotalActiveUsers)
	assert.Equal(t, []DailyActiveUserStat{
		{Date: "2026-09-01", ActiveUsers: 2},
		{Date: "2026-09-02", ActiveUsers: 1},
		{Date: "2026-09-03", ActiveUsers: 0},
	}, stats.Daily)
}

func TestGetActiveUserStatsIncludesTheIntersectingFirstHourBucket(t *testing.T) {
	db, _ := setupICodeStatsTest(t)
	location := time.FixedZone("UTC+8", 8*60*60)
	bucketStart := time.Date(2026, time.September, 1, 12, 0, 0, 0, location)
	require.NoError(t, db.Create(&QuotaData{
		UserID: 7, Username: "bucket-user", CreatedAt: bucketStart.Unix(), Count: 1,
	}).Error)

	stats, err := GetActiveUserStats(
		bucketStart.Add(30*time.Minute).Unix(),
		bucketStart.Add(45*time.Minute).Unix(),
		location,
	)

	require.NoError(t, err)
	assert.Equal(t, 1, stats.TotalActiveUsers)
	require.Len(t, stats.Daily, 1)
	assert.Equal(t, 1, stats.Daily[0].ActiveUsers)
}

func TestGetActiveUserStatsDoesNotDuplicateBucketsAcrossHalfHourDayBoundaries(t *testing.T) {
	db, _ := setupICodeStatsTest(t)
	location := time.FixedZone("UTC+5:30", 5*60*60+30*60)
	start := time.Date(2026, time.September, 1, 0, 0, 0, 0, location)
	end := time.Date(2026, time.September, 2, 23, 59, 59, 0, location)
	boundaryBucket := time.Date(2026, time.September, 1, 23, 30, 0, 0, location)
	require.NoError(t, db.Create(&QuotaData{
		UserID: 9, Username: "boundary-user", CreatedAt: boundaryBucket.Unix(), Count: 1,
	}).Error)

	stats, err := GetActiveUserStats(start.Unix(), end.Unix(), location)

	require.NoError(t, err)
	assert.Equal(t, []DailyActiveUserStat{
		{Date: "2026-09-01", ActiveUsers: 1},
		{Date: "2026-09-02", ActiveUsers: 0},
	}, stats.Daily)
}
