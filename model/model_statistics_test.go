package model

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetModelStatisticsAggregatesCostAndShare(t *testing.T) {
	mainDB, _ := setupICodeStatsTest(t)
	require.NoError(t, mainDB.Create(&[]QuotaData{
		{UserID: 1, ModelName: "gpt-5", CreatedAt: 100, Count: 2, Quota: 40},
		{UserID: 2, ModelName: "gpt-5", CreatedAt: 110, Count: 1, Quota: 30},
		{UserID: 1, ModelName: "claude-4", CreatedAt: 120, Count: 3, Quota: 30},
		{UserID: 1, ModelName: "ignored", CreatedAt: 300, Count: 9, Quota: 900},
	}).Error)

	items, err := GetModelStatistics(90, 200, 0)

	require.NoError(t, err)
	require.Len(t, items, 2)
	assert.Equal(t, ModelUsageStat{ModelName: "gpt-5", Count: 3, Quota: 70, Share: 0.7}, items[0])
	assert.Equal(t, ModelUsageStat{ModelName: "claude-4", Count: 3, Quota: 30, Share: 0.3}, items[1])
}

func TestGetModelStatisticsRestrictsToUser(t *testing.T) {
	mainDB, _ := setupICodeStatsTest(t)
	require.NoError(t, mainDB.Create(&[]QuotaData{
		{UserID: 1, ModelName: "gpt-5", CreatedAt: 100, Count: 2, Quota: 40},
		{UserID: 2, ModelName: "claude-4", CreatedAt: 100, Count: 3, Quota: 60},
	}).Error)

	items, err := GetModelStatistics(90, 200, 1)

	require.NoError(t, err)
	require.Len(t, items, 1)
	assert.Equal(t, "gpt-5", items[0].ModelName)
	assert.Equal(t, float64(1), items[0].Share)
}
