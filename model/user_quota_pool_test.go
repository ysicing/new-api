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

func TestInsertAssignsNewUserQuotaPoolWhenFeatureEnabled(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:new-user-pool-%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&User{}, &Log{}, &QuotaPool{}, &QuotaPoolAdmin{}, &QuotaPoolTransaction{}))
	previousDB, previousLogDB := DB, LOG_DB
	previousEnabled, previousQuota := common.QuotaPoolEnabled, common.QuotaForNewUser
	DB, LOG_DB = db, db
	common.QuotaPoolEnabled, common.QuotaForNewUser = true, 500
	t.Cleanup(func() {
		DB, LOG_DB = previousDB, previousLogDB
		common.QuotaPoolEnabled, common.QuotaForNewUser = previousEnabled, previousQuota
	})
	require.NoError(t, syncSystemQuotaPools(db))
	var newUserPool QuotaPool
	require.NoError(t, db.Where("pool_type = ?", QuotaPoolTypeNewUser).First(&newUserPool).Error)

	user := User{Username: "new-pool-user", Password: "password", Role: common.RoleCommonUser, Status: common.UserStatusEnabled}
	require.NoError(t, user.Insert(0))

	assert.Equal(t, newUserPool.Id, user.QuotaPoolId)
	assert.Equal(t, 500, user.Quota)
}
