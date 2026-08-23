package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGrantQuotaPoolAdminUsesUnifiedLevelForPoolMembers(t *testing.T) {
	db := setupQuotaPoolFundsTestDB(t)
	pool, member := seedQuotaPoolMember(t, db, 1000, 0)
	outsider := User{Username: "outsider", Password: "password", AffCode: "outsider-code", Role: common.RoleCommonUser, Status: common.UserStatusEnabled}
	require.NoError(t, db.Create(&outsider).Error)

	require.NoError(t, GrantQuotaPoolAdmin(pool.Id, member.Id))
	summary, err := GetQuotaPoolAdminSummary(member.Id)
	require.NoError(t, err)
	require.NotNil(t, summary)
	assert.Equal(t, pool.Id, summary.PoolId)
	var admin QuotaPoolAdmin
	require.NoError(t, db.Where("user_id = ?", member.Id).First(&admin).Error)
	assert.Equal(t, QuotaPoolAdminLevel, admin.Level)

	err = GrantQuotaPoolAdmin(pool.Id, outsider.Id)
	require.ErrorIs(t, err, ErrQuotaPoolMemberMismatch)
}
