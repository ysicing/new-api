package model

import (
	"fmt"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func insertUsersForPaginationTest(t *testing.T, total int) {
	t.Helper()
	for id := 1; id <= total; id++ {
		user := &User{
			Id:          id,
			Username:    fmt.Sprintf("user%02d", id),
			Password:    "password123",
			DisplayName: fmt.Sprintf("User %02d", id),
			Email:       fmt.Sprintf("user%02d@example.com", id),
			Role:        common.RoleCommonUser,
			Status:      common.UserStatusEnabled,
			Group:       "default",
			AffCode:     fmt.Sprintf("aff%02d", id),
		}
		require.NoError(t, DB.Create(user).Error)
	}
}

func collectUserIDs(users []*User) []int {
	ids := make([]int, 0, len(users))
	for _, user := range users {
		ids = append(ids, user.Id)
	}
	return ids
}

func TestGetAllUsersSortsBeforePagination(t *testing.T) {
	truncateTables(t)
	insertUsersForPaginationTest(t, 42)

	pageOne, total, err := GetAllUsers(&common.PageInfo{Page: 1, PageSize: 20}, NewUserSortOptions("id", "asc"))
	require.NoError(t, err)
	assert.Equal(t, int64(42), total)
	assert.Equal(t, []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20}, collectUserIDs(pageOne))

	pageTwo, total, err := GetAllUsers(&common.PageInfo{Page: 2, PageSize: 20}, NewUserSortOptions("id", "asc"))
	require.NoError(t, err)
	assert.Equal(t, int64(42), total)
	assert.Equal(t, []int{21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 31, 32, 33, 34, 35, 36, 37, 38, 39, 40}, collectUserIDs(pageTwo))

	pageThree, total, err := GetAllUsers(&common.PageInfo{Page: 3, PageSize: 20}, NewUserSortOptions("id", "asc"))
	require.NoError(t, err)
	assert.Equal(t, int64(42), total)
	assert.Equal(t, []int{41, 42}, collectUserIDs(pageThree))
}

func TestSearchUsersSortsBeforePagination(t *testing.T) {
	truncateTables(t)
	insertUsersForPaginationTest(t, 42)

	users, total, err := SearchUsers("user", "", nil, nil, nil, 20, 20, NewUserSortOptions("id", "asc"))
	require.NoError(t, err)
	assert.Equal(t, int64(42), total)
	assert.Equal(t, []int{21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 31, 32, 33, 34, 35, 36, 37, 38, 39, 40}, collectUserIDs(users))
}

func TestSearchUsersFiltersByQuotaPool(t *testing.T) {
	truncateTables(t)
	insertUsersForPaginationTest(t, 4)
	require.NoError(t, DB.Model(&User{}).Where("id IN ?", []int{2, 4}).Update("quota_pool_id", 9).Error)
	poolId := 9

	users, total, err := SearchUsers("user", "", nil, nil, &poolId, 0, 20, NewUserSortOptions("id", "asc"))

	require.NoError(t, err)
	assert.EqualValues(t, 2, total)
	assert.Equal(t, []int{2, 4}, collectUserIDs(users))
}

func TestDefaultPoolUsersUseTheSystemPoolRecordName(t *testing.T) {
	truncateTables(t)
	require.NoError(t, DB.AutoMigrate(&QuotaPool{}))
	require.NoError(t, DB.Exec("DELETE FROM quota_pools").Error)
	t.Cleanup(func() {
		require.NoError(t, DB.Exec("DELETE FROM quota_pools").Error)
	})

	require.NoError(t, DB.Create(&QuotaPool{
		Name: QuotaPoolDefaultName, PoolType: QuotaPoolTypeDefault,
		Enabled: true, IsDefault: true, BaseQuota: -1, Quota: -1,
	}).Error)
	user := &User{
		Username: "default-pool-user", Password: "password",
		AffCode: "default-pool-user-aff", Role: common.RoleCommonUser,
		Status: common.UserStatusEnabled, QuotaPoolId: QuotaPoolDefaultUserPoolId,
	}
	require.NoError(t, DB.Create(user).Error)

	users, total, err := GetAllUsers(
		&common.PageInfo{Page: 1, PageSize: 20},
		NewUserSortOptions("id", "asc"),
	)
	require.NoError(t, err)
	assert.EqualValues(t, 1, total)
	require.Len(t, users, 1)
	assert.Equal(t, QuotaPoolDefaultName, users[0].QuotaPoolName)

	detail, err := GetUserById(user.Id, false)
	require.NoError(t, err)
	assert.Equal(t, QuotaPoolDefaultName, detail.QuotaPoolName)
}
