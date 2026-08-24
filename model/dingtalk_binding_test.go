package model

import (
	"fmt"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupDingTalkBindingTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:dingtalk-binding-%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&User{}, &ExternalIdentityClaim{}))
	previous := DB
	DB = db
	t.Cleanup(func() { DB = previous })
	return db
}

func newDingTalkBindingUser(name string) User {
	return User{
		Username: name, Password: "password", AffCode: name + "-aff",
		Role: common.RoleCommonUser, Status: common.UserStatusEnabled,
	}
}

func TestBindDingTalkIdentityPersistsAndIsIdempotent(t *testing.T) {
	db := setupDingTalkBindingTestDB(t)
	user := newDingTalkBindingUser("bind-user")
	require.NoError(t, db.Create(&user).Error)

	boundNow, err := BindDingTalkIdentity(user.Id, "union-1")
	require.NoError(t, err)
	assert.True(t, boundNow)
	boundNow, err = BindDingTalkIdentity(user.Id, "union-1")
	require.NoError(t, err)
	assert.False(t, boundNow)
	require.NoError(t, db.First(&user, user.Id).Error)
	assert.Equal(t, "union-1", user.DingTalkId)
	var claim ExternalIdentityClaim
	require.NoError(t, db.Where("provider = ? AND subject = ?", "dingtalk", "union-1").First(&claim).Error)
	assert.Equal(t, user.Id, claim.UserId)
}

func TestClearDingTalkBindingReleasesIdentityClaim(t *testing.T) {
	db := setupDingTalkBindingTestDB(t)
	user := newDingTalkBindingUser("clear-user")
	require.NoError(t, db.Create(&user).Error)
	_, err := BindDingTalkIdentity(user.Id, "union-clear")
	require.NoError(t, err)

	require.NoError(t, user.ClearBinding("dingtalk"))

	var count int64
	require.NoError(t, db.Model(&ExternalIdentityClaim{}).
		Where("provider = ? AND user_id = ?", "dingtalk", user.Id).Count(&count).Error)
	assert.Zero(t, count)
}

func TestBindDingTalkIdentityRejectsReplacement(t *testing.T) {
	db := setupDingTalkBindingTestDB(t)
	user := newDingTalkBindingUser("replace-user")
	user.DingTalkId = "union-old"
	require.NoError(t, db.Create(&user).Error)

	_, err := BindDingTalkIdentity(user.Id, "union-new")

	assert.ErrorIs(t, err, ErrDingTalkIdentityConflict)
}

func TestBindDingTalkIdentityRejectsActiveAndDeletedOwners(t *testing.T) {
	for _, deleted := range []bool{false, true} {
		t.Run(fmt.Sprintf("deleted=%t", deleted), func(t *testing.T) {
			db := setupDingTalkBindingTestDB(t)
			owner := newDingTalkBindingUser("identity-owner")
			owner.DingTalkId = "union-owned"
			target := newDingTalkBindingUser("identity-target")
			require.NoError(t, db.Create(&owner).Error)
			require.NoError(t, db.Create(&target).Error)
			if deleted {
				require.NoError(t, db.Delete(&owner).Error)
			}

			_, err := BindDingTalkIdentity(target.Id, "union-owned")

			assert.ErrorIs(t, err, ErrDingTalkIdentityConflict)
		})
	}
}
