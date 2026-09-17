package model

import (
	"fmt"
	"strconv"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestQuotaPoolTransactionFiltersApplyBeforePagination(t *testing.T) {
	db := setupQuotaPoolFundsTestDB(t)
	member := User{Username: "alice_%", DisplayName: "产品成员", AffCode: "history-member"}
	operator := User{Username: "manager", DisplayName: "管理员", AffCode: "history-operator"}
	require.NoError(t, db.Create(&member).Error)
	require.NoError(t, db.Create(&operator).Error)
	records := []QuotaPoolTransaction{
		{PoolId: 7, Type: QuotaPoolTransactionAllocateManual, UserId: member.Id, OperatorId: operator.Id, Amount: -50},
		{PoolId: 7, Type: QuotaPoolTransactionAllocateAuto, UserId: member.Id, Amount: -20},
		{PoolId: 8, Type: QuotaPoolTransactionAllocateManual, UserId: member.Id, OperatorId: operator.Id, Amount: -100},
		{PoolId: 7, Type: QuotaPoolTransactionAllocateManual, UserId: 999, Amount: -10},
	}
	require.NoError(t, db.Create(&records).Error)
	for _, keyword := range []string{"ALICE", "产品", "%", "_", "manager", "管理员", strconv.Itoa(operator.Id)} {
		t.Run(keyword, func(t *testing.T) {
			items, total, err := ListQuotaPoolTransactions(7, &common.PageInfo{Page: 1, PageSize: 1}, QuotaPoolTransactionAllocateManual, keyword)
			require.NoError(t, err)
			require.Len(t, items, 1)
			assert.EqualValues(t, 1, total)
			assert.Equal(t, records[0].Id, items[0].Id)
		})
	}
	items, total, err := ListQuotaPoolTransactions(7, &common.PageInfo{Page: 2, PageSize: 1}, "", "alice")
	require.NoError(t, err)
	require.Len(t, items, 1)
	assert.EqualValues(t, 2, total)
	assert.Equal(t, records[0].Id, items[0].Id)
	items, total, err = ListQuotaPoolTransactions(7, &common.PageInfo{Page: 1, PageSize: 10}, "", "999")
	require.NoError(t, err)
	require.Len(t, items, 1)
	assert.EqualValues(t, 1, total)
	assert.Equal(t, records[3].Id, items[0].Id)
	items, total, err = ListQuotaPoolTransactions(7, &common.PageInfo{Page: 1, PageSize: 10}, "", "not-found")
	require.NoError(t, err)
	assert.Empty(t, items)
	assert.Zero(t, total)
}

func TestQuotaPoolOperationFilterUsesExactActionAndPoolInLogDatabase(t *testing.T) {
	setupQuotaPoolFundsTestDB(t)
	logDB, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:quota-history-log-%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, logDB.AutoMigrate(&Log{}))
	previous := LOG_DB
	LOG_DB = logDB
	t.Cleanup(func() { LOG_DB = previous })
	var records []Log
	for _, item := range []struct {
		pool   int
		action string
	}{
		{7, "quota_pool.member_add"}, {7, "quota_pool.member_add"}, {8, "quota_pool.member_add"}, {7, "quota_pool.member_add_suffix"}, {7, "quota_pool.member_remove"},
	} {
		other, err := common.Marshal(map[string]any{"quota_pool_id": item.pool, "op": buildOpField(item.action, map[string]any{"quota_pool_id": item.pool})})
		require.NoError(t, err)
		records = append(records, Log{Type: LogTypeManage, Other: string(other)})
	}
	records = append(records, Log{Type: LogTypeManage, Content: "legacy", Other: `{"quota_pool_id":7}`})
	require.NoError(t, logDB.Create(&records).Error)
	items, total, err := ListQuotaPoolOperationLogs(7, &common.PageInfo{Page: 2, PageSize: 1}, "quota_pool.member_add")
	require.NoError(t, err)
	require.Len(t, items, 1)
	assert.EqualValues(t, 2, total)
	assert.Equal(t, records[0].Id, items[0].Id)
	items, total, err = ListQuotaPoolOperationLogs(7, &common.PageInfo{Page: 1, PageSize: 10}, "quota_pool.member_%")
	require.NoError(t, err)
	assert.Zero(t, total)
	assert.Empty(t, items)
	items, total, err = ListQuotaPoolOperationLogs(7, &common.PageInfo{Page: 1, PageSize: 10}, "")
	require.NoError(t, err)
	assert.EqualValues(t, 5, total)
	assert.Len(t, items, 5)
}
