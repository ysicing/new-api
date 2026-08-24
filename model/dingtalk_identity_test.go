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

func TestDingTalkIdentityPersistsAndFindsUser(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:dingtalk-identity-%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&User{}))
	previousDB := DB
	DB = db
	t.Cleanup(func() { DB = previousDB })

	user := User{
		Username: "dingtalk-user", Password: "password", AffCode: "dingtalk-aff",
		Role: common.RoleCommonUser, Status: common.UserStatusEnabled, DingTalkId: "union-1",
	}
	require.NoError(t, db.Create(&user).Error)
	assert.True(t, IsDingTalkIdAlreadyTaken("union-1"))

	found := User{DingTalkId: "union-1"}
	require.NoError(t, found.FillUserByDingTalkId())
	assert.Equal(t, user.Id, found.Id)
}
