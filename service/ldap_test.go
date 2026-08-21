package service

import (
	"fmt"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/system_setting"
	ergoldap "github.com/ergoapi/ldap"
	"github.com/glebarez/sqlite"
	goldap "github.com/go-ldap/ldap/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupLDAPTest(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:ldap-%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.User{}, &model.Log{}))
	previousDB, previousLogDB := model.DB, model.LOG_DB
	previousRegister, previousPoolEnabled := common.RegisterEnabled, common.QuotaPoolEnabled
	previousSecret := common.SessionSecret
	previousSettings := *system_setting.GetLDAPSettings()
	previousRequest, previousSearch := ldapRequest, ldapSearch
	model.DB, model.LOG_DB = db, db
	common.RegisterEnabled, common.QuotaPoolEnabled = true, false
	common.SessionSecret = "ldap-test-secret"
	*system_setting.GetLDAPSettings() = system_setting.LDAPSettings{Enabled: true, URL: "ldap://ldap.example.com:389", BaseDN: "ou=users,dc=example,dc=com", UID: "uid", Scope: 3, ConnectionTimeout: 30}
	t.Cleanup(func() {
		model.DB, model.LOG_DB = previousDB, previousLogDB
		common.RegisterEnabled, common.QuotaPoolEnabled = previousRegister, previousPoolEnabled
		common.SessionSecret = previousSecret
		*system_setting.GetLDAPSettings() = previousSettings
		ldapRequest, ldapSearch = previousRequest, previousSearch
	})
	return db
}

func ldapTestSearchResult(username, email string) *goldap.SearchResult {
	return &goldap.SearchResult{Entries: []*goldap.Entry{{
		DN: "uid=" + username + ",ou=users,dc=example,dc=com",
		Attributes: []*goldap.EntryAttribute{
			{Name: "uid", Values: []string{username}},
			{Name: "mail", Values: []string{email}},
			{Name: "displayName", Values: []string{"LDAP Alice"}},
			{Name: "department", Values: []string{"Engineering"}},
		},
	}}}
}

func TestLoginWithLDAPReusesExistingUserByNormalizedEmail(t *testing.T) {
	db := setupLDAPTest(t)
	existing := model.User{Username: "local-alice", Password: "password", AffCode: "ldap-existing-aff", Email: "alice@example.com", Role: common.RoleCommonUser, Status: common.UserStatusEnabled}
	require.NoError(t, db.Create(&existing).Error)
	ldapRequest = func(_ ergoldap.LdapConf, username, password string) (*goldap.SearchResult, error) {
		assert.Equal(t, "alice", username)
		assert.Equal(t, "secret", password)
		return ldapTestSearchResult("alice", "Alice@Example.com"), nil
	}

	user, err := LoginWithLDAP("alice", "secret")

	require.NoError(t, err)
	assert.Equal(t, existing.Id, user.Id)
	assert.Equal(t, "local-alice", user.Username)
	assert.Equal(t, "LDAP Alice", user.DisplayName)
	assert.Equal(t, "alice@example.com", user.LDAPId)
	assert.Equal(t, "Engineering", user.Department)
}

func TestSearchAndSyncLDAPCandidateRejectsTampering(t *testing.T) {
	setupLDAPTest(t)
	ldapSearch = func(_ ergoldap.LdapConf, _ string) []ergoldap.LdapUser {
		return []ergoldap.LdapUser{{Username: "alice", Email: "alice@example.com", DisplayName: "Alice", Department: "Engineering"}}
	}
	candidates, err := SearchLDAPUsers("alice")
	require.NoError(t, err)
	require.Len(t, candidates, 1)
	assert.NotEmpty(t, candidates[0].Signature)

	candidates[0].Email = "attacker@example.com"
	_, err = SyncLDAPCandidate(candidates[0])
	require.ErrorIs(t, err, ErrLDAPCandidateInvalid)
}

func TestSyncLDAPCandidateSupportsIdentityWithoutEmail(t *testing.T) {
	setupLDAPTest(t)
	candidate := LDAPSyncCandidate{Key: "lee", Username: "lee", DisplayName: "Lee User", Department: "Engineering", LDAPId: "lee"}
	candidate.Signature = signLDAPSyncCandidate(candidate)

	user, err := SyncLDAPCandidate(candidate)

	require.NoError(t, err)
	assert.Equal(t, "", user.Email)
	assert.Equal(t, "lee", user.LDAPId)
	assert.Equal(t, "Lee User", user.Username)
}
