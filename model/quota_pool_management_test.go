package model

import (
	"strconv"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreateQuotaPoolWritesInitialFundAndRejectsDuplicateName(t *testing.T) {
	db := setupQuotaPoolFundsTestDB(t)

	pool, err := CreateQuotaPool(" 研发二组 ", 1000, 7)

	require.NoError(t, err)
	assert.Equal(t, "研发二组", pool.Name)
	assert.Equal(t, 1000, pool.BaseQuota)
	assert.Equal(t, 1000, pool.Quota)
	var transaction QuotaPoolTransaction
	require.NoError(t, db.Where("pool_id = ?", pool.Id).First(&transaction).Error)
	assert.Equal(t, QuotaPoolTransactionInitialFund, transaction.Type)
	assert.Equal(t, 1000, transaction.Amount)
	assert.Equal(t, 7, transaction.OperatorId)

	_, err = CreateQuotaPool("研发二组", 2000, 7)
	require.ErrorIs(t, err, ErrQuotaPoolNameExists)
}

func TestListQuotaPoolItemsIncludesSystemAutoRecharge(t *testing.T) {
	db := setupQuotaPoolFundsTestDB(t)
	require.NoError(t, db.Create(&QuotaPool{
		Name: "继承池", PoolType: QuotaPoolTypeNormal, Enabled: true,
	}).Error)
	config := operation_setting.GetAutoRechargeSetting()
	previous := *config
	config.Enabled = true
	config.Interval = 15
	config.Threshold = 30
	config.Amount = 120
	config.WeeklyLimit = 4
	config.MonthlyLimit = 12
	t.Cleanup(func() { *config = previous })

	items, err := ListQuotaPoolItems()

	require.NoError(t, err)
	require.Len(t, items, 1)
	assert.Equal(t, QuotaPoolSystemAutoRecharge{
		Enabled: true, Interval: 15,
		Threshold:   int(float64(30) * common.QuotaPerUnit),
		Amount:      int(float64(120) * common.QuotaPerUnit),
		WeeklyLimit: 4, MonthlyLimit: 12,
	}, items[0].SystemAutoRecharge)
}

func TestListQuotaPoolItemsPageSearchesAndPaginates(t *testing.T) {
	db := setupQuotaPoolFundsTestDB(t)
	pools := []QuotaPool{
		{Name: "保障默认池", PoolType: QuotaPoolTypeDefault, IsDefault: true, Enabled: true},
		{Name: "平台保障部", PoolType: QuotaPoolTypeNormal, Enabled: true},
		{Name: "team_%", PoolType: QuotaPoolTypeNormal, Enabled: true},
		{Name: "teamX", PoolType: QuotaPoolTypeNormal, Enabled: true},
	}
	require.NoError(t, db.Create(&pools).Error)

	first, total, err := ListQuotaPoolItemsPage("保障", &common.PageInfo{Page: 1, PageSize: 1})
	require.NoError(t, err)
	assert.EqualValues(t, 2, total)
	require.Len(t, first, 1)
	assert.Equal(t, pools[0].Id, first[0].Id)

	second, total, err := ListQuotaPoolItemsPage("保障", &common.PageInfo{Page: 2, PageSize: 1})
	require.NoError(t, err)
	assert.EqualValues(t, 2, total)
	require.Len(t, second, 1)
	assert.Equal(t, pools[1].Id, second[0].Id)

	byID, total, err := ListQuotaPoolItemsPage(strconv.Itoa(pools[3].Id), &common.PageInfo{Page: 1, PageSize: 20})
	require.NoError(t, err)
	assert.EqualValues(t, 1, total)
	require.Len(t, byID, 1)
	assert.Equal(t, pools[3].Id, byID[0].Id)

	literal, total, err := ListQuotaPoolItemsPage("team_", &common.PageInfo{Page: 1, PageSize: 20})
	require.NoError(t, err)
	assert.EqualValues(t, 1, total)
	require.Len(t, literal, 1)
	assert.Equal(t, pools[2].Id, literal[0].Id)
}

func TestAddQuotaPoolManualRefillEnforcesAmountAndMonthlyLimit(t *testing.T) {
	db := setupQuotaPoolFundsTestDB(t)
	pool, _ := seedQuotaPoolMember(t, db, 1000, 0)

	first, err := AddQuotaPoolManualRefill(pool.Id, 500, 8)
	require.NoError(t, err)
	assert.Equal(t, 1500, first.QuotaAfter)
	_, err = AddQuotaPoolManualRefill(pool.Id, 750, 8)
	require.NoError(t, err, "second refill can use the updated base quota")
	_, err = AddQuotaPoolManualRefill(pool.Id, 100, 8)
	require.ErrorIs(t, err, ErrQuotaPoolRefillLimited)

	var transactions int64
	require.NoError(t, db.Model(&QuotaPoolTransaction{}).
		Where("pool_id = ? AND type = ?", pool.Id, QuotaPoolTransactionManualRefill).
		Count(&transactions).Error)
	assert.EqualValues(t, 2, transactions)
}

func TestGrantQuotaPoolAdminSupportsV1AndV2ForPoolMembers(t *testing.T) {
	db := setupQuotaPoolFundsTestDB(t)
	pool, member := seedQuotaPoolMember(t, db, 1000, 0)
	outsider := User{Username: "outsider", Password: "password", AffCode: "outsider-code", Role: common.RoleCommonUser, Status: common.UserStatusEnabled}
	require.NoError(t, db.Create(&outsider).Error)

	require.NoError(t, GrantQuotaPoolAdmin(pool.Id, member.Id, QuotaPoolAdminLevelV2))
	summary, err := GetQuotaPoolAdminSummary(member.Id)
	require.NoError(t, err)
	require.NotNil(t, summary)
	assert.Equal(t, QuotaPoolAdminLevelV2, summary.Level)
	assert.Equal(t, pool.Id, summary.PoolId)

	err = GrantQuotaPoolAdmin(pool.Id, outsider.Id, QuotaPoolAdminLevelV1)
	require.ErrorIs(t, err, ErrQuotaPoolMemberMismatch)
}

func TestListQuotaPoolMembersSearchesCurrentPoolUserFields(t *testing.T) {
	db := setupQuotaPoolFundsTestDB(t)
	pool := QuotaPool{Name: "搜索池", PoolType: QuotaPoolTypeNormal, Enabled: true}
	otherPool := QuotaPool{Name: "其他池", PoolType: QuotaPoolTypeNormal, Enabled: true}
	require.NoError(t, db.Create(&pool).Error)
	require.NoError(t, db.Create(&otherPool).Error)
	target := User{
		Username: "alice", DisplayName: "Alice Chen", Email: "alice@example.com",
		Department: "研发一部", Password: "password", AffCode: "search-alice",
		Role: common.RoleCommonUser, Status: common.UserStatusEnabled, QuotaPoolId: pool.Id,
	}
	require.NoError(t, db.Create(&target).Error)
	require.NoError(t, db.Create(&User{
		Username: "outsider", Department: "研发一部", Password: "password", AffCode: "search-outsider",
		Role: common.RoleCommonUser, Status: common.UserStatusEnabled, QuotaPoolId: otherPool.Id,
	}).Error)

	for _, keyword := range []string{strconv.Itoa(target.Id), "alice", "alice chen", "alice@example.com", "研发一部"} {
		t.Run(keyword, func(t *testing.T) {
			items, total, err := ListQuotaPoolMembers(pool.Id, keyword, &common.PageInfo{Page: 1, PageSize: 10})

			require.NoError(t, err)
			assert.EqualValues(t, 1, total)
			require.Len(t, items, 1)
			assert.Equal(t, target.Id, items[0].Id)
		})
	}
	literalUnderscore := User{
		Username: "alice_dev", Password: "password", AffCode: "search-literal",
		Role: common.RoleCommonUser, Status: common.UserStatusEnabled, QuotaPoolId: pool.Id,
	}
	require.NoError(t, db.Create(&literalUnderscore).Error)
	require.NoError(t, db.Create(&User{
		Username: "alicexdev", Password: "password", AffCode: "search-wildcard",
		Role: common.RoleCommonUser, Status: common.UserStatusEnabled, QuotaPoolId: pool.Id,
	}).Error)

	items, total, err := ListQuotaPoolMembers(pool.Id, "alice_dev", &common.PageInfo{Page: 1, PageSize: 10})
	require.NoError(t, err)
	assert.EqualValues(t, 1, total)
	require.Len(t, items, 1)
	assert.Equal(t, literalUnderscore.Id, items[0].Id)
}

func TestListQuotaPoolMembersReturnsRequestedPageAndTotal(t *testing.T) {
	db := setupQuotaPoolFundsTestDB(t)
	pool := QuotaPool{Name: "分页池", PoolType: QuotaPoolTypeNormal, Enabled: true}
	require.NoError(t, db.Create(&pool).Error)
	users := []User{
		{Username: "member-a", Password: "password", AffCode: "page-a", Role: common.RoleCommonUser, Status: common.UserStatusEnabled, QuotaPoolId: pool.Id},
		{Username: "member-b", Password: "password", AffCode: "page-b", Role: common.RoleCommonUser, Status: common.UserStatusEnabled, QuotaPoolId: pool.Id},
		{Username: "member-c", Password: "password", AffCode: "page-c", Role: common.RoleCommonUser, Status: common.UserStatusEnabled, QuotaPoolId: pool.Id},
	}
	require.NoError(t, db.Create(&users).Error)

	firstPage, total, err := ListQuotaPoolMembers(pool.Id, "", &common.PageInfo{Page: 1, PageSize: 2})
	require.NoError(t, err)
	assert.EqualValues(t, 3, total)
	require.Len(t, firstPage, 2)
	assert.Equal(t, users[0].Id, firstPage[0].Id)

	secondPage, total, err := ListQuotaPoolMembers(pool.Id, "", &common.PageInfo{Page: 2, PageSize: 2})
	require.NoError(t, err)
	assert.EqualValues(t, 3, total)
	require.Len(t, secondPage, 1)
	assert.Equal(t, users[2].Id, secondPage[0].Id)
}

func TestListQuotaPoolCandidatesFiltersEligibilityAndSearchesByID(t *testing.T) {
	db := setupQuotaPoolFundsTestDB(t)
	newUserPool := QuotaPool{Name: "新用户池", PoolType: QuotaPoolTypeNewUser, Enabled: true}
	otherPool := QuotaPool{Name: "其他池", PoolType: QuotaPoolTypeNormal, Enabled: true}
	require.NoError(t, db.Create(&newUserPool).Error)
	require.NoError(t, db.Create(&otherPool).Error)
	users := []User{
		{Username: "eligible", Password: "password", AffCode: "candidate-eligible", Role: common.RoleCommonUser, Status: common.UserStatusEnabled},
		{Username: "disabled", Password: "password", AffCode: "candidate-disabled", Role: common.RoleCommonUser, Status: common.UserStatusDisabled},
		{Username: "root", Password: "password", AffCode: "candidate-root", Role: common.RoleRootUser, Status: common.UserStatusEnabled},
		{Username: "guest", Password: "password", AffCode: "candidate-guest", Role: common.RoleGuestUser, Status: common.UserStatusEnabled},
		{Username: "pool-super", Password: "password", AffCode: "candidate-super", Role: common.RoleQuotaPoolSuperAdmin, Status: common.UserStatusEnabled},
		{Username: "admin", Password: "password", AffCode: "candidate-admin", Role: common.RoleAdminUser, Status: common.UserStatusEnabled, QuotaPoolId: newUserPool.Id},
		{Username: "other-member", Password: "password", AffCode: "candidate-other", Role: common.RoleCommonUser, Status: common.UserStatusEnabled, QuotaPoolId: otherPool.Id},
	}
	require.NoError(t, db.Create(&users).Error)
	require.NoError(t, db.Model(&User{}).Where("id = ?", users[3].Id).Update("role", common.RoleGuestUser).Error)

	items, total, err := ListQuotaPoolCandidates("", &common.PageInfo{Page: 1, PageSize: 20})

	require.NoError(t, err)
	assert.EqualValues(t, 3, total)
	require.Len(t, items, 3)
	assert.ElementsMatch(t, []int{users[0].Id, users[4].Id, users[5].Id}, []int{items[0].Id, items[1].Id, items[2].Id})

	items, total, err = ListQuotaPoolCandidates(strconv.Itoa(users[0].Id), &common.PageInfo{Page: 1, PageSize: 20})
	require.NoError(t, err)
	assert.EqualValues(t, 1, total)
	require.Len(t, items, 1)
	assert.Equal(t, users[0].Id, items[0].Id)
}

func TestUpdateQuotaPoolConfigAdjustsAvailableQuotaWithBaseQuota(t *testing.T) {
	db := setupQuotaPoolFundsTestDB(t)
	pool, _ := seedQuotaPoolMember(t, db, 1000, 0)
	require.NoError(t, db.Model(&pool).Update("quota", 400).Error)

	change, err := UpdateQuotaPoolConfig(pool.Id, map[string]any{"base_quota": 1200, "weekly_limit": 3}, 7)

	require.NoError(t, err)
	require.NotNil(t, change)
	assert.Equal(t, 600, change.QuotaAfter)
	require.NoError(t, db.First(&pool, pool.Id).Error)
	assert.Equal(t, 1200, pool.BaseQuota)
	assert.Equal(t, 600, pool.Quota)
	assert.Equal(t, 3, pool.WeeklyLimit)
}

func TestUpdateQuotaPoolConfigValidatesSpecialValuesAndMonthlyPolicy(t *testing.T) {
	invalidUpdates := []map[string]any{
		{"weekly_limit": -2},
		{"monthly_limit": -2},
		{"monthly_refill_amount": -1},
		{"monthly_refill_enabled": true},
	}
	for index, updates := range invalidUpdates {
		t.Run(strconv.Itoa(index), func(t *testing.T) {
			db := setupQuotaPoolFundsTestDB(t)
			pool := QuotaPool{Name: "校验池", PoolType: QuotaPoolTypeNormal, Enabled: true}
			require.NoError(t, db.Create(&pool).Error)

			_, err := UpdateQuotaPoolConfig(pool.Id, updates, 7)

			assert.ErrorIs(t, err, ErrQuotaPoolInvalidAmount)
		})
	}
	t.Run("legacy invalid day", func(t *testing.T) {
		db := setupQuotaPoolFundsTestDB(t)
		pool := QuotaPool{
			Name: "旧数据池", PoolType: QuotaPoolTypeNormal, Enabled: true,
			MonthlyRefillAmount: 500, MonthlyRefillDay: 29,
		}
		require.NoError(t, db.Create(&pool).Error)

		_, err := UpdateQuotaPoolConfig(pool.Id, map[string]any{"monthly_refill_enabled": true}, 7)

		assert.ErrorIs(t, err, ErrQuotaPoolInvalidAmount)
	})

	db := setupQuotaPoolFundsTestDB(t)
	pool := QuotaPool{Name: "有效池", PoolType: QuotaPoolTypeNormal, Enabled: true}
	require.NoError(t, db.Create(&pool).Error)
	_, err := UpdateQuotaPoolConfig(pool.Id, map[string]any{
		"monthly_refill_enabled": true,
		"monthly_refill_top_up":  true,
		"monthly_refill_amount":  500,
		"monthly_refill_day":     15,
	}, 7)
	require.NoError(t, err)
}

func TestDeleteQuotaPoolRejectsSystemPoolsAndPoolsWithMembers(t *testing.T) {
	db := setupQuotaPoolFundsTestDB(t)
	pool, user := seedQuotaPoolMember(t, db, 1000, 0)
	systemPool := QuotaPool{Name: QuotaPoolNewUserName, PoolType: QuotaPoolTypeNewUser, Enabled: true, BaseQuota: -1, Quota: -1}
	require.NoError(t, db.Create(&systemPool).Error)

	require.ErrorIs(t, DeleteQuotaPool(systemPool.Id), ErrQuotaPoolSystemReadonly)
	require.ErrorIs(t, DeleteQuotaPool(pool.Id), ErrQuotaPoolHasMembers)
	require.NoError(t, db.Model(&user).Update("quota_pool_id", 0).Error)
	require.NoError(t, DeleteQuotaPool(pool.Id))
}
