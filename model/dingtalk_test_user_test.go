package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestListDingTalkTestUsersSearchesEligibleUsers(t *testing.T) {
	db := setupDingTalkBindingTestDB(t)
	users := []User{
		{Username: "alice", DisplayName: "Alice", Department: "研发部", Email: "alice@example.com", Password: "password", AffCode: "test-user-alice", Role: common.RoleCommonUser, Status: common.UserStatusEnabled, DingTalkId: "union-alice"},
		{Username: "bob", DisplayName: "Bob", Department: "平台部", Email: "bob@example.com", Password: "password", AffCode: "test-user-bob", Role: common.RoleAdminUser, Status: common.UserStatusEnabled},
		{Username: "disabled", Email: "disabled@example.com", Password: "password", AffCode: "test-user-disabled", Role: common.RoleCommonUser, Status: common.UserStatusDisabled},
		{Username: "root", Email: "root@example.com", Password: "password", AffCode: "test-user-root", Role: common.RoleRootUser, Status: common.UserStatusEnabled},
		{Username: "invalid", Email: "invalid", Password: "password", AffCode: "test-user-invalid", Role: common.RoleCommonUser, Status: common.UserStatusEnabled},
	}
	require.NoError(t, db.Create(&users).Error)

	items, total, err := ListDingTalkTestUsers("研发", 0, 20)

	require.NoError(t, err)
	assert.EqualValues(t, 1, total)
	require.Len(t, items, 1)
	assert.Equal(t, users[0].Id, items[0].Id)
	assert.True(t, items[0].DingTalkBound)

	items, total, err = ListDingTalkTestUsers("", 0, 20)
	require.NoError(t, err)
	assert.EqualValues(t, 2, total)
	assert.ElementsMatch(t, []int{users[0].Id, users[1].Id}, []int{items[0].Id, items[1].Id})
}
