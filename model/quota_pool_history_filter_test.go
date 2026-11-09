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
	require.NoError(t, logDB.AutoMigrate(&Log{}, &AuditLog{}))
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
		other, err := common.Marshal(map[string]any{"quota_pool_id": item.pool, "op": &AuditOperation{Action: item.action, Params: AuditFields{"quota_pool_id": item.pool}}})
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

func TestLegacyPoolOperationLogsIncludeOldZeroIdentifier(t *testing.T) {
	db := setupQuotaPoolFundsTestDB(t)
	require.NoError(t, db.AutoMigrate(&Log{}, &AuditLog{}))
	previous := LOG_DB
	LOG_DB = db
	t.Cleanup(func() { LOG_DB = previous })
	pool := QuotaPool{Name: QuotaPoolDefaultName, PoolType: QuotaPoolTypeNormal, LegacyDefault: true, Enabled: true}
	require.NoError(t, db.Create(&pool).Error)
	require.NoError(t, db.Create(&[]Log{
		{Type: LogTypeTopup, Other: `{"quota_pool_id":0,"recharge_source":"auto"}`},
		{Type: LogTypeManage, Other: fmt.Sprintf(`{"quota_pool_id":%d,"op":{"action":"quota_pool.update"}}`, pool.Id)},
		{Type: LogTypeTopup, Other: `{"quota_pool_id":10}`},
	}).Error)
	items, total, err := ListQuotaPoolOperationLogs(pool.Id, &common.PageInfo{Page: 1, PageSize: 10}, "")
	require.NoError(t, err)
	assert.EqualValues(t, 2, total)
	assert.Len(t, items, 2)
	items, total, err = ListQuotaPoolOperationLogs(pool.Id, &common.PageInfo{Page: 1, PageSize: 10}, "quota_pool.update")
	require.NoError(t, err)
	assert.EqualValues(t, 1, total)
	assert.Len(t, items, 1)
}

func TestQuotaPoolOperationLogsMergeAuditHistoryWithRoleProjection(t *testing.T) {
	db := setupQuotaPoolFundsTestDB(t)
	require.NoError(t, db.AutoMigrate(&Log{}, &AuditLog{}))
	previous := LOG_DB
	LOG_DB = db
	t.Cleanup(func() { LOG_DB = previous })
	verifyQuotaPoolOperationLogHistory(t, db)
}

func verifyQuotaPoolOperationLogHistory(t *testing.T, logDB *gorm.DB) {
	t.Helper()
	legacy := []Log{
		{Type: LogTypeManage, CreatedAt: 50, Other: `{"quota_pool_id":7,"op":{"action":"quota_pool.member_add"},"admin_info":{"admin_id":8},"root_info":{"secret":true}}`},
		{Type: LogTypeTopup, CreatedAt: 100, Other: `{"quota_pool_id":7,"recharge_source":"auto"}`},
		{Type: LogTypeTopup, CreatedAt: 300, RequestId: "member-recharge", Other: `{"quota_pool_id":7,"op":{"action":"quota_pool.member_recharge"}}`},
		{Type: LogTypeTopup, CreatedAt: 400, RequestId: "root-recharge", Other: `{"quota_pool_id":7,"op":{"action":"quota_pool.member_recharge"}}`},
	}
	require.NoError(t, logDB.Create(&legacy).Error)
	for _, event := range []struct {
		role      int
		timestamp int64
		requestId string
		action    string
		poolId    int
	}{
		{common.RoleQuotaPoolSuperAdmin, 200, "role-two", "quota_pool.member_add", 7},
		{common.RoleAdminUser, 300, "member-recharge", "quota_pool.member_recharge", 7},
		{common.RoleRootUser, 400, "root-recharge", "quota_pool.member_recharge", 7},
		{0, 500, "unknown-role", "quota_pool.member_remove", 7},
		{common.RoleAdminUser, 600, "other-pool", "quota_pool.member_add", 8},
	} {
		RecordAuditLog(nil, AuditLog{
			Username: "operator", ActorRole: event.role, CreatedAt: event.timestamp, RequestId: event.requestId,
			Category: AuditCategoryOperation, Action: event.action, Success: true,
			Other: AuditOther{
				Op:        &AuditOperation{Action: event.action, Params: AuditFields{"quota_pool_id": event.poolId}},
				AdminInfo: &AuditAdminInfo{AdminID: 8, AdminRole: event.role},
				AuditInfo: &AuditRequestInfo{Method: "POST", Status: 200, Success: true},
				RootInfo:  AuditFields{"secret": true},
			},
		})
	}

	items, total, err := ListQuotaPoolOperationLogs(7, &common.PageInfo{Page: 1, PageSize: 10}, "")
	require.NoError(t, err)
	assert.EqualValues(t, 5, total)
	require.Len(t, items, 5)
	assert.Equal(t, "root-recharge", items[0].RequestId)
	assert.Equal(t, LogTypeTopup, items[0].Type)
	assert.NotEmpty(t, items[0].EventId)
	assert.Negative(t, items[0].Id)
	assert.Equal(t, common.RoleRootUser, items[0].ActorRole)
	assert.Equal(t, "member-recharge", items[1].RequestId)
	assert.Equal(t, common.RoleQuotaPoolSuperAdmin, items[2].ActorRole)
	for _, item := range items {
		var other map[string]any
		require.NoError(t, common.UnmarshalJsonStr(item.Other, &other))
		assert.NotContains(t, other, "admin_info")
		assert.NotContains(t, other, "root_info")
		assert.NotContains(t, other, "audit_info")
	}
	items, total, err = ListQuotaPoolOperationLogs(7, &common.PageInfo{Page: 2, PageSize: 1}, "")
	require.NoError(t, err)
	assert.EqualValues(t, 5, total)
	require.Len(t, items, 1)
	assert.Equal(t, "member-recharge", items[0].RequestId)
	items, total, err = ListQuotaPoolOperationLogs(7, &common.PageInfo{Page: 1, PageSize: 10}, "quota_pool.member_add")
	require.NoError(t, err)
	assert.EqualValues(t, 2, total)
	assert.Len(t, items, 2)

	for _, view := range []struct {
		name     string
		options  QuotaPoolOperationLogView
		total    int64
		hasAdmin bool
		hasRoot  bool
	}{
		{"pool_manager_self", QuotaPoolOperationLogView{ViewerRole: common.RoleCommonUser, SelfView: true}, 5, false, false},
		{"role_two", QuotaPoolOperationLogView{ViewerRole: common.RoleQuotaPoolSuperAdmin}, 5, false, false},
		{"admin", QuotaPoolOperationLogView{ViewerRole: common.RoleAdminUser}, 5, true, false},
		{"admin_self", QuotaPoolOperationLogView{ViewerRole: common.RoleAdminUser, SelfView: true}, 5, false, false},
		{"root", QuotaPoolOperationLogView{ViewerRole: common.RoleRootUser}, 6, true, true},
		{"root_self", QuotaPoolOperationLogView{ViewerRole: common.RoleRootUser, SelfView: true}, 6, false, false},
	} {
		t.Run(view.name, func(t *testing.T) {
			items, total, err := ListQuotaPoolOperationLogs(7, &common.PageInfo{Page: 1, PageSize: 20}, "", view.options)
			require.NoError(t, err)
			assert.Equal(t, view.total, total)
			require.Len(t, items, int(view.total))
			var rootRecharges, unknownActors int
			for _, item := range items {
				assert.NotEqual(t, "other-pool", item.RequestId)
				switch item.RequestId {
				case "root-recharge":
					rootRecharges++
					assert.Equal(t, common.RoleRootUser, item.ActorRole)
					assert.Equal(t, LogTypeTopup, item.Type)
					assert.NotEmpty(t, item.EventId)
				case "unknown-role":
					unknownActors++
				}
				if item.EventId == "" {
					continue
				}
				var other map[string]any
				require.NoError(t, common.UnmarshalJsonStr(item.Other, &other))
				_, hasAdmin := other["admin_info"]
				_, hasAudit := other["audit_info"]
				_, hasRoot := other["root_info"]
				assert.Equal(t, view.hasAdmin, hasAdmin)
				assert.Equal(t, view.hasAdmin, hasAudit)
				assert.Equal(t, view.hasRoot, hasRoot)
			}
			assert.Equal(t, 1, rootRecharges)
			if view.options.ViewerRole < common.RoleRootUser {
				assert.Zero(t, unknownActors)
			} else {
				assert.Equal(t, 1, unknownActors)
			}
		})
	}

	for _, role := range []int{common.RoleCommonUser, common.RoleQuotaPoolSuperAdmin, common.RoleAdminUser} {
		audits, total, err := GetAuditLogs(AuditLogFilter{RequestId: "role-two"}, 0, 10, role)
		require.NoError(t, err)
		assert.EqualValues(t, 1, total)
		require.Len(t, audits, 1)
		assert.Equal(t, common.RoleQuotaPoolSuperAdmin, audits[0].ActorRole)
		assert.Empty(t, audits[0].Other.RootInfo)
		audits, total, err = GetAuditLogs(AuditLogFilter{RequestId: "root-recharge"}, 0, 10, role)
		require.NoError(t, err)
		assert.Zero(t, total)
		assert.Empty(t, audits)
	}
}
