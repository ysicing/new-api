package model

import (
	"math"
	"strconv"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestListQuotaPoolRechargeRecordsNormalizesInvalidPagination(t *testing.T) {
	db := setupQuotaPoolFundsTestDB(t)
	pool := QuotaPool{Name: "分页池", PoolType: QuotaPoolTypeNormal, Enabled: true, BaseQuota: 1000, Quota: 1000}
	user := User{Username: "pagination-user", Password: "password", AffCode: "recharge-query-pagination", Role: common.RoleCommonUser, Status: common.UserStatusEnabled}
	require.NoError(t, db.Create(&pool).Error)
	require.NoError(t, db.Create(&user).Error)
	transactions := make([]QuotaPoolTransaction, 0, 25)
	for index := 1; index <= 25; index++ {
		transactions = append(transactions, QuotaPoolTransaction{
			PoolId: pool.Id, UserId: user.Id, Type: QuotaPoolTransactionAllocateAuto,
			Amount: -index, CreatedAt: int64(index),
		})
	}
	require.NoError(t, db.Create(&transactions).Error)
	page := &common.PageInfo{Page: math.MaxInt, PageSize: -1}

	items, total, err := ListQuotaPoolRechargeRecords(1, 25, page)

	require.NoError(t, err)
	assert.EqualValues(t, 25, total)
	assert.Equal(t, 3, page.Page)
	assert.Equal(t, common.ItemsPerPage, page.PageSize)
	assert.Len(t, items, 5)
}

func TestFindUserByRechargeIdentifierMatchesExactIdUsernameAndEmail(t *testing.T) {
	db := setupQuotaPoolFundsTestDB(t)
	user := User{Username: "exact-user", Email: "exact@example.com", Password: "password", AffCode: "recharge-query-exact", Role: common.RoleCommonUser, Status: common.UserStatusEnabled}
	require.NoError(t, db.Create(&user).Error)

	for _, identifier := range []string{strconv.Itoa(user.Id), user.Username, user.Email} {
		found, err := FindUserByRechargeIdentifier(identifier)
		require.NoError(t, err, identifier)
		assert.Equal(t, user.Id, found.Id, identifier)
	}
}

func TestFindUserByRechargeIdentifierUsesExactCaseAndHandlesAmbiguity(t *testing.T) {
	db := setupQuotaPoolFundsTestDB(t)
	users := []User{
		{Username: "CaseUser", Email: "shared@example.com", Password: "password", AffCode: "recharge-query-case", Role: common.RoleCommonUser, Status: common.UserStatusEnabled},
		{Username: "shared@example.com", Email: "other@example.com", Password: "password", AffCode: "recharge-query-ambiguous", Role: common.RoleCommonUser, Status: common.UserStatusEnabled},
	}
	require.NoError(t, db.Create(&users).Error)

	found, err := FindUserByRechargeIdentifier("CaseUser")
	require.NoError(t, err)
	assert.Equal(t, users[0].Id, found.Id)
	_, err = FindUserByRechargeIdentifier("CASEUSER")
	assert.ErrorIs(t, err, gorm.ErrRecordNotFound)

	_, err = FindUserByRechargeIdentifier("shared@example.com")
	assert.ErrorIs(t, err, ErrQuotaPoolRechargeUserAmbiguous)

	_, err = FindUserByRechargeIdentifier("missing@example.com")
	assert.ErrorIs(t, err, gorm.ErrRecordNotFound)
}

func TestListQuotaPoolRechargeRecordsFiltersRangeAndAllocationTypes(t *testing.T) {
	db := setupQuotaPoolFundsTestDB(t)
	pool := QuotaPool{Name: "研发额度池", PoolType: QuotaPoolTypeNormal, Enabled: true, BaseQuota: 1000, Quota: 1000}
	receiver := User{Username: "receiver", Email: "receiver@example.com", Password: "password", AffCode: "recharge-query-receiver", Role: common.RoleCommonUser, Status: common.UserStatusEnabled}
	operator := User{Username: "operator", Password: "password", AffCode: "recharge-query-operator", Role: common.RoleAdminUser, Status: common.UserStatusEnabled}
	require.NoError(t, db.Create(&pool).Error)
	require.NoError(t, db.Create(&receiver).Error)
	require.NoError(t, db.Create(&operator).Error)
	require.NoError(t, db.Create(&[]QuotaPoolTransaction{
		{PoolId: pool.Id, UserId: receiver.Id, Type: QuotaPoolTransactionAllocateAuto, Amount: -100, CreatedAt: 100},
		{PoolId: pool.Id, UserId: receiver.Id, OperatorId: operator.Id, Type: QuotaPoolTransactionAllocateManual, Amount: -200, CreatedAt: 200},
		{PoolId: pool.Id, UserId: receiver.Id, OperatorId: operator.Id, Type: QuotaPoolTransactionMonthlyRefill, Amount: 300, CreatedAt: 150},
		{PoolId: pool.Id, UserId: receiver.Id, Type: QuotaPoolTransactionAllocateAuto, Amount: -400, CreatedAt: 99},
	}).Error)

	items, total, err := ListQuotaPoolRechargeRecords(100, 200, &common.PageInfo{Page: 1, PageSize: 20})

	require.NoError(t, err)
	assert.EqualValues(t, 2, total)
	require.Len(t, items, 2)
	assert.Equal(t, QuotaPoolTransactionAllocateManual, items[0].Type)
	assert.Equal(t, 200, items[0].Amount)
	assert.Equal(t, "研发额度池", items[0].PoolName)
	assert.Equal(t, "receiver", items[0].UserName)
	assert.Equal(t, "receiver@example.com", items[0].UserEmail)
	assert.Equal(t, "operator", items[0].OperatorName)
	assert.Equal(t, QuotaPoolTransactionAllocateAuto, items[1].Type)
	assert.Equal(t, 100, items[1].Amount)
	assert.Empty(t, items[1].OperatorName)
}
