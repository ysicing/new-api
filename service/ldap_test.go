package service

import (
	"crypto/tls"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/system_setting"
	ergoldap "github.com/ergoapi/ldap"
	"github.com/glebarez/sqlite"
	ber "github.com/go-asn1-ber/asn1-ber"
	goldap "github.com/go-ldap/ldap/v3"
	mysqlDriver "github.com/go-sql-driver/mysql"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func setupLDAPTest(t *testing.T) *gorm.DB {
	t.Helper()
	dialect := os.Getenv("TEST_LDAP_DIALECT")
	if dialect == "" {
		dialect = "sqlite"
	}
	require.Contains(t, []string{"sqlite", "mysql", "postgres"}, dialect)
	dsn := os.Getenv("TEST_" + strings.ToUpper(dialect) + "_DSN")
	var admin *gorm.DB
	if dialect != "sqlite" {
		require.NotEmpty(t, dsn)
		driver := mysql.Open(dsn)
		if dialect == "postgres" {
			driver = postgres.New(postgres.Config{DSN: dsn, PreferSimpleProtocol: true})
		}
		var err error
		admin, err = gorm.Open(driver, &gorm.Config{})
		require.NoError(t, err)
		connection, err := admin.DB()
		require.NoError(t, err)
		connection.SetMaxOpenConns(1)
		t.Cleanup(func() { assert.NoError(t, connection.Close()) })
	}
	databases := make([]*gorm.DB, 0, 2)
	for _, purpose := range []string{"main", "log"} {
		namespace := "ldap_" + common.GetUUID() + "_" + purpose
		var driver gorm.Dialector
		switch dialect {
		case "sqlite":
			driver = sqlite.Open(filepath.Join(t.TempDir(), purpose+".db"))
		case "mysql":
			require.NoError(t, admin.Exec("CREATE DATABASE ? CHARACTER SET utf8mb4", clause.Table{Name: namespace}).Error)
			t.Cleanup(func() { assert.NoError(t, admin.Exec("DROP DATABASE ?", clause.Table{Name: namespace}).Error) })
			config, err := mysqlDriver.ParseDSN(dsn)
			require.NoError(t, err)
			config.DBName = namespace
			driver = mysql.Open(config.FormatDSN())
		case "postgres":
			require.NoError(t, admin.Exec("CREATE SCHEMA ?", clause.Table{Name: namespace}).Error)
			t.Cleanup(func() { assert.NoError(t, admin.Exec("DROP SCHEMA ? CASCADE", clause.Table{Name: namespace}).Error) })
			driver = postgres.New(postgres.Config{DSN: dsn, PreferSimpleProtocol: true})
		}
		db, err := gorm.Open(driver, &gorm.Config{})
		require.NoError(t, err)
		connection, err := db.DB()
		require.NoError(t, err)
		connection.SetMaxOpenConns(1)
		connection.SetMaxIdleConns(1)
		t.Cleanup(func() { assert.NoError(t, connection.Close()) })
		if dialect == "postgres" {
			require.NoError(t, db.Exec("SET search_path TO ?", clause.Table{Name: namespace}).Error)
		}
		versionSQL := "SELECT version()"
		if dialect == "sqlite" {
			versionSQL = "SELECT sqlite_version()"
		}
		var version string
		require.NoError(t, db.Raw(versionSQL).Scan(&version).Error)
		t.Logf("%s database: %s %s", purpose, dialect, version)
		databases = append(databases, db)
	}
	db, logDB := databases[0], databases[1]
	require.NoError(t, db.AutoMigrate(&model.User{}, &model.ExternalIdentityClaim{}, &model.DingTalkNotification{}))
	require.NoError(t, logDB.AutoMigrate(&model.Log{}))
	previousDB, previousLogDB := model.DB, model.LOG_DB
	previousRegister, previousPoolEnabled := common.RegisterEnabled, common.QuotaPoolEnabled
	previousSecret := common.SessionSecret
	previousSettings := *system_setting.GetLDAPSettings()
	previousRequest, previousSearch := ldapRequest, ldapSearch
	previousType, previousLogType, previousRedis := common.MainDatabaseType(), common.LogDatabaseType(), common.RedisEnabled
	model.DB, model.LOG_DB = db, logDB
	common.SetDatabaseTypes(common.DatabaseType(dialect), common.DatabaseType(dialect))
	common.RedisEnabled = false
	common.RegisterEnabled, common.QuotaPoolEnabled = true, false
	common.SessionSecret = "ldap-test-secret"
	*system_setting.GetLDAPSettings() = system_setting.LDAPSettings{Enabled: true, URL: "ldap://ldap.example.com:389", BaseDN: "ou=users,dc=example,dc=com", UID: "uid", Scope: 3, ConnectionTimeout: 30}
	t.Cleanup(func() {
		model.DB, model.LOG_DB = previousDB, previousLogDB
		common.RegisterEnabled, common.QuotaPoolEnabled = previousRegister, previousPoolEnabled
		common.SessionSecret = previousSecret
		*system_setting.GetLDAPSettings() = previousSettings
		ldapRequest, ldapSearch = previousRequest, previousSearch
		common.SetDatabaseTypes(previousType, previousLogType)
		common.RedisEnabled = previousRedis
	})
	return db
}

func TestLDAPIdentityRejectsInactiveOrConflictingAccounts(t *testing.T) {
	for _, scenario := range []string{"disabled", "deleted", "different_identity", "ambiguous_email", "ambiguous_identity"} {
		t.Run(scenario, func(t *testing.T) {
			db := setupLDAPTest(t)
			existing := model.User{
				Username: "local-owner", DisplayName: "Unchanged", Email: "alice@example.com",
				Role: common.RoleCommonUser, Status: common.UserStatusEnabled, AffCode: "ldap-owner",
			}
			switch scenario {
			case "disabled":
				existing.Status = common.UserStatusDisabled
			case "different_identity":
				existing.LDAPId = "different-directory-identity"
			case "ambiguous_identity":
				existing.LDAPId = "alice@example.com"
			}
			require.NoError(t, db.Create(&existing).Error)
			if scenario == "deleted" {
				require.NoError(t, db.Delete(&existing).Error)
			}
			if strings.HasPrefix(scenario, "ambiguous") {
				duplicate := model.User{
					Username: "second-owner", Role: common.RoleCommonUser, Status: common.UserStatusEnabled, AffCode: "ldap-second",
				}
				if scenario == "ambiguous_email" {
					duplicate.Email = "ALICE@example.com"
				} else {
					duplicate.LDAPId = existing.LDAPId
				}
				require.NoError(t, db.Create(&duplicate).Error)
			}
			ldapRequest = func(ergoldap.LdapConf, string, string) (*goldap.SearchResult, error) {
				return ldapTestSearchResult("alice", "alice@example.com"), nil
			}
			user, err := LoginWithLDAP("alice", "secret")
			require.Error(t, err)
			assert.Nil(t, user)
			require.NoError(t, db.Unscoped().First(&existing, existing.Id).Error)
			assert.Equal(t, "Unchanged", existing.DisplayName)
			var claims int64
			require.NoError(t, db.Model(&model.ExternalIdentityClaim{}).Count(&claims).Error)
			assert.Zero(t, claims)
		})
	}
}

func TestLDAPCandidateExpiresAndCannotExtendItsSignature(t *testing.T) {
	setupLDAPTest(t)
	candidate := LDAPSyncCandidate{
		Username: "alice", LDAPId: "alice", ExpiresAt: time.Now().Add(-time.Second).Unix(),
	}
	candidate.Signature = signLDAPSyncCandidate(candidate)
	_, err := SyncLDAPCandidate(candidate)
	assert.ErrorIs(t, err, ErrLDAPCandidateInvalid)
	candidate.ExpiresAt = time.Now().Add(time.Minute).Unix()
	_, err = SyncLDAPCandidate(candidate)
	assert.ErrorIs(t, err, ErrLDAPCandidateInvalid)
}

func TestLDAPDirectoryPreservesLegacyTransport(t *testing.T) {
	previousTimeout := goldap.DefaultTimeout
	t.Cleanup(func() { goldap.DefaultTimeout = previousTimeout })
	for _, scenario := range []struct {
		name, prefix string
		encrypted    bool
	}{
		{name: "ldap", prefix: "ldap://"},
		{name: "hostname"},
		{name: "ldaps", prefix: "ldaps://", encrypted: true},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			listener, err := net.Listen("tcp", "127.0.0.1:0")
			require.NoError(t, err)
			if scenario.encrypted {
				certificateServer := httptest.NewTLSServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
				listener = tls.NewListener(listener, certificateServer.TLS.Clone())
				certificateServer.Close()
			}
			done := make(chan struct{})
			go func() {
				defer close(done)
				for {
					connection, err := listener.Accept()
					if errors.Is(err, net.ErrClosed) {
						return
					}
					if !assert.NoError(t, err) {
						return
					}
					serveLDAPTestConnection(t, connection)
				}
			}()
			t.Cleanup(func() {
				assert.NoError(t, listener.Close())
				<-done
			})
			config := ergoldap.LdapConf{
				LdapURL:    scenario.prefix + strings.Replace(listener.Addr().String(), "127.0.0.1:", "localhost:", 1),
				LdapBaseDn: "ou=users,dc=example,dc=com",
				LdapUID:    "uid", LdapScope: 3, LdapConnectionTimeout: 2,
			}
			result, err := ldapRequest(config, "alice", "directory-secret")
			require.NoError(t, err)
			require.Len(t, result.Entries, 1)
			assert.Equal(t, "alice", result.Entries[0].GetAttributeValue("uid"))
			users := ldapSearch(config, "ali")
			require.Len(t, users, 1)
			assert.Equal(t, "alice", users[0].Username)
			assert.Equal(t, "alice@example.com", users[0].Email)
			_, err = ldapRequest(config, "alice", "incorrect-password")
			require.Error(t, err)
		})
	}
}

func serveLDAPTestConnection(t *testing.T, connection net.Conn) {
	t.Helper()
	defer connection.Close()
	if !assert.NoError(t, connection.SetDeadline(time.Now().Add(5*time.Second))) {
		return
	}
	for {
		request, err := ber.ReadPacket(connection)
		if errors.Is(err, io.EOF) {
			return
		}
		if !assert.NoError(t, err) || !assert.Len(t, request.Children, 2) {
			return
		}
		operation := request.Children[1]
		responseTag, resultCode := ber.Tag(goldap.ApplicationExtendedResponse), int64(goldap.LDAPResultProtocolError)
		switch operation.Tag {
		case goldap.ApplicationUnbindRequest:
			return
		case goldap.ApplicationBindRequest:
			responseTag, resultCode = goldap.ApplicationBindResponse, goldap.LDAPResultInvalidCredentials
			if operation.Children[2].Data.String() == "directory-secret" {
				resultCode = goldap.LDAPResultSuccess
			}
		case goldap.ApplicationSearchRequest:
			entry := ber.Encode(ber.ClassApplication, ber.TypeConstructed, goldap.ApplicationSearchResultEntry, nil, "")
			entry.AppendChild(ber.NewString(ber.ClassUniversal, ber.TypePrimitive, ber.TagOctetString, "uid=alice,ou=users,dc=example,dc=com", ""))
			attributes := ber.Encode(ber.ClassUniversal, ber.TypeConstructed, ber.TagSequence, nil, "")
			for _, attribute := range []struct{ name, value string }{{"uid", "alice"}, {"mail", "alice@example.com"}} {
				encoded := ber.NewSequence("")
				encoded.AppendChild(ber.NewString(ber.ClassUniversal, ber.TypePrimitive, ber.TagOctetString, attribute.name, ""))
				values := ber.Encode(ber.ClassUniversal, ber.TypeConstructed, ber.TagSet, nil, "")
				values.AppendChild(ber.NewString(ber.ClassUniversal, ber.TypePrimitive, ber.TagOctetString, attribute.value, ""))
				encoded.AppendChild(values)
				attributes.AppendChild(encoded)
			}
			entry.AppendChild(attributes)
			response := ber.NewSequence("")
			response.AppendChild(request.Children[0])
			response.AppendChild(entry)
			if _, err := connection.Write(response.Bytes()); !assert.NoError(t, err) {
				return
			}
			responseTag, resultCode = goldap.ApplicationSearchResultDone, goldap.LDAPResultSuccess
		}
		result := ber.Encode(ber.ClassApplication, ber.TypeConstructed, responseTag, nil, "")
		result.AppendChild(ber.NewInteger(ber.ClassUniversal, ber.TypePrimitive, ber.TagEnumerated, resultCode, ""))
		result.AppendChild(ber.NewString(ber.ClassUniversal, ber.TypePrimitive, ber.TagOctetString, "", ""))
		result.AppendChild(ber.NewString(ber.ClassUniversal, ber.TypePrimitive, ber.TagOctetString, "", ""))
		response := ber.NewSequence("")
		response.AppendChild(request.Children[0])
		response.AppendChild(result)
		if _, err := connection.Write(response.Bytes()); !assert.NoError(t, err) {
			return
		}
	}
}

func TestLDAPConfigurationRejectsInvalidFilters(t *testing.T) {
	setupLDAPTest(t)
	settings := system_setting.GetLDAPSettings()
	settings.UID = "uid)(objectClass=*"
	_, err := buildLDAPConfig()
	require.Error(t, err)
	settings.UID = "uid"
	settings.Filter = "(objectClass="
	_, err = buildLDAPConfig()
	require.Error(t, err)
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

func TestLoginWithLDAPReusesExistingBoundIdentity(t *testing.T) {
	db := setupLDAPTest(t)
	existing := model.User{Username: "local-alice", Password: "password", AffCode: "ldap-existing-aff", Email: "alice@example.com", LDAPId: "alice@example.com", Role: common.RoleCommonUser, Status: common.UserStatusEnabled}
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

func TestLDAPMergesEmailDuringLoginAndSync(t *testing.T) {
	for _, scenario := range []string{"password", "passwordless", "other identity"} {
		t.Run(scenario, func(t *testing.T) {
			db := setupLDAPTest(t)
			existing := model.User{
				Username: "local-alice", Password: "password", DisplayName: "Unchanged",
				AffCode: "ldap-email-aff", Email: "alice@example.com",
				Role: common.RoleCommonUser, Status: common.UserStatusEnabled,
			}
			if scenario == "passwordless" {
				existing.Password = ""
			}
			if scenario == "other identity" {
				existing.LDAPId = "another-directory-identity"
			}
			require.NoError(t, db.Create(&existing).Error)
			before := existing
			ldapRequest = func(ergoldap.LdapConf, string, string) (*goldap.SearchResult, error) {
				return ldapTestSearchResult("alice", "Alice@Example.com"), nil
			}
			ldapSearch = func(ergoldap.LdapConf, string) []ergoldap.LdapUser {
				return []ergoldap.LdapUser{{Username: "alice", Email: "Alice@Example.com", DisplayName: "LDAP Alice"}}
			}
			user, err := LoginWithLDAP("alice", "secret")
			if scenario == "other identity" {
				require.Error(t, err)
				assert.Nil(t, user)
			} else {
				require.NoError(t, err)
				assert.Equal(t, existing.Id, user.Id)
			}
			candidates, err := SearchLDAPUsers("alice")
			require.NoError(t, err)
			require.Len(t, candidates, 1)
			user, err = SyncLDAPCandidate(candidates[0])
			if scenario == "other identity" {
				require.Error(t, err)
				assert.Nil(t, user)
			} else {
				require.NoError(t, err)
				assert.Equal(t, existing.Id, user.Id)
			}
			require.NoError(t, db.First(&existing, existing.Id).Error)
			assert.Equal(t, before.Password, existing.Password)
			if scenario == "other identity" {
				assert.Equal(t, before.LDAPId, existing.LDAPId)
				assert.Equal(t, before.DisplayName, existing.DisplayName)
			} else {
				assert.Equal(t, "alice@example.com", existing.LDAPId)
				assert.Equal(t, "LDAP Alice", existing.DisplayName)
			}
			var count int64
			require.NoError(t, db.Model(&model.User{}).Count(&count).Error)
			assert.EqualValues(t, 1, count)
			require.NoError(t, db.Model(&model.ExternalIdentityClaim{}).Count(&count).Error)
			if scenario == "other identity" {
				assert.Zero(t, count)
			} else {
				assert.EqualValues(t, 1, count)
			}
		})
	}
}

func TestLDAPUnbindReleasesClaimAtomicallyAndAllowsEmailRelinking(t *testing.T) {
	for _, scenario := range []string{"released", "release fails"} {
		t.Run(scenario, func(t *testing.T) {
			db := setupLDAPTest(t)
			existing := model.User{
				Username: "bound-alice", Password: "password", AffCode: "ldap-unbind-aff",
				Email: "alice@example.com", LDAPId: "alice@example.com",
				Role: common.RoleCommonUser, Status: common.UserStatusEnabled,
			}
			require.NoError(t, db.Create(&existing).Error)
			ldapRequest = func(ergoldap.LdapConf, string, string) (*goldap.SearchResult, error) {
				return ldapTestSearchResult("alice", "alice@example.com"), nil
			}
			user, err := LoginWithLDAP("alice", "secret")
			require.NoError(t, err)
			var claim model.ExternalIdentityClaim
			require.NoError(t, db.Where("provider = ? AND user_id = ?", model.ExternalIdentityProviderLDAP, existing.Id).First(&claim).Error)
			if scenario == "release fails" {
				failure := errors.New("claim release failed")
				const callback = "test:ldap-claim-release-failure"
				require.NoError(t, db.Callback().Delete().Before("gorm:delete").Register(callback, func(tx *gorm.DB) {
					if tx.Statement.Table == "external_identity_claims" {
						tx.AddError(failure)
					}
				}))
				t.Cleanup(func() { db.Callback().Delete().Remove(callback) })
				require.ErrorIs(t, user.ClearBinding("ldap"), failure)
				require.NoError(t, db.First(&existing, existing.Id).Error)
				assert.Equal(t, "alice@example.com", existing.LDAPId)
				var persisted model.ExternalIdentityClaim
				require.NoError(t, db.First(&persisted, claim.Id).Error)
				assert.Equal(t, claim.UserId, persisted.UserId)
				return
			}
			require.NoError(t, user.ClearBinding("ldap"))
			assert.Empty(t, user.LDAPId)
			var count int64
			require.NoError(t, db.Model(&model.ExternalIdentityClaim{}).Where("provider = ? AND user_id = ?", model.ExternalIdentityProviderLDAP, user.Id).Count(&count).Error)
			assert.Zero(t, count)
			relinked, err := LoginWithLDAP("alice", "secret")
			require.NoError(t, err)
			require.NotNil(t, relinked)
			assert.Equal(t, existing.Id, relinked.Id)
			require.NoError(t, relinked.ClearBinding("ldap"))
			other := model.User{Username: "new-owner", AffCode: "ldap-new-owner", Role: common.RoleCommonUser, Status: common.UserStatusEnabled}
			require.NoError(t, db.Create(&other).Error)
			require.NoError(t, db.Transaction(func(tx *gorm.DB) error {
				return model.ClaimExternalIdentityWithTx(tx, model.ExternalIdentityProviderLDAP, claim.Subject, other.Id)
			}))
		})
	}
}

func TestLDAPEmailMergeReusesAccountWithRegistrationDisabled(t *testing.T) {
	db := setupLDAPTest(t)
	existing := model.User{
		Username: "local-alice", AffCode: "ldap-closed-aff", Email: "alice@example.com",
		Quota: 1234, UsedQuota: 56, Role: common.RoleCommonUser, Status: common.UserStatusEnabled,
	}
	require.NoError(t, db.Create(&existing).Error)
	common.RegisterEnabled = false
	ldapRequest = func(ergoldap.LdapConf, string, string) (*goldap.SearchResult, error) {
		return ldapTestSearchResult("alice", "Alice@Example.com"), nil
	}
	user, err := LoginWithLDAP("alice", "secret")
	require.NoError(t, err)
	assert.Equal(t, existing.Id, user.Id)
	assert.Equal(t, existing.Quota, user.Quota)
	assert.Equal(t, existing.UsedQuota, user.UsedQuota)
	var registrations int64
	require.NoError(t, db.Model(&model.DingTalkNotification{}).Count(&registrations).Error)
	assert.Zero(t, registrations)
}

func TestLDAPEmailMergeRequiresSuccessfulAuthentication(t *testing.T) {
	db := setupLDAPTest(t)
	existing := model.User{
		Username: "local-alice", AffCode: "ldap-auth-aff", Email: "alice@example.com",
		Role: common.RoleCommonUser, Status: common.UserStatusEnabled,
	}
	require.NoError(t, db.Create(&existing).Error)
	ldapRequest = func(ergoldap.LdapConf, string, string) (*goldap.SearchResult, error) {
		return nil, errors.New("invalid credentials")
	}
	user, err := LoginWithLDAP("alice", "wrong-password")
	require.Error(t, err)
	assert.Nil(t, user)
	require.NoError(t, db.First(&existing, existing.Id).Error)
	assert.Empty(t, existing.LDAPId)
}

func TestLDAPEmailMergeRollsBackWhenIdentityAlreadyClaimed(t *testing.T) {
	db := setupLDAPTest(t)
	existing := model.User{
		Username: "local-alice", DisplayName: "Local Alice", Department: "Original",
		AffCode: "ldap-rollback-aff", Email: "alice@example.com",
		Role: common.RoleCommonUser, Status: common.UserStatusEnabled,
	}
	other := model.User{
		Username: "other-owner", AffCode: "ldap-other-aff",
		Role: common.RoleCommonUser, Status: common.UserStatusEnabled,
	}
	require.NoError(t, db.Create(&existing).Error)
	require.NoError(t, db.Create(&other).Error)
	require.NoError(t, db.Transaction(func(tx *gorm.DB) error {
		return model.ClaimLDAPIdentityWithTx(tx, "alice@example.com", other.Id)
	}))
	ldapRequest = func(ergoldap.LdapConf, string, string) (*goldap.SearchResult, error) {
		return ldapTestSearchResult("alice", "alice@example.com"), nil
	}
	user, err := LoginWithLDAP("alice", "secret")
	require.ErrorIs(t, err, model.ErrExternalIdentityAlreadyClaimed)
	assert.Nil(t, user)
	require.NoError(t, db.First(&existing, existing.Id).Error)
	assert.Empty(t, existing.LDAPId)
	assert.Equal(t, "Local Alice", existing.DisplayName)
	assert.Equal(t, "Original", existing.Department)
	var claim model.ExternalIdentityClaim
	require.NoError(t, db.Where("provider = ?", model.ExternalIdentityProviderLDAP).First(&claim).Error)
	assert.Equal(t, other.Id, claim.UserId)
}

func TestLoginWithLDAPNotifiesNewlyCreatedUser(t *testing.T) {
	db := setupLDAPTest(t)
	require.NoError(t, i18n.Init())
	previousLanguage := common.DefaultLanguage
	common.DefaultLanguage = i18n.LangZhCN
	t.Cleanup(func() { common.DefaultLanguage = previousLanguage })
	dingTalkSettings := system_setting.GetDingTalkSettings()
	previousDingTalkSettings := *dingTalkSettings
	previousRedisEnabled := common.RedisEnabled
	*dingTalkSettings = system_setting.DingTalkSettings{}
	common.RedisEnabled = false
	t.Cleanup(func() {
		*dingTalkSettings = previousDingTalkSettings
		common.RedisEnabled = previousRedisEnabled
	})
	ldapRequest = func(_ ergoldap.LdapConf, username, password string) (*goldap.SearchResult, error) {
		assert.Equal(t, "alice", username)
		assert.Equal(t, "secret", password)
		return ldapTestSearchResult("alice", "alice@example.com"), nil
	}

	user, err := LoginWithLDAP("alice", "secret")

	require.NoError(t, err)
	var record model.DingTalkNotification
	require.Eventually(t, func() bool {
		err := db.Where("event_type = ? AND user_id = ?", model.DingTalkNotificationEventUserRegistered, user.Id).
			First(&record).Error
		return err == nil && record.Status == model.DingTalkNotificationStatusSkipped
	}, time.Second, 10*time.Millisecond)
	assert.Equal(t, "欢迎使用 iCode", record.Title)
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
	candidate := LDAPSyncCandidate{Key: "lee", Username: "lee", DisplayName: "Lee User", Department: "Engineering", LDAPId: "lee", ExpiresAt: time.Now().Add(time.Minute).Unix()}
	candidate.Signature = signLDAPSyncCandidate(candidate)

	user, err := SyncLDAPCandidate(candidate)

	require.NoError(t, err)
	assert.Equal(t, "", user.Email)
	assert.Equal(t, "lee", user.LDAPId)
	assert.Equal(t, "Lee User", user.Username)
}
