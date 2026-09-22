package model

import (
	"fmt"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupQuotaPoolBudgetTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:quota-pool-budget-%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&QuotaPool{}, &QuotaPoolTransaction{}, &QuotaPoolBudgetTag{}))
	previousDB := DB
	DB = db
	t.Cleanup(func() { DB = previousDB })
	return db
}

func TestQuotaPoolBudgetTagNormalizesNamesAndProtectsAssignedTags(t *testing.T) {
	db := setupQuotaPoolBudgetTestDB(t)
	tag, err := CreateQuotaPoolBudgetTag("  产研中心  ")
	require.NoError(t, err)
	assert.Equal(t, "产研中心", tag.Name)

	_, err = CreateQuotaPoolBudgetTag("产研中心")
	assert.ErrorIs(t, err, ErrQuotaPoolBudgetTagNameExists)
	updated, previousName, err := UpdateQuotaPoolBudgetTag(tag.Id, "产研与产品中心")
	require.NoError(t, err)
	assert.Equal(t, "产研中心", previousName)
	assert.Equal(t, "产研与产品中心", updated.Name)
	unchanged, previousName, err := UpdateQuotaPoolBudgetTag(tag.Id, "产研与产品中心")
	require.NoError(t, err)
	assert.Equal(t, "产研与产品中心", previousName)
	assert.Equal(t, "产研与产品中心", unchanged.Name)
	other, err := CreateQuotaPoolBudgetTag("市场中心")
	require.NoError(t, err)
	_, _, err = UpdateQuotaPoolBudgetTag(other.Id, "产研与产品中心")
	assert.ErrorIs(t, err, ErrQuotaPoolBudgetTagNameExists)

	pool := QuotaPool{Name: "研发一部", PoolType: QuotaPoolTypeNormal, Enabled: true}
	require.NoError(t, db.Create(&pool).Error)
	_, err = SetQuotaPoolBudgetTag(pool.Id, tag.Id)
	require.NoError(t, err)
	_, err = DeleteQuotaPoolBudgetTag(tag.Id)
	assert.ErrorIs(t, err, ErrQuotaPoolBudgetTagInUse)

	_, err = SetQuotaPoolBudgetTag(pool.Id, 0)
	require.NoError(t, err)
	_, err = DeleteQuotaPoolBudgetTag(tag.Id)
	require.NoError(t, err)
}

func TestReplaceQuotaPoolBudgetTagPoolsAtomicallyReassignsCurrentPools(t *testing.T) {
	db := setupQuotaPoolBudgetTestDB(t)
	first, err := CreateQuotaPoolBudgetTag("产研中心")
	require.NoError(t, err)
	second, err := CreateQuotaPoolBudgetTag("市场中心")
	require.NoError(t, err)
	pools := []QuotaPool{
		{Name: "研发一部", PoolType: QuotaPoolTypeNormal, Enabled: true, BudgetTagId: first.Id},
		{Name: "研发二部", PoolType: QuotaPoolTypeNormal, Enabled: true, BudgetTagId: first.Id},
		{Name: "共享池", PoolType: QuotaPoolTypeNormal, Enabled: true, BudgetTagId: second.Id},
	}
	require.NoError(t, db.Create(&pools).Error)

	change, err := ReplaceQuotaPoolBudgetTagPools(first.Id, []int{pools[1].Id, pools[2].Id})
	require.NoError(t, err)
	assert.Equal(t, map[int]int{pools[0].Id: first.Id, pools[1].Id: first.Id, pools[2].Id: second.Id}, change.Before)
	assert.Equal(t, map[int]int{pools[0].Id: 0, pools[1].Id: first.Id, pools[2].Id: first.Id}, change.After)

	var reloaded []QuotaPool
	require.NoError(t, db.Order("id ASC").Find(&reloaded).Error)
	assert.Zero(t, reloaded[0].BudgetTagId)
	assert.Equal(t, first.Id, reloaded[1].BudgetTagId)
	assert.Equal(t, first.Id, reloaded[2].BudgetTagId)
}

func TestGetQuotaPoolBudgetStatsAggregatesNaturalMonthsAndCurrentTags(t *testing.T) {
	db := setupQuotaPoolBudgetTestDB(t)
	research, err := CreateQuotaPoolBudgetTag("产研中心")
	require.NoError(t, err)
	marketing, err := CreateQuotaPoolBudgetTag("市场中心")
	require.NoError(t, err)
	pools := []QuotaPool{
		{Name: "研发一部", PoolType: QuotaPoolTypeNormal, Enabled: true, BudgetTagId: research.Id},
		{Name: "研发二部", PoolType: QuotaPoolTypeNormal, Enabled: true, BudgetTagId: research.Id},
		{Name: "市场部", PoolType: QuotaPoolTypeNormal, Enabled: true, BudgetTagId: marketing.Id},
		{Name: "未标记", PoolType: QuotaPoolTypeNormal, Enabled: true},
	}
	require.NoError(t, db.Create(&pools).Error)
	beijing := common.BeijingTimeLocation
	at := func(year int, month time.Month, day int) int64 {
		return time.Date(year, month, day, 12, 0, 0, 0, beijing).Unix()
	}
	transactions := []QuotaPoolTransaction{
		{PoolId: pools[0].Id, Type: QuotaPoolTransactionInitialFund, Amount: 1_000, CreatedAt: at(2026, time.January, 1)},
		{PoolId: pools[0].Id, Type: QuotaPoolTransactionAdjustBase, Amount: -100, CreatedAt: at(2026, time.January, 2)},
		{PoolId: pools[0].Id, Type: QuotaPoolTransactionManualDeduct, Amount: -150, CreatedAt: at(2026, time.January, 2)},
		{PoolId: pools[0].Id, Type: QuotaPoolTransactionAllocateAuto, Amount: -300, CreatedAt: at(2026, time.January, 3)},
		{PoolId: pools[0].Id, Type: QuotaPoolTransactionReclaimUser, Amount: 50, CreatedAt: at(2026, time.January, 4)},
		{PoolId: pools[1].Id, Type: QuotaPoolTransactionManualRefill, Amount: 500, CreatedAt: at(2026, time.January, 5)},
		{PoolId: pools[1].Id, Type: QuotaPoolTransactionAllocateManual, Amount: -100, CreatedAt: at(2026, time.January, 6)},
		{PoolId: pools[0].Id, Type: QuotaPoolTransactionMonthlyRefill, Amount: 200, CreatedAt: at(2026, time.February, 1)},
		{PoolId: pools[0].Id, Type: QuotaPoolTransactionAllocateAuto, Amount: -80, CreatedAt: at(2026, time.February, 2)},
		{PoolId: pools[2].Id, Type: QuotaPoolTransactionInitialFund, Amount: 700, CreatedAt: at(2026, time.January, 7)},
		{PoolId: pools[2].Id, Type: QuotaPoolTransactionAllocateManual, Amount: -70, CreatedAt: at(2026, time.January, 8)},
		{PoolId: pools[3].Id, Type: QuotaPoolTransactionInitialFund, Amount: 9_999, CreatedAt: at(2026, time.January, 9)},
		{PoolId: pools[0].Id, Type: QuotaPoolTransactionInitialFund, Amount: 8_888, CreatedAt: at(2025, time.December, 31)},
		{PoolId: pools[0].Id, Type: QuotaPoolTransactionAllocateAuto, Amount: 999, CreatedAt: at(2026, time.January, 10)},
		{PoolId: pools[0].Id, Type: QuotaPoolTransactionReclaimUser, Amount: -999, CreatedAt: at(2026, time.January, 11)},
	}
	require.NoError(t, db.Create(&transactions).Error)

	stats, err := GetQuotaPoolBudgetStats("2026-01", "2026-02", beijing)
	require.NoError(t, err)
	assert.Equal(t, int64(2_150), stats.Summary.NetRecharge)
	assert.Equal(t, int64(500), stats.Summary.NetConsumption)
	require.Len(t, stats.Months, 2)
	assert.Equal(t, QuotaPoolBudgetMonthStat{Month: "2026-01", QuotaPoolBudgetAmounts: QuotaPoolBudgetAmounts{NetRecharge: 1_950, NetConsumption: 420}}, stats.Months[0])
	assert.Equal(t, QuotaPoolBudgetMonthStat{Month: "2026-02", QuotaPoolBudgetAmounts: QuotaPoolBudgetAmounts{NetRecharge: 200, NetConsumption: 80}}, stats.Months[1])
	require.Len(t, stats.Tags, 2)
	assert.Equal(t, "产研中心", stats.Tags[0].Name)
	assert.Equal(t, int64(1_450), stats.Tags[0].NetRecharge)
	assert.Equal(t, int64(430), stats.Tags[0].NetConsumption)
	require.Len(t, stats.Tags[0].Pools, 2)

	_, err = SetQuotaPoolBudgetTag(pools[1].Id, marketing.Id)
	require.NoError(t, err)
	reassigned, err := GetQuotaPoolBudgetStats("2026-01", "2026-02", beijing)
	require.NoError(t, err)
	assert.Equal(t, int64(950), reassigned.Tags[0].NetRecharge)
	assert.Equal(t, int64(330), reassigned.Tags[0].NetConsumption)
	assert.Equal(t, int64(1_200), reassigned.Tags[1].NetRecharge)
	assert.Equal(t, int64(170), reassigned.Tags[1].NetConsumption)
}

func TestGetQuotaPoolBudgetStatsRejectsInvalidOrOversizedMonthRanges(t *testing.T) {
	setupQuotaPoolBudgetTestDB(t)

	_, err := GetQuotaPoolBudgetStats("2026-02", "2026-01", common.BeijingTimeLocation)
	assert.ErrorIs(t, err, ErrQuotaPoolBudgetMonthRangeInvalid)
	_, err = GetQuotaPoolBudgetStats("2024-01", "2026-01", common.BeijingTimeLocation)
	assert.ErrorIs(t, err, ErrQuotaPoolBudgetMonthRangeInvalid)
	_, err = GetQuotaPoolBudgetStats("not-a-month", "2026-01", common.BeijingTimeLocation)
	assert.ErrorIs(t, err, ErrQuotaPoolBudgetMonthRangeInvalid)
}

func TestGetQuotaPoolBudgetStatsRejectsAmountsOutsideJavaScriptSafeIntegerRange(t *testing.T) {
	db := setupQuotaPoolBudgetTestDB(t)
	tag, err := CreateQuotaPoolBudgetTag("大额预算")
	require.NoError(t, err)
	pool := QuotaPool{Name: "大额池", PoolType: QuotaPoolTypeNormal, Enabled: true, BudgetTagId: tag.Id}
	require.NoError(t, db.Create(&pool).Error)
	require.NoError(t, db.Create(&QuotaPoolTransaction{
		PoolId: pool.Id, Type: QuotaPoolTransactionInitialFund,
		Amount: 9_007_199_254_740_992, CreatedAt: time.Date(2026, time.January, 1, 0, 0, 0, 0, common.BeijingTimeLocation).Unix(),
	}).Error)

	_, err = GetQuotaPoolBudgetStats("2026-01", "2026-01", common.BeijingTimeLocation)
	assert.ErrorIs(t, err, ErrQuotaPoolBudgetAmountOverflow)
}

func TestGetQuotaPoolBudgetStatsReturnsEmptyArraysForUnassignedTag(t *testing.T) {
	setupQuotaPoolBudgetTestDB(t)
	_, err := CreateQuotaPoolBudgetTag("暂无额度池")
	require.NoError(t, err)

	stats, err := GetQuotaPoolBudgetStats("2026-01", "2026-01", common.BeijingTimeLocation)
	require.NoError(t, err)
	require.Len(t, stats.Tags, 1)
	assert.NotNil(t, stats.Tags[0].Pools)
	assert.Empty(t, stats.Tags[0].Pools)
}
