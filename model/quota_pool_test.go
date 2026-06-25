package model

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func setupQuotaPoolTestDB(t *testing.T) (*gorm.DB, func()) {
	t.Helper()

	oldDB := DB
	oldLogDB := LOG_DB
	oldRedisEnabled := common.RedisEnabled
	oldUsingSQLite := common.UsingSQLite
	oldUsingMySQL := common.UsingMySQL
	oldUsingPostgreSQL := common.UsingPostgreSQL

	common.RedisEnabled = false
	common.UsingSQLite = true
	common.UsingMySQL = false
	common.UsingPostgreSQL = false
	initCol()

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite db failed: %v", err)
	}
	DB = db
	LOG_DB = db
	if err := db.AutoMigrate(&User{}, &QuotaPool{}, &QuotaPoolAdmin{}, &QuotaPoolTransaction{}); err != nil {
		t.Fatalf("migrate quota pool test tables failed: %v", err)
	}
	cleanup := func() {
		sqlDB, err := db.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
		DB = oldDB
		LOG_DB = oldLogDB
		common.RedisEnabled = oldRedisEnabled
		common.UsingSQLite = oldUsingSQLite
		common.UsingMySQL = oldUsingMySQL
		common.UsingPostgreSQL = oldUsingPostgreSQL
		initCol()
	}
	return db, cleanup
}

func createQuotaPoolTestUser(t *testing.T, db *gorm.DB, id int, quota int, poolId int) *User {
	t.Helper()
	user := &User{
		Id:          id,
		Username:    fmt.Sprintf("user_%d", id),
		Password:    "password",
		Role:        common.RoleCommonUser,
		Status:      common.UserStatusEnabled,
		Quota:       quota,
		QuotaPoolId: poolId,
		Group:       "default",
		AffCode:     fmt.Sprintf("user_%d_code", id),
	}
	if err := db.Create(user).Error; err != nil {
		t.Fatalf("create user failed: %v", err)
	}
	return user
}

func createQuotaPoolForTest(t *testing.T, db *gorm.DB, quota int) *QuotaPool {
	t.Helper()
	pool := &QuotaPool{
		Name:               fmt.Sprintf("pool_%d", time.Now().UnixNano()),
		Enabled:            true,
		BaseQuota:          quota,
		Quota:              quota,
		AutoRechargeAmount: QuotaPoolAutoRechargeInherit,
		WeeklyLimit:        QuotaPoolAutoRechargeInherit,
		MonthlyLimit:       QuotaPoolAutoRechargeInherit,
		MonthlyRefillDay:   1,
	}
	if err := db.Create(pool).Error; err != nil {
		t.Fatalf("create quota pool failed: %v", err)
	}
	return pool
}

func TestDefaultQuotaPoolMonthlyRefillDayCapsAt28(t *testing.T) {
	cases := []struct {
		now  time.Time
		want int
	}{
		{now: time.Date(2026, 2, 24, 0, 0, 0, 0, time.Local), want: 24},
		{now: time.Date(2026, 3, 29, 0, 0, 0, 0, time.Local), want: 28},
		{now: time.Date(2026, 5, 31, 0, 0, 0, 0, time.Local), want: 28},
	}
	for _, testCase := range cases {
		got := defaultQuotaPoolMonthlyRefillDay(testCase.now)
		if got != testCase.want {
			t.Fatalf("default monthly refill day for %s = %d, want %d", testCase.now.Format("2006-01-02"), got, testCase.want)
		}
	}
}

func TestCreateQuotaPoolDefaultsMonthlyRefill(t *testing.T) {
	db, cleanup := setupQuotaPoolTestDB(t)
	defer cleanup()

	pool, err := CreateQuotaPool("team-default-refill", 1000, 1)
	if err != nil {
		t.Fatalf("create pool failed: %v", err)
	}

	var got QuotaPool
	if err := db.First(&got, pool.Id).Error; err != nil {
		t.Fatalf("load pool failed: %v", err)
	}
	if !got.MonthlyRefillEnabled {
		t.Fatalf("expected monthly refill enabled by default")
	}
	if got.MonthlyRefillAmount != got.BaseQuota || got.MonthlyRefillAmount != 1000 {
		t.Fatalf("monthly refill amount = %d, want base quota %d", got.MonthlyRefillAmount, got.BaseQuota)
	}
	wantDay := defaultQuotaPoolMonthlyRefillDay(time.Now())
	if got.MonthlyRefillDay != wantDay {
		t.Fatalf("monthly refill day = %d, want %d", got.MonthlyRefillDay, wantDay)
	}
	now := time.Now()
	wantMonth := now.Year()*100 + int(now.Month())
	if got.LastRefillMonth != wantMonth {
		t.Fatalf("last refill month = %d, want current month %d", got.LastRefillMonth, wantMonth)
	}
}

func TestListQuotaPoolsDoesNotCreateDefaultPool(t *testing.T) {
	_, cleanup := setupQuotaPoolTestDB(t)
	defer cleanup()

	pool, err := CreateQuotaPool("team-list-default", 1000, 1)
	if err != nil {
		t.Fatalf("create pool failed: %v", err)
	}
	items, err := ListQuotaPools()
	if err != nil {
		t.Fatalf("list quota pools failed: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("pool count = %d, want 1", len(items))
	}
	if items[0].Id != pool.Id || items[0].IsDefault {
		t.Fatalf("only existing non-default pool should be listed, got %+v", items)
	}
}

func TestListQuotaPoolsIncludesSystemAutoRechargeDefaults(t *testing.T) {
	_, cleanup := setupQuotaPoolTestDB(t)
	defer cleanup()
	cfg := operation_setting.GetAutoRechargeSetting()
	original := *cfg
	defer func() {
		*cfg = original
	}()
	cfg.Enabled = true
	cfg.Interval = 15
	cfg.Threshold = 7
	cfg.Amount = 3
	cfg.WeeklyLimit = 5
	cfg.MonthlyLimit = 9

	if _, err := CreateQuotaPool("team-system-defaults", 1000, 1); err != nil {
		t.Fatalf("create pool failed: %v", err)
	}
	items, err := ListQuotaPools()
	if err != nil {
		t.Fatalf("list quota pools failed: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("pool count = %d, want 1", len(items))
	}
	defaults := items[0].SystemAutoRecharge
	if !defaults.Enabled ||
		defaults.Interval != 15 ||
		defaults.Threshold != int(7*common.QuotaPerUnit) ||
		defaults.Amount != int(3*common.QuotaPerUnit) ||
		defaults.WeeklyLimit != 5 ||
		defaults.MonthlyLimit != 9 {
		t.Fatalf("unexpected system auto recharge defaults: %+v", defaults)
	}
}

func TestSyncDefaultQuotaPoolCreatesDefaultPool(t *testing.T) {
	db, cleanup := setupQuotaPoolTestDB(t)
	defer cleanup()

	pool, err := SyncDefaultQuotaPool()
	if err != nil {
		t.Fatalf("sync default pool failed: %v", err)
	}
	if pool.Name != "系统默认额度池" || !pool.Enabled || !pool.IsDefault {
		t.Fatalf("unexpected default pool: %+v", pool)
	}
	if pool.BaseQuota != QuotaPoolUnlimitedQuota || pool.Quota != QuotaPoolUnlimitedQuota {
		t.Fatalf("default quota = %d/%d, want unlimited", pool.Quota, pool.BaseQuota)
	}

	var defaultCount int64
	_ = db.Model(&QuotaPool{}).Where("is_default = ?", true).Count(&defaultCount).Error
	if defaultCount != 1 {
		t.Fatalf("default pool count = %d, want 1", defaultCount)
	}

	syncedAgain, err := SyncDefaultQuotaPool()
	if err != nil {
		t.Fatalf("sync default pool again failed: %v", err)
	}
	if syncedAgain.Id != pool.Id {
		t.Fatalf("sync default pool again id = %d, want %d", syncedAgain.Id, pool.Id)
	}
}

func TestListQuotaPoolsBackfillsBaseQuotaFromFundingTransactions(t *testing.T) {
	db, cleanup := setupQuotaPoolTestDB(t)
	defer cleanup()

	pool := createQuotaPoolForTest(t, db, 1000)
	if err := db.Model(&QuotaPool{}).Where("id = ?", pool.Id).Update("base_quota", 1000).Error; err != nil {
		t.Fatalf("reset base quota failed: %v", err)
	}
	for _, tx := range []QuotaPoolTransaction{
		{PoolId: pool.Id, Type: QuotaPoolTransactionInitialFund, Amount: 1000},
		{PoolId: pool.Id, Type: QuotaPoolTransactionManualRefill, Amount: 500},
		{PoolId: pool.Id, Type: QuotaPoolTransactionMonthlyRefill, Amount: 300},
		{PoolId: pool.Id, Type: QuotaPoolTransactionAllocateManual, Amount: -200},
	} {
		if err := db.Create(&tx).Error; err != nil {
			t.Fatalf("create transaction failed: %v", err)
		}
	}

	items, err := ListQuotaPools()
	if err != nil {
		t.Fatalf("list quota pools failed: %v", err)
	}
	var got *QuotaPoolListItem
	for i := range items {
		if items[i].Id == pool.Id {
			got = &items[i]
			break
		}
	}
	if got == nil {
		t.Fatalf("pool not found in list: %+v", items)
	}
	if got.BaseQuota != 1800 {
		t.Fatalf("listed base quota = %d, want 1800", got.BaseQuota)
	}
	var updated QuotaPool
	_ = db.First(&updated, pool.Id).Error
	if updated.BaseQuota != 1800 {
		t.Fatalf("stored base quota = %d, want 1800", updated.BaseQuota)
	}
}

func TestTransferQuotaFromPoolToUserDebitsPoolAndCreditsUser(t *testing.T) {
	db, cleanup := setupQuotaPoolTestDB(t)
	defer cleanup()

	pool := createQuotaPoolForTest(t, db, 1000)
	user := createQuotaPoolTestUser(t, db, 1, 10, pool.Id)

	result, err := TransferQuotaFromPoolToUser(pool.Id, user.Id, 200)
	if err != nil {
		t.Fatalf("transfer failed: %v", err)
	}
	if !result.PoolChanged {
		t.Fatalf("expected non-default pool to be changed")
	}
	if result.Change.QuotaBefore != 1000 || result.Change.QuotaAfter != 800 || result.Change.Amount != -200 {
		t.Fatalf("unexpected change: %+v", result.Change)
	}

	var gotPool QuotaPool
	if err := db.First(&gotPool, pool.Id).Error; err != nil {
		t.Fatalf("load pool failed: %v", err)
	}
	if gotPool.Quota != 800 {
		t.Fatalf("pool quota = %d, want 800", gotPool.Quota)
	}
	var gotUser User
	if err := db.First(&gotUser, user.Id).Error; err != nil {
		t.Fatalf("load user failed: %v", err)
	}
	if gotUser.Quota != 210 {
		t.Fatalf("user quota = %d, want 210", gotUser.Quota)
	}
}

func TestTransferQuotaFromPoolToUserRejectsInsufficientPool(t *testing.T) {
	db, cleanup := setupQuotaPoolTestDB(t)
	defer cleanup()

	pool := createQuotaPoolForTest(t, db, 100)
	user := createQuotaPoolTestUser(t, db, 1, 10, pool.Id)

	_, err := TransferQuotaFromPoolToUser(pool.Id, user.Id, 200)
	if err == nil {
		t.Fatalf("expected insufficient quota error")
	}

	var gotPool QuotaPool
	_ = db.First(&gotPool, pool.Id).Error
	if gotPool.Quota != 100 {
		t.Fatalf("pool quota = %d, want 100", gotPool.Quota)
	}
	var gotUser User
	_ = db.First(&gotUser, user.Id).Error
	if gotUser.Quota != 10 {
		t.Fatalf("user quota = %d, want 10", gotUser.Quota)
	}
}

func TestTransferQuotaFromPoolToUserRejectsWhenConditionalDebitAffectsNoRows(t *testing.T) {
	db, cleanup := setupQuotaPoolTestDB(t)
	defer cleanup()

	pool := createQuotaPoolForTest(t, db, 200)
	user := createQuotaPoolTestUser(t, db, 1, 10, pool.Id)

	callbackName := "quota_pool_test:drain_pool_before_debit"
	drained := false
	if err := db.Callback().Update().Before("gorm:update").Register(callbackName, func(tx *gorm.DB) {
		if drained {
			return
		}
		if tx.Statement.Schema == nil || tx.Statement.Schema.Name != "QuotaPool" || tx.Statement.Table != "quota_pools" {
			return
		}
		drained = true
		tx.Exec("UPDATE quota_pools SET quota = 100 WHERE id = ?", pool.Id)
	}); err != nil {
		t.Fatalf("register update callback failed: %v", err)
	}
	defer db.Callback().Update().Remove(callbackName)

	_, err := TransferQuotaFromPoolToUser(pool.Id, user.Id, 200)
	if !errors.Is(err, ErrQuotaPoolInsufficientQuota) {
		t.Fatalf("expected insufficient quota after conditional debit miss, got %v", err)
	}

	var gotPool QuotaPool
	_ = db.First(&gotPool, pool.Id).Error
	if gotPool.Quota != 200 {
		t.Fatalf("pool quota = %d, want unchanged 200", gotPool.Quota)
	}
	var gotUser User
	_ = db.First(&gotUser, user.Id).Error
	if gotUser.Quota != 10 {
		t.Fatalf("user quota = %d, want unchanged 10", gotUser.Quota)
	}
}

func TestTransferQuotaFromPoolToUserRejectsNonMember(t *testing.T) {
	db, cleanup := setupQuotaPoolTestDB(t)
	defer cleanup()

	pool := createQuotaPoolForTest(t, db, 1000)
	otherPool := createQuotaPoolForTest(t, db, 1000)
	user := createQuotaPoolTestUser(t, db, 1, 10, otherPool.Id)

	_, err := TransferQuotaFromPoolToUser(pool.Id, user.Id, 200)
	if !errors.Is(err, ErrQuotaPoolMemberMismatch) {
		t.Fatalf("expected member mismatch error, got %v", err)
	}

	var gotPool QuotaPool
	_ = db.First(&gotPool, pool.Id).Error
	if gotPool.Quota != 1000 {
		t.Fatalf("pool quota = %d, want 1000", gotPool.Quota)
	}
	var gotUser User
	_ = db.First(&gotUser, user.Id).Error
	if gotUser.Quota != 10 {
		t.Fatalf("user quota = %d, want 10", gotUser.Quota)
	}
}

func TestReclaimQuotaFromUserToPoolCreditsPoolAndDebitsUser(t *testing.T) {
	db, cleanup := setupQuotaPoolTestDB(t)
	defer cleanup()

	pool := createQuotaPoolForTest(t, db, 1000)
	user := createQuotaPoolTestUser(t, db, 1, 300, pool.Id)

	result, err := ReclaimQuotaFromUserToPool(pool.Id, user.Id, 200, 50)
	if err != nil {
		t.Fatalf("reclaim failed: %v", err)
	}
	if !result.PoolChanged {
		t.Fatalf("expected non-default pool to be changed")
	}
	if result.Change.QuotaBefore != 1000 || result.Change.QuotaAfter != 1200 || result.Change.Amount != 200 {
		t.Fatalf("unexpected change: %+v", result.Change)
	}

	var gotPool QuotaPool
	if err := db.First(&gotPool, pool.Id).Error; err != nil {
		t.Fatalf("load pool failed: %v", err)
	}
	if gotPool.Quota != 1200 {
		t.Fatalf("pool quota = %d, want 1200", gotPool.Quota)
	}
	var gotUser User
	if err := db.First(&gotUser, user.Id).Error; err != nil {
		t.Fatalf("load user failed: %v", err)
	}
	if gotUser.Quota != 100 {
		t.Fatalf("user quota = %d, want 100", gotUser.Quota)
	}
}

func TestReclaimQuotaFromUserToPoolRejectsInsufficientUserQuota(t *testing.T) {
	db, cleanup := setupQuotaPoolTestDB(t)
	defer cleanup()

	pool := createQuotaPoolForTest(t, db, 1000)
	user := createQuotaPoolTestUser(t, db, 1, 100, pool.Id)

	_, err := ReclaimQuotaFromUserToPool(pool.Id, user.Id, 200, 0)
	if !errors.Is(err, ErrQuotaPoolInsufficientUserQuota) {
		t.Fatalf("expected insufficient user quota error, got %v", err)
	}

	var gotPool QuotaPool
	_ = db.First(&gotPool, pool.Id).Error
	if gotPool.Quota != 1000 {
		t.Fatalf("pool quota = %d, want 1000", gotPool.Quota)
	}
	var gotUser User
	_ = db.First(&gotUser, user.Id).Error
	if gotUser.Quota != 100 {
		t.Fatalf("user quota = %d, want 100", gotUser.Quota)
	}
}

func TestReclaimQuotaFromUserToPoolRejectsAutoRechargeThreshold(t *testing.T) {
	db, cleanup := setupQuotaPoolTestDB(t)
	defer cleanup()

	pool := createQuotaPoolForTest(t, db, 1000)
	user := createQuotaPoolTestUser(t, db, 1, 300, pool.Id)

	_, err := ReclaimQuotaFromUserToPool(pool.Id, user.Id, 200, 100)
	if !errors.Is(err, ErrQuotaPoolReclaimTriggersAuto) {
		t.Fatalf("expected auto recharge threshold error, got %v", err)
	}

	var gotPool QuotaPool
	_ = db.First(&gotPool, pool.Id).Error
	if gotPool.Quota != 1000 {
		t.Fatalf("pool quota = %d, want 1000", gotPool.Quota)
	}
	var gotUser User
	_ = db.First(&gotUser, user.Id).Error
	if gotUser.Quota != 300 {
		t.Fatalf("user quota = %d, want 300", gotUser.Quota)
	}
}

func TestListQuotaPoolMembersAndCandidatesOmitAccessToken(t *testing.T) {
	db, cleanup := setupQuotaPoolTestDB(t)
	defer cleanup()

	pool := createQuotaPoolForTest(t, db, 1000)
	member := createQuotaPoolTestUser(t, db, 1, 10, pool.Id)
	candidate := createQuotaPoolTestUser(t, db, 2, 10, QuotaPoolDefaultUserPoolId)
	memberToken := "0123456789abcdef0123456789abcdef"
	candidateToken := "abcdef0123456789abcdef0123456789"
	if err := db.Model(&User{}).Where("id = ?", member.Id).Update("access_token", memberToken).Error; err != nil {
		t.Fatalf("update member access token failed: %v", err)
	}
	if err := db.Create(&QuotaPoolAdmin{PoolId: pool.Id, UserId: member.Id, Level: QuotaPoolAdminLevelV2}).Error; err != nil {
		t.Fatalf("create quota pool admin failed: %v", err)
	}
	if err := db.Model(&User{}).Where("id = ?", candidate.Id).Update("access_token", candidateToken).Error; err != nil {
		t.Fatalf("update candidate access token failed: %v", err)
	}

	members, _, err := ListQuotaPoolMembers(pool.Id, &common.PageInfo{Page: 1, PageSize: 10})
	if err != nil {
		t.Fatalf("list members failed: %v", err)
	}
	if len(members) != 1 || members[0].QuotaPoolAdminLevel != QuotaPoolAdminLevelV2 {
		t.Fatalf("expected member admin level v2, got %#v", members)
	}
	memberJSON, err := common.Marshal(members[0])
	if err != nil {
		t.Fatalf("marshal member failed: %v", err)
	}
	if strings.Contains(string(memberJSON), "access_token") || strings.Contains(string(memberJSON), memberToken) {
		t.Fatalf("expected member access token omitted, got %s", string(memberJSON))
	}

	candidates, _, err := ListDefaultQuotaPoolCandidates("", &common.PageInfo{Page: 1, PageSize: 10})
	if err != nil {
		t.Fatalf("list candidates failed: %v", err)
	}
	if len(candidates) != 1 {
		t.Fatalf("expected one candidate, got %#v", candidates)
	}
	candidateJSON, err := common.Marshal(candidates[0])
	if err != nil {
		t.Fatalf("marshal candidate failed: %v", err)
	}
	candidateJSONText := string(candidateJSON)
	if strings.Contains(candidateJSONText, "access_token") || strings.Contains(candidateJSONText, candidateToken) {
		t.Fatalf("expected candidate access token omitted, got %s", candidateJSONText)
	}
	for _, field := range []string{"role", "status", "group", "quota", "used_quota", "quota_pool_id", "created_at", "last_login_at"} {
		if strings.Contains(candidateJSONText, `"`+field+`"`) {
			t.Fatalf("expected candidate field %s omitted, got %s", field, candidateJSONText)
		}
	}
	candidatesById, _, err := ListDefaultQuotaPoolCandidates(fmt.Sprintf("%d", candidate.Id), &common.PageInfo{Page: 1, PageSize: 10})
	if err != nil {
		t.Fatalf("list candidates by id failed: %v", err)
	}
	if len(candidatesById) != 1 || candidatesById[0].Id != candidate.Id {
		t.Fatalf("expected candidate searchable by id, got %#v", candidatesById)
	}
}

func TestListDefaultQuotaPoolCandidatesIncludesSystemAdmin(t *testing.T) {
	db, cleanup := setupQuotaPoolTestDB(t)
	defer cleanup()

	commonUser := createQuotaPoolTestUser(t, db, 1, 10, QuotaPoolDefaultUserPoolId)
	systemAdmin := createQuotaPoolTestUser(t, db, 2, 10, QuotaPoolDefaultUserPoolId)
	rootUser := createQuotaPoolTestUser(t, db, 3, 10, QuotaPoolDefaultUserPoolId)
	if err := db.Model(&User{}).Where("id = ?", systemAdmin.Id).Update("role", common.RoleAdminUser).Error; err != nil {
		t.Fatalf("update system admin role failed: %v", err)
	}
	if err := db.Model(&User{}).Where("id = ?", rootUser.Id).Update("role", common.RoleRootUser).Error; err != nil {
		t.Fatalf("update root role failed: %v", err)
	}

	candidates, _, err := ListDefaultQuotaPoolCandidates("", &common.PageInfo{Page: 1, PageSize: 10})
	if err != nil {
		t.Fatalf("list candidates failed: %v", err)
	}
	gotIds := map[int]bool{}
	for _, candidate := range candidates {
		gotIds[candidate.Id] = true
	}
	if !gotIds[commonUser.Id] || !gotIds[systemAdmin.Id] {
		t.Fatalf("expected common user and system admin candidates, got %#v", candidates)
	}
	if gotIds[rootUser.Id] {
		t.Fatalf("root user should not be a quota pool candidate: %#v", candidates)
	}
}

func TestListQuotaPoolTransactionsIncludesUserNames(t *testing.T) {
	db, cleanup := setupQuotaPoolTestDB(t)
	defer cleanup()

	pool := createQuotaPoolForTest(t, db, 1000)
	operator := createQuotaPoolTestUser(t, db, 1, 0, pool.Id)
	member := createQuotaPoolTestUser(t, db, 2, 0, pool.Id)
	records := []*QuotaPoolTransaction{
		{
			PoolId:      pool.Id,
			Type:        QuotaPoolTransactionAllocateManual,
			Amount:      -100,
			QuotaBefore: 1000,
			QuotaAfter:  900,
			UserId:      member.Id,
			OperatorId:  operator.Id,
		},
		{
			PoolId:      pool.Id,
			Type:        QuotaPoolTransactionAllocateAuto,
			Amount:      -100,
			QuotaBefore: 900,
			QuotaAfter:  800,
			UserId:      member.Id,
			OperatorId:  0,
		},
		{
			PoolId:      pool.Id,
			Type:        QuotaPoolTransactionManualRefill,
			Amount:      100,
			QuotaBefore: 800,
			QuotaAfter:  900,
			UserId:      999,
			OperatorId:  operator.Id,
		},
	}
	if err := db.Create(&records).Error; err != nil {
		t.Fatalf("create transactions failed: %v", err)
	}

	items, total, err := ListQuotaPoolTransactions(pool.Id, &common.PageInfo{Page: 1, PageSize: 10})
	if err != nil {
		t.Fatalf("list transactions failed: %v", err)
	}
	if total != 3 || len(items) != 3 {
		t.Fatalf("expected 3 transactions, got total=%d len=%d", total, len(items))
	}
	if items[0].OperatorName != operator.Username || items[0].UserName != "" {
		t.Fatalf("expected missing user and named operator, got user=%q operator=%q", items[0].UserName, items[0].OperatorName)
	}
	if items[1].UserName != member.Username || items[1].OperatorName != "" {
		t.Fatalf("expected named user and system operator, got user=%q operator=%q", items[1].UserName, items[1].OperatorName)
	}
	if items[2].UserName != member.Username || items[2].OperatorName != operator.Username {
		t.Fatalf("expected named user/operator, got user=%q operator=%q", items[2].UserName, items[2].OperatorName)
	}
}

func TestGetQuotaPoolStatsScopesUsageAndRechargeToPool(t *testing.T) {
	db, cleanup := setupQuotaPoolTestDB(t)
	defer cleanup()
	if err := db.AutoMigrate(&Log{}); err != nil {
		t.Fatalf("migrate log table failed: %v", err)
	}

	pool := createQuotaPoolForTest(t, db, 1000)
	otherPool := createQuotaPoolForTest(t, db, 1000)
	member := createQuotaPoolTestUser(t, db, 1, 100, pool.Id)
	otherMember := createQuotaPoolTestUser(t, db, 2, 100, otherPool.Id)
	now := time.Now().Unix()
	oldTs := now - 10*24*60*60

	logs := []*Log{
		{UserId: member.Id, Username: member.Username, Type: LogTypeConsume, ModelName: "gpt-4o", Quota: 60, CreatedAt: now},
		{UserId: member.Id, Username: member.Username, Type: LogTypeConsume, ModelName: "claude-3-5-sonnet", Quota: 40, CreatedAt: now},
		{UserId: member.Id, Username: member.Username, Type: LogTypeConsume, ModelName: "gpt-4o", Quota: 999, CreatedAt: oldTs},
		{UserId: otherMember.Id, Username: otherMember.Username, Type: LogTypeConsume, ModelName: "gpt-4o", Quota: 200, CreatedAt: now},
	}
	if err := db.Create(&logs).Error; err != nil {
		t.Fatalf("create logs failed: %v", err)
	}
	transactions := []*QuotaPoolTransaction{
		{PoolId: pool.Id, Type: QuotaPoolTransactionManualRefill, Amount: 300, CreatedAt: now},
		{PoolId: pool.Id, Type: QuotaPoolTransactionAllocateAuto, Amount: -80, UserId: member.Id, CreatedAt: now},
		{PoolId: pool.Id, Type: QuotaPoolTransactionAllocateManual, Amount: -20, UserId: member.Id, CreatedAt: now},
		{PoolId: pool.Id, Type: QuotaPoolTransactionReclaimUser, Amount: 50, UserId: member.Id, CreatedAt: now},
		{PoolId: pool.Id, Type: QuotaPoolTransactionMonthlyRefill, Amount: 999, CreatedAt: oldTs},
		{PoolId: otherPool.Id, Type: QuotaPoolTransactionManualRefill, Amount: 500, CreatedAt: now},
	}
	if err := db.Create(&transactions).Error; err != nil {
		t.Fatalf("create transactions failed: %v", err)
	}

	stats, err := GetQuotaPoolStats(pool.Id, now-60, now+60)
	if err != nil {
		t.Fatalf("GetQuotaPoolStats returned error: %v", err)
	}
	if stats.TotalUsage != 100 {
		t.Fatalf("total usage = %d, want 100", stats.TotalUsage)
	}
	if stats.TotalRefill != 300 {
		t.Fatalf("total refill = %d, want 300", stats.TotalRefill)
	}
	if stats.TotalAllocate != 100 {
		t.Fatalf("total allocate = %d, want 100", stats.TotalAllocate)
	}
	if len(stats.Usage) != 1 {
		t.Fatalf("usage length = %d, want 1", len(stats.Usage))
	}
	if stats.Usage[0].UserId != member.Id || stats.Usage[0].GptQuota != 60 || stats.Usage[0].ClaudeQuota != 40 {
		t.Fatalf("unexpected usage stat: %+v", stats.Usage[0])
	}
	if len(stats.Recharge) != 4 {
		t.Fatalf("recharge length = %d, want 4", len(stats.Recharge))
	}
	for _, item := range stats.Recharge {
		if item.Type == QuotaPoolTransactionReclaimUser && item.Amount != -50 {
			t.Fatalf("reclaim amount = %d, want -50", item.Amount)
		}
	}
}

func TestTransferQuotaFromDefaultPoolUsesUserQuotaOnly(t *testing.T) {
	db, cleanup := setupQuotaPoolTestDB(t)
	defer cleanup()

	user := createQuotaPoolTestUser(t, db, 1, 10, QuotaPoolDefaultUserPoolId)

	result, err := TransferQuotaFromPoolToUser(QuotaPoolDefaultUserPoolId, user.Id, 200)
	if err != nil {
		t.Fatalf("default transfer failed: %v", err)
	}
	if result.PoolChanged {
		t.Fatalf("default pool should not be marked changed")
	}
	var gotUser User
	_ = db.First(&gotUser, user.Id).Error
	if gotUser.Quota != 210 {
		t.Fatalf("user quota = %d, want 210", gotUser.Quota)
	}
}

func TestMoveUserQuotaPoolRejectsSamePoolWithoutClearingQuota(t *testing.T) {
	db, cleanup := setupQuotaPoolTestDB(t)
	defer cleanup()

	pool := createQuotaPoolForTest(t, db, 500)
	user := createQuotaPoolTestUser(t, db, 1, 120, pool.Id)

	_, err := MoveUserQuotaPool(user.Id, pool.Id)
	if !errors.Is(err, ErrQuotaPoolSamePool) {
		t.Fatalf("expected same pool error, got %v", err)
	}

	var gotUser User
	_ = db.First(&gotUser, user.Id).Error
	if gotUser.Quota != 120 || gotUser.QuotaPoolId != pool.Id {
		t.Fatalf("unexpected user after same-pool move: quota=%d pool=%d", gotUser.Quota, gotUser.QuotaPoolId)
	}
}

func TestMoveUserQuotaPoolReclaimsOldFinitePoolQuota(t *testing.T) {
	db, cleanup := setupQuotaPoolTestDB(t)
	defer cleanup()

	oldPool := createQuotaPoolForTest(t, db, 500)
	newPool := createQuotaPoolForTest(t, db, 700)
	user := createQuotaPoolTestUser(t, db, 1, 120, oldPool.Id)
	if err := db.Create(&QuotaPoolAdmin{PoolId: oldPool.Id, UserId: user.Id, Level: QuotaPoolAdminLevelV1}).Error; err != nil {
		t.Fatalf("create admin failed: %v", err)
	}

	result, err := MoveUserQuotaPool(user.Id, newPool.Id)
	if err != nil {
		t.Fatalf("move failed: %v", err)
	}
	if !result.Reclaimed || result.Change.QuotaBefore != 500 || result.Change.QuotaAfter != 620 {
		t.Fatalf("unexpected reclaim result: %+v", result)
	}

	var gotUser User
	_ = db.First(&gotUser, user.Id).Error
	if gotUser.Quota != 0 || gotUser.QuotaPoolId != newPool.Id {
		t.Fatalf("unexpected user after move: quota=%d pool=%d", gotUser.Quota, gotUser.QuotaPoolId)
	}
	var gotOldPool QuotaPool
	_ = db.First(&gotOldPool, oldPool.Id).Error
	if gotOldPool.Quota != 620 {
		t.Fatalf("old pool quota = %d, want 620", gotOldPool.Quota)
	}
	var adminCount int64
	_ = db.Model(&QuotaPoolAdmin{}).Where("user_id = ?", user.Id).Count(&adminCount).Error
	if adminCount != 0 {
		t.Fatalf("expected old pool admin relation removed, got %d", adminCount)
	}
}

func TestGrantQuotaPoolAdminRejectsDisabledPool(t *testing.T) {
	db, cleanup := setupQuotaPoolTestDB(t)
	defer cleanup()

	pool := createQuotaPoolForTest(t, db, 500)
	if err := db.Model(&QuotaPool{}).Where("id = ?", pool.Id).Update("enabled", false).Error; err != nil {
		t.Fatalf("disable pool failed: %v", err)
	}
	user := createQuotaPoolTestUser(t, db, 1, 0, pool.Id)

	err := GrantQuotaPoolAdmin(pool.Id, user.Id, QuotaPoolAdminLevelV1)
	if !errors.Is(err, ErrQuotaPoolDisabled) {
		t.Fatalf("expected disabled pool error, got %v", err)
	}

	var adminCount int64
	_ = db.Model(&QuotaPoolAdmin{}).Where("user_id = ?", user.Id).Count(&adminCount).Error
	if adminCount != 0 {
		t.Fatalf("expected no admin relation, got %d", adminCount)
	}
}

func TestAddQuotaPoolManualRefillLimitsAmountAndMonthlyCount(t *testing.T) {
	db, cleanup := setupQuotaPoolTestDB(t)
	defer cleanup()

	pool := createQuotaPoolForTest(t, db, 1000)

	if _, err := AddQuotaPoolManualRefill(pool.Id, 600, 99); err == nil {
		t.Fatalf("expected amount over 50 percent to fail")
	}
	if _, err := AddQuotaPoolManualRefill(pool.Id, 500, 99); err != nil {
		t.Fatalf("first refill failed: %v", err)
	}
	if _, err := AddQuotaPoolManualRefill(pool.Id, 500, 99); err != nil {
		t.Fatalf("second refill failed: %v", err)
	}
	if _, err := AddQuotaPoolManualRefill(pool.Id, 1, 99); err == nil {
		t.Fatalf("expected third refill in same month to fail")
	}

	var gotPool QuotaPool
	_ = db.First(&gotPool, pool.Id).Error
	if gotPool.Quota != 2000 {
		t.Fatalf("pool quota = %d, want 2000", gotPool.Quota)
	}
	if gotPool.BaseQuota != 2000 {
		t.Fatalf("pool base quota = %d, want 2000", gotPool.BaseQuota)
	}
}

func TestAddQuotaPoolManualRefillBackfillsBaseQuotaBeforeLimit(t *testing.T) {
	db, cleanup := setupQuotaPoolTestDB(t)
	defer cleanup()

	pool := createQuotaPoolForTest(t, db, 1000)
	for _, tx := range []QuotaPoolTransaction{
		{PoolId: pool.Id, Type: QuotaPoolTransactionInitialFund, Amount: 1000},
		{PoolId: pool.Id, Type: QuotaPoolTransactionMonthlyRefill, Amount: 1000},
	} {
		if err := db.Create(&tx).Error; err != nil {
			t.Fatalf("create transaction failed: %v", err)
		}
	}
	if err := db.Model(&QuotaPool{}).Where("id = ?", pool.Id).Updates(map[string]interface{}{"base_quota": 1000, "quota": 2000}).Error; err != nil {
		t.Fatalf("prepare legacy pool failed: %v", err)
	}

	if _, err := AddQuotaPoolManualRefill(pool.Id, 900, 99); err != nil {
		t.Fatalf("refill should use backfilled base quota: %v", err)
	}
	var gotPool QuotaPool
	_ = db.First(&gotPool, pool.Id).Error
	if gotPool.BaseQuota != 2900 {
		t.Fatalf("pool base quota = %d, want 2900", gotPool.BaseQuota)
	}
}
