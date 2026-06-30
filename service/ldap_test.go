package service

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/system_setting"
	ergoldap "github.com/ergoapi/ldap"
	"github.com/glebarez/sqlite"
	goldap "github.com/go-ldap/ldap/v3"
	"gorm.io/gorm"
)

func setupLDAPTest(t *testing.T) *gorm.DB {
	t.Helper()

	oldDB := model.DB
	oldLogDB := model.LOG_DB
	oldQuotaForNewUser := common.QuotaForNewUser
	oldRegisterEnabled := common.RegisterEnabled
	oldLDAPSettings := *system_setting.GetLDAPSettings()
	oldLDAPRequest := ldapRequest
	oldLDAPSearch := ldapSearch

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite failed: %v", err)
	}
	model.DB = db
	model.LOG_DB = db
	common.QuotaForNewUser = 0
	common.RegisterEnabled = true
	*system_setting.GetLDAPSettings() = system_setting.LDAPSettings{
		Enabled:           true,
		URL:               "ldap://ldap.example.com:389",
		BaseDN:            "ou=users,dc=example,dc=com",
		UID:               "sAMAccountName",
		Scope:             3,
		ConnectionTimeout: 30,
	}

	if err := db.AutoMigrate(&model.User{}, &model.Log{}); err != nil {
		t.Fatalf("migrate failed: %v", err)
	}

	t.Cleanup(func() {
		sqlDB, err := db.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
		model.DB = oldDB
		model.LOG_DB = oldLogDB
		common.QuotaForNewUser = oldQuotaForNewUser
		common.RegisterEnabled = oldRegisterEnabled
		*system_setting.GetLDAPSettings() = oldLDAPSettings
		ldapRequest = oldLDAPRequest
		ldapSearch = oldLDAPSearch
	})

	return db
}

func ldapSearchResult(username, email, displayName string) *goldap.SearchResult {
	return ldapSearchResultWithDepartment(username, email, displayName, "Engineering")
}

func ldapSearchResultWithDepartment(username, email, displayName, department string) *goldap.SearchResult {
	return &goldap.SearchResult{
		Entries: []*goldap.Entry{
			{
				DN: "cn=" + username + ",ou=users,dc=example,dc=com",
				Attributes: []*goldap.EntryAttribute{
					{Name: "sAMAccountName", Values: []string{username}},
					{Name: "mail", Values: []string{email}},
					{Name: "displayName", Values: []string{displayName}},
					{Name: "department", Values: []string{department}},
				},
			},
		},
	}
}

func TestLoginWithLDAPRejectsWhenDisabled(t *testing.T) {
	setupLDAPTest(t)
	system_setting.GetLDAPSettings().Enabled = false
	ldapRequest = func(conf ergoldap.LdapConf, username, password string) (*goldap.SearchResult, error) {
		t.Fatalf("ldap request should not be called when disabled")
		return nil, nil
	}

	_, err := LoginWithLDAP("alice", "secret")
	if !errors.Is(err, ErrLDAPLoginDisabled) {
		t.Fatalf("expected disabled error, got %v", err)
	}
}

func TestLoginWithLDAPCreatesUserWhenRegisterEnabled(t *testing.T) {
	setupLDAPTest(t)
	ldapRequest = func(conf ergoldap.LdapConf, username, password string) (*goldap.SearchResult, error) {
		if username != "alice" || password != "secret" {
			t.Fatalf("unexpected credentials: %s/%s", username, password)
		}
		if conf.LdapUID != "sAMAccountName" || conf.LdapScope != 3 {
			t.Fatalf("unexpected ldap config: %+v", conf)
		}
		return ldapSearchResult("alice", "alice@example.com", "Alice Doe"), nil
	}

	user, err := LoginWithLDAP("alice", "secret")
	if err != nil {
		t.Fatalf("ldap login failed: %v", err)
	}
	if user.Id == 0 || user.Username != "Alice Doe" || user.Email != "alice@example.com" || user.DisplayName != "Alice Doe" || user.Department != "Engineering" || user.LDAPId != "alice@example.com" {
		t.Fatalf("unexpected user: %+v", user)
	}
	if user.Status != common.UserStatusEnabled || user.Role != common.RoleCommonUser {
		t.Fatalf("unexpected user defaults: %+v", user)
	}
}

func TestLoginWithLDAPPrefixFallsBackToEmailSearch(t *testing.T) {
	db := setupLDAPTest(t)
	system_setting.GetLDAPSettings().UID = "mail"
	existing := model.User{
		Username:    "local",
		Password:    "password",
		DisplayName: "Local",
		Department:  "Local Department",
		Email:       "alice@example.com",
		Role:        common.RoleCommonUser,
		Status:      common.UserStatusEnabled,
		AffCode:     "local-code",
		CreatedAt:   time.Now().Unix(),
	}
	if err := db.Create(&existing).Error; err != nil {
		t.Fatalf("create existing user failed: %v", err)
	}

	var requested []string
	ldapRequest = func(conf ergoldap.LdapConf, username, password string) (*goldap.SearchResult, error) {
		requested = append(requested, username)
		if username == "alice@example.com" {
			return ldapSearchResult("alice", "alice@example.com", "Alice Doe"), nil
		}
		return nil, errors.New("not found")
	}
	ldapSearch = func(conf ergoldap.LdapConf, username string) []ergoldap.LdapUser {
		if username != "alice" {
			t.Fatalf("unexpected search username: %s", username)
		}
		return []ergoldap.LdapUser{{Username: "alice", Email: "alice@example.com"}}
	}

	user, err := LoginWithLDAP("alice", "secret")
	if err != nil {
		t.Fatalf("ldap login failed: %v", err)
	}
	if user.Id != existing.Id || user.Email != "alice@example.com" {
		t.Fatalf("unexpected user: %+v", user)
	}
	if user.Username != "local" || user.DisplayName != "Alice Doe" || user.Department != "Engineering" || user.LDAPId != "alice@example.com" {
		t.Fatalf("expected ldap profile sync, got %+v", user)
	}
	if strings.Join(requested, ",") != "alice,alice@example.com" {
		t.Fatalf("unexpected ldap request sequence: %v", requested)
	}
}

func TestLoginWithLDAPEmailFallsBackToEmailPrefix(t *testing.T) {
	db := setupLDAPTest(t)
	existing := model.User{
		Username:    "local",
		Password:    "password",
		DisplayName: "Local",
		Department:  "Local Department",
		Email:       "alice@example.com",
		Role:        common.RoleCommonUser,
		Status:      common.UserStatusEnabled,
		AffCode:     "local-code",
		CreatedAt:   time.Now().Unix(),
	}
	if err := db.Create(&existing).Error; err != nil {
		t.Fatalf("create existing user failed: %v", err)
	}

	var requested []string
	ldapRequest = func(conf ergoldap.LdapConf, username, password string) (*goldap.SearchResult, error) {
		requested = append(requested, username)
		if username == "alice" {
			return ldapSearchResult("alice", "alice@example.com", "Alice Doe"), nil
		}
		return nil, errors.New("not found")
	}
	ldapSearch = func(conf ergoldap.LdapConf, username string) []ergoldap.LdapUser {
		t.Fatalf("ldap search should not be called for email prefix fallback")
		return nil
	}

	user, err := LoginWithLDAP("alice@example.com", "secret")
	if err != nil {
		t.Fatalf("ldap login failed: %v", err)
	}
	if user.Id != existing.Id || user.Username != "local" || user.DisplayName != "Alice Doe" {
		t.Fatalf("expected existing user synced by email login, got %+v", user)
	}
	if strings.Join(requested, ",") != "alice" {
		t.Fatalf("unexpected ldap request sequence: %v", requested)
	}
}

func TestLoginWithLDAPReusesExistingUserByEmail(t *testing.T) {
	db := setupLDAPTest(t)
	existing := model.User{
		Username:    "local",
		Password:    "password",
		DisplayName: "Local",
		Department:  "Local Department",
		Email:       "alice@example.com",
		Role:        common.RoleCommonUser,
		Status:      common.UserStatusEnabled,
		AffCode:     "local-code",
		CreatedAt:   time.Now().Unix(),
	}
	if err := db.Create(&existing).Error; err != nil {
		t.Fatalf("create existing user failed: %v", err)
	}
	ldapRequest = func(conf ergoldap.LdapConf, username, password string) (*goldap.SearchResult, error) {
		return ldapSearchResult("alice", "alice@example.com", "LDAP Alice"), nil
	}

	user, err := LoginWithLDAP("alice", "secret")
	if err != nil {
		t.Fatalf("ldap login failed: %v", err)
	}
	if user.Id != existing.Id || user.Username != "local" {
		t.Fatalf("expected existing user, got %+v", user)
	}
	if user.DisplayName != "LDAP Alice" || user.Department != "Engineering" || user.LDAPId != "alice@example.com" {
		t.Fatalf("local user profile should be synced from ldap, got %+v", user)
	}

	var count int64
	if err := db.Model(&model.User{}).Count(&count).Error; err != nil {
		t.Fatalf("count users failed: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected one user, got %d", count)
	}
}

func TestLoginWithLDAPNormalizesEmailForExistingUserMatch(t *testing.T) {
	db := setupLDAPTest(t)
	existing := model.User{
		Username:    "local",
		Password:    "password",
		DisplayName: "Local",
		Department:  "Local Department",
		Email:       "alice@example.com",
		Role:        common.RoleCommonUser,
		Status:      common.UserStatusEnabled,
		AffCode:     "local-code",
		CreatedAt:   time.Now().Unix(),
	}
	if err := db.Create(&existing).Error; err != nil {
		t.Fatalf("create existing user failed: %v", err)
	}
	ldapRequest = func(conf ergoldap.LdapConf, username, password string) (*goldap.SearchResult, error) {
		return ldapSearchResult("alice", "Alice@Example.com", "LDAP Alice"), nil
	}

	user, err := LoginWithLDAP("alice", "secret")
	if err != nil {
		t.Fatalf("ldap login failed: %v", err)
	}
	if user.Id != existing.Id || user.Email != "alice@example.com" || user.LDAPId != "alice@example.com" {
		t.Fatalf("expected normalized email to reuse existing user, got %+v", user)
	}

	var count int64
	if err := db.Model(&model.User{}).Count(&count).Error; err != nil {
		t.Fatalf("count users failed: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected one user, got %d", count)
	}
}

func TestLoginWithLDAPCreatesNewUserWhenOnlyLocalUsernameMatches(t *testing.T) {
	db := setupLDAPTest(t)
	existing := model.User{
		Username:    "alice",
		Password:    "password",
		DisplayName: "Existing Alice",
		Email:       "other@example.com",
		Role:        common.RoleCommonUser,
		Status:      common.UserStatusEnabled,
		AffCode:     "alice-code",
		CreatedAt:   time.Now().Unix(),
	}
	if err := db.Create(&existing).Error; err != nil {
		t.Fatalf("create existing user failed: %v", err)
	}
	ldapRequest = func(conf ergoldap.LdapConf, username, password string) (*goldap.SearchResult, error) {
		return ldapSearchResult("alice", "alice@example.com", "LDAP Alice"), nil
	}

	user, err := LoginWithLDAP("alice", "secret")
	if err != nil {
		t.Fatalf("ldap login failed: %v", err)
	}
	if user.Id == existing.Id {
		t.Fatalf("expected a new user, got existing user: %+v", user)
	}
	if user.Username != "LDAP Alice" || user.Email != "alice@example.com" || user.DisplayName != "LDAP Alice" || user.Department != "Engineering" || user.LDAPId != "alice@example.com" {
		t.Fatalf("unexpected new user: %+v", user)
	}

	var count int64
	if err := db.Model(&model.User{}).Count(&count).Error; err != nil {
		t.Fatalf("count users failed: %v", err)
	}
	if count != 2 {
		t.Fatalf("expected two users, got %d", count)
	}
}

func TestLoginWithLDAPCreatesNextUsernameSuffixWhenDisplayNameConflicts(t *testing.T) {
	db := setupLDAPTest(t)
	existing := model.User{
		Username:    "LDAP Alice",
		Password:    "password",
		DisplayName: "Existing Alice",
		Email:       "other@example.com",
		Role:        common.RoleCommonUser,
		Status:      common.UserStatusEnabled,
		AffCode:     "alice-code",
		CreatedAt:   time.Now().Unix(),
	}
	if err := db.Create(&existing).Error; err != nil {
		t.Fatalf("create existing user failed: %v", err)
	}
	existingSuffix := model.User{
		Username:    "LDAP Alice1",
		Password:    "password",
		DisplayName: "Existing Alice 1",
		Email:       "other1@example.com",
		Role:        common.RoleCommonUser,
		Status:      common.UserStatusEnabled,
		AffCode:     "alice-code-1",
		CreatedAt:   time.Now().Unix(),
	}
	if err := db.Create(&existingSuffix).Error; err != nil {
		t.Fatalf("create existing suffix user failed: %v", err)
	}
	ldapRequest = func(conf ergoldap.LdapConf, username, password string) (*goldap.SearchResult, error) {
		return ldapSearchResult("alice", "alice@example.com", "LDAP Alice"), nil
	}

	user, err := LoginWithLDAP("alice", "secret")
	if err != nil {
		t.Fatalf("ldap login failed: %v", err)
	}
	if user.Username != "LDAP Alice2" || user.DisplayName != "LDAP Alice" || user.Department != "Engineering" {
		t.Fatalf("expected suffixed ldap username, got %+v", user)
	}
}

func TestLoginWithLDAPPreservesLongDepartment(t *testing.T) {
	setupLDAPTest(t)
	longDepartment := strings.Repeat("研发平台部", 40)
	ldapRequest = func(conf ergoldap.LdapConf, username, password string) (*goldap.SearchResult, error) {
		return ldapSearchResultWithDepartment("alice", "alice@example.com", "Alice Doe", longDepartment), nil
	}

	user, err := LoginWithLDAP("alice", "secret")
	if err != nil {
		t.Fatalf("ldap login failed: %v", err)
	}
	if user.Department != longDepartment {
		t.Fatalf("expected long department to be preserved, got %q", user.Department)
	}
}

func TestLoginWithLDAPPreservesExistingUsernameWhenDisplayNameConflicts(t *testing.T) {
	db := setupLDAPTest(t)
	conflict := model.User{
		Username:    "LDAP Alice",
		Password:    "password",
		DisplayName: "Existing Alice",
		Email:       "other@example.com",
		Role:        common.RoleCommonUser,
		Status:      common.UserStatusEnabled,
		AffCode:     "conflict-code",
		CreatedAt:   time.Now().Unix(),
	}
	existing := model.User{
		Username:    "local",
		Password:    "password",
		DisplayName: "Local",
		Department:  "Local Department",
		Email:       "alice@example.com",
		Role:        common.RoleCommonUser,
		Status:      common.UserStatusEnabled,
		AffCode:     "local-code",
		CreatedAt:   time.Now().Unix(),
	}
	if err := db.Create(&conflict).Error; err != nil {
		t.Fatalf("create conflict user failed: %v", err)
	}
	if err := db.Create(&existing).Error; err != nil {
		t.Fatalf("create existing user failed: %v", err)
	}
	ldapRequest = func(conf ergoldap.LdapConf, username, password string) (*goldap.SearchResult, error) {
		return ldapSearchResult("alice", "alice@example.com", "LDAP Alice"), nil
	}

	user, err := LoginWithLDAP("alice", "secret")
	if err != nil {
		t.Fatalf("ldap login failed: %v", err)
	}
	if user.Id != existing.Id || user.Username != "local" {
		t.Fatalf("expected existing user to keep local username, got %+v", user)
	}
	if user.DisplayName != "LDAP Alice" || user.Department != "Engineering" {
		t.Fatalf("expected ldap profile sync, got %+v", user)
	}
}

func TestLoginWithLDAPRejectsNewUserWhenRegisterDisabled(t *testing.T) {
	setupLDAPTest(t)
	common.RegisterEnabled = false
	ldapRequest = func(conf ergoldap.LdapConf, username, password string) (*goldap.SearchResult, error) {
		return ldapSearchResult("alice", "alice@example.com", "Alice Doe"), nil
	}

	_, err := LoginWithLDAP("alice", "secret")
	if err == nil || !strings.Contains(err.Error(), "关闭了新用户注册") {
		t.Fatalf("expected register disabled error, got %v", err)
	}
}

func TestSyncLDAPUserCreatesUserWhenRegisterDisabled(t *testing.T) {
	setupLDAPTest(t)
	common.RegisterEnabled = false
	ldapSearch = func(conf ergoldap.LdapConf, username string) []ergoldap.LdapUser {
		if username != "alice" {
			t.Fatalf("unexpected search username: %s", username)
		}
		return []ergoldap.LdapUser{{
			Username:    "alice",
			Email:       "alice@example.com",
			DisplayName: "Alice Doe",
			Department:  "Engineering",
		}}
	}

	user, err := SyncLDAPUser("alice")
	if err != nil {
		t.Fatalf("sync ldap user failed: %v", err)
	}
	if user.Username != "Alice Doe" || user.Email != "alice@example.com" || user.DisplayName != "Alice Doe" || user.Department != "Engineering" || user.LDAPId != "alice@example.com" {
		t.Fatalf("unexpected synced user: %+v", user)
	}
}

func TestSyncLDAPUserFallsBackToEmailSearch(t *testing.T) {
	setupLDAPTest(t)
	var requested []string
	var attrs []string
	ldapSearch = func(conf ergoldap.LdapConf, username string) []ergoldap.LdapUser {
		requested = append(requested, username)
		attrs = append(attrs, conf.LdapUID)
		if conf.LdapUID == "mail" && username == "alice@example.com" {
			return []ergoldap.LdapUser{{
				Username:    "alice",
				Email:       "alice@example.com",
				DisplayName: "Alice Doe",
				Department:  "Engineering",
			}}
		}
		return nil
	}

	user, err := SyncLDAPUser("alice@example.com")
	if err != nil {
		t.Fatalf("sync ldap user failed: %v", err)
	}
	if user.Email != "alice@example.com" || user.Username != "Alice Doe" || user.LDAPId != "alice@example.com" {
		t.Fatalf("unexpected synced user: %+v", user)
	}
	if strings.Join(requested, ",") != "alice@example.com,alice@example.com" {
		t.Fatalf("unexpected search sequence: %v", requested)
	}
	if strings.Join(attrs, ",") != "sAMAccountName,mail" {
		t.Fatalf("unexpected search attrs: %v", attrs)
	}
}

func TestSyncLDAPUserRejectsMultipleMatches(t *testing.T) {
	setupLDAPTest(t)
	ldapSearch = func(conf ergoldap.LdapConf, username string) []ergoldap.LdapUser {
		return []ergoldap.LdapUser{
			{Username: "alice", Email: "alice@example.com"},
			{Username: "alice2", Email: "alice2@example.com"},
		}
	}

	_, err := SyncLDAPUser("alice")
	if err == nil || !strings.Contains(err.Error(), "匹配到多个 LDAP 用户") {
		t.Fatalf("expected multiple matches error, got %v", err)
	}
}

func TestSearchLDAPUsersReturnsMultipleCandidates(t *testing.T) {
	setupLDAPTest(t)
	ldapSearch = func(conf ergoldap.LdapConf, username string) []ergoldap.LdapUser {
		if username != "alice" {
			t.Fatalf("unexpected search username: %s", username)
		}
		return []ergoldap.LdapUser{
			{
				Username:    "alice",
				Email:       "Alice@Example.com",
				DisplayName: "Alice Doe",
				Department:  "Engineering",
			},
			{
				Username:    "alice2",
				Email:       "alice2@example.com",
				DisplayName: "Alice Two",
				Department:  "Product",
			},
		}
	}

	candidates, err := SearchLDAPUsers("alice")
	if err != nil {
		t.Fatalf("search ldap users failed: %v", err)
	}
	if len(candidates) != 2 {
		t.Fatalf("expected two candidates, got %+v", candidates)
	}
	if candidates[0].Key != "alice@example.com" || candidates[0].Email != "alice@example.com" || candidates[0].DisplayName != "Alice Doe" {
		t.Fatalf("unexpected first candidate: %+v", candidates[0])
	}
	if candidates[1].Key != "alice2@example.com" || candidates[1].Department != "Product" {
		t.Fatalf("unexpected second candidate: %+v", candidates[1])
	}
}

func TestSyncLDAPUserByEmailSyncsSelectedUser(t *testing.T) {
	db := setupLDAPTest(t)
	var requested []string
	var attrs []string
	ldapSearch = func(conf ergoldap.LdapConf, username string) []ergoldap.LdapUser {
		requested = append(requested, username)
		attrs = append(attrs, conf.LdapUID)
		switch {
		case conf.LdapUID == "mail" && username == "alice@example.com":
			return []ergoldap.LdapUser{{
				Username:    "alice",
				Email:       "alice@example.com",
				DisplayName: "Alice Doe",
				Department:  "Engineering",
			}}
		default:
			return nil
		}
	}

	user, err := SyncLDAPUserByEmail("alice@example.com")
	if err != nil {
		t.Fatalf("sync ldap user failed: %v", err)
	}
	if user.Email != "alice@example.com" || user.Username != "Alice Doe" || user.Department != "Engineering" {
		t.Fatalf("unexpected synced user: %+v", user)
	}

	var count int64
	if err := db.Model(&model.User{}).Count(&count).Error; err != nil {
		t.Fatalf("count users failed: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected one user, got %d", count)
	}
	if strings.Join(requested, ",") != "alice@example.com,alice@example.com" {
		t.Fatalf("unexpected search sequence: %v", requested)
	}
	if strings.Join(attrs, ",") != "sAMAccountName,mail" {
		t.Fatalf("unexpected search attrs: %v", attrs)
	}
}

func TestSyncLDAPUserByEmailRequiresEmail(t *testing.T) {
	setupLDAPTest(t)
	ldapSearch = func(conf ergoldap.LdapConf, username string) []ergoldap.LdapUser {
		t.Fatalf("ldap search should not be called")
		return nil
	}

	_, err := SyncLDAPUserByEmail(" ")
	if err == nil || !strings.Contains(err.Error(), "请选择一个要同步的 LDAP 用户") {
		t.Fatalf("expected selected user error, got %v", err)
	}
}
