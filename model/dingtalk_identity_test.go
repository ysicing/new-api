package model

import (
	"errors"
	"fmt"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

type legacyIdentityMigrationUser struct {
	Id          int    `gorm:"primaryKey"`
	Username    string `gorm:"unique;index"`
	Password    string `gorm:"not null"`
	DisplayName string `gorm:"index"`
	Role        int    `gorm:"type:int;default:1"`
	Status      int    `gorm:"type:int;default:1"`
	Email       string `gorm:"index"`
	GitHubId    string `gorm:"column:github_id;index"`
	TelegramId  string `gorm:"column:telegram_id;index"`
	AffCode     string `gorm:"type:varchar(32);column:aff_code;uniqueIndex"`
	Quota       int    `gorm:"type:int;default:0"`
}

func (legacyIdentityMigrationUser) TableName() string { return "users" }

type legacyCustomIdentityMigrationUser struct {
	Legacy     legacyIdentityMigrationUser `gorm:"embedded"`
	Department string                      `gorm:"type:varchar(512);column:department;index"`
	LDAPId     string                      `gorm:"type:varchar(256);column:ldap_id;index"`
	DingTalkId string                      `gorm:"type:varchar(256);column:dingtalk_id;index"`
}

func (legacyCustomIdentityMigrationUser) TableName() string { return "users" }

func TestExternalIdentityMigrationDatabaseMatrix(t *testing.T) {
	for _, dialect := range []string{"sqlite", "mysql", "postgres"} {
		t.Run(dialect, func(t *testing.T) {
			db, logDB := setupOperationsDatabaseMatrix(t, dialect)
			for _, source := range []string{"fresh", "official_rc49", "custom_rc25"} {
				t.Run(source, func(t *testing.T) {
					require.NoError(t, db.Migrator().DropTable(&ExternalIdentityClaim{}, &User{}))
					var userID int
					if source == "official_rc49" {
						require.NoError(t, db.AutoMigrate(&legacyIdentityMigrationUser{}))
						legacy := legacyIdentityMigrationUser{
							Username: "legacy-owner", Password: "preserved-password-hash", DisplayName: "Legacy Owner",
							Role: common.RoleCommonUser, Status: common.UserStatusEnabled,
							Email: "owner@example.com", GitHubId: "original-github", TelegramId: "original-telegram",
							AffCode: "identity-legacy", Quota: 73,
						}
						require.NoError(t, db.Create(&legacy).Error)
						userID = legacy.Id
					} else if source == "custom_rc25" {
						require.NoError(t, db.AutoMigrate(&legacyCustomIdentityMigrationUser{}))
						legacy := legacyCustomIdentityMigrationUser{
							Legacy: legacyIdentityMigrationUser{
								Username: "legacy-owner", Password: "preserved-password-hash", DisplayName: "Legacy Owner",
								Role: common.RoleCommonUser, Status: common.UserStatusEnabled,
								Email: "owner@example.com", GitHubId: "original-github", TelegramId: "original-telegram",
								AffCode: "identity-legacy", Quota: 73,
							},
							LDAPId: "directory-owner", DingTalkId: "union-owner", Department: "Engineering",
						}
						require.NoError(t, db.Create(&legacy).Error)
						userID = legacy.Legacy.Id
					}
					for range 2 {
						require.NoError(t, db.AutoMigrate(&User{}, &ExternalIdentityClaim{}))
						require.NoError(t, InitializeExternalIdentityClaims())
					}
					assert.False(t, logDB.Migrator().HasTable(&ExternalIdentityClaim{}))
					if userID == 0 {
						var count int64
						require.NoError(t, db.Model(&User{}).Count(&count).Error)
						assert.Zero(t, count)
						return
					}
					var user User
					require.NoError(t, db.First(&user, userID).Error)
					assert.Equal(t, "preserved-password-hash", user.Password)
					assert.Equal(t, "owner@example.com", user.Email)
					assert.Equal(t, "original-github", user.GitHubId)
					assert.Equal(t, "original-telegram", user.TelegramId)
					assert.Equal(t, 73, user.Quota)
					expectedClaims := 1
					if source == "custom_rc25" {
						assert.Equal(t, "directory-owner", user.LDAPId)
						assert.Equal(t, "union-owner", user.DingTalkId)
						assert.Equal(t, "Engineering", user.Department)
						expectedClaims = 3
					} else {
						assert.Empty(t, user.LDAPId)
						assert.Empty(t, user.DingTalkId)
					}
					var claims []ExternalIdentityClaim
					require.NoError(t, db.Where("user_id = ?", userID).Find(&claims).Error)
					assert.Len(t, claims, expectedClaims)
					duplicate := User{Username: user.Username, AffCode: "duplicate-aff", Password: "password"}
					require.Error(t, db.Create(&duplicate).Error, "migration must preserve username uniqueness")
				})
			}
			for _, provider := range []string{ExternalIdentityProviderLDAP, ExternalIdentityProviderDingTalk} {
				t.Run(provider, func(t *testing.T) {
					require.NoError(t, db.Migrator().DropTable(&ExternalIdentityClaim{}, &User{}))
					require.NoError(t, db.AutoMigrate(&User{}, &ExternalIdentityClaim{}))
					first := newDingTalkBindingUser("identity-first")
					second := newDingTalkBindingUser("identity-second")
					first.Email, second.Email = "local@example.com", "other@example.com"
					column := "ldap_id"
					if provider == ExternalIdentityProviderDingTalk {
						column = "dingtalk_id"
					}
					require.NoError(t, db.Create(&first).Error)
					require.NoError(t, db.Create(&second).Error)
					require.NoError(t, db.Model(&first).Update(column, "legacy-subject").Error)
					require.NoError(t, db.Model(&second).Update(column, "legacy-subject").Error)
					require.NoError(t, db.Delete(&second).Error)
					require.ErrorIs(t, InitializeExternalIdentityClaims(), ErrExternalIdentityAlreadyClaimed, "deleted owners still reserve their identity")
					var count int64
					require.NoError(t, db.Model(&ExternalIdentityClaim{}).Count(&count).Error)
					assert.Zero(t, count, "ambiguous import must roll back every claim")
					require.NoError(t, db.Unscoped().Model(&User{}).Where("id = ?", second.Id).Update(column, "").Error)
					require.NoError(t, db.Unscoped().Model(&User{}).Where("id = ?", second.Id).Update("deleted_at", nil).Error)
					require.NoError(t, InitializeExternalIdentityClaims())
					require.NoError(t, InitializeExternalIdentityClaims())
					var imported ExternalIdentityClaim
					require.NoError(t, db.Where("provider = ?", provider).First(&imported).Error)
					assert.Equal(t, first.Id, imported.UserId)
					require.ErrorIs(t, db.Transaction(func(tx *gorm.DB) error {
						return ClaimExternalIdentityWithTx(tx, provider, imported.Subject, second.Id)
					}), ErrExternalIdentityAlreadyClaimed)
					require.ErrorIs(t, db.Transaction(func(tx *gorm.DB) error {
						return ClaimExternalIdentityWithTx(tx, provider, "replacement-subject", first.Id)
					}), ErrExternalIdentityAlreadyClaimed)
					if provider == ExternalIdentityProviderLDAP {
						matches, err := FindLDAPIdentityUsersWithTx(db, "legacy-subject")
						require.NoError(t, err)
						require.Len(t, matches, 1)
						assert.Equal(t, first.Id, matches[0].Id)
					} else {
						bound, err := BindDingTalkIdentity(first.Id, "legacy-subject")
						require.NoError(t, err)
						assert.False(t, bound)
						_, err = BindDingTalkIdentity(second.Id, "legacy-subject")
						require.ErrorIs(t, err, ErrDingTalkIdentityConflict)
					}
					failure := errors.New("claim deletion failed")
					const callback = "test:identity-claim-release-failure"
					require.NoError(t, db.Callback().Delete().Before("gorm:delete").Register(callback, func(tx *gorm.DB) {
						if tx.Statement.Table == "external_identity_claims" {
							tx.AddError(failure)
						}
					}))
					err := first.ClearBinding(provider)
					require.NoError(t, db.Callback().Delete().Remove(callback))
					require.ErrorIs(t, err, failure)
					var unchanged User
					require.NoError(t, db.First(&unchanged, first.Id).Error)
					if provider == ExternalIdentityProviderLDAP {
						assert.Equal(t, "legacy-subject", unchanged.LDAPId)
					} else {
						assert.Equal(t, "legacy-subject", unchanged.DingTalkId)
					}
					require.NoError(t, first.ClearBinding(provider))
					require.NoError(t, db.Model(&ExternalIdentityClaim{}).Where("provider = ?", provider).Count(&count).Error)
					assert.Zero(t, count)
					if provider == ExternalIdentityProviderLDAP {
						require.NoError(t, db.Transaction(func(tx *gorm.DB) error {
							if err := ClaimLDAPIdentityWithTx(tx, "legacy-subject", second.Id); err != nil {
								return err
							}
							return tx.Model(&User{}).Where("id = ?", second.Id).Update(column, "legacy-subject").Error
						}))
					} else {
						_, err := BindDingTalkIdentity(second.Id, "legacy-subject")
						require.NoError(t, err)
					}
					imported = ExternalIdentityClaim{}
					require.NoError(t, db.Where("provider = ?", provider).First(&imported).Error)
					assert.Equal(t, second.Id, imported.UserId)
					require.NoError(t, db.First(&first, first.Id).Error)
					assert.Equal(t, "local@example.com", first.Email)
					assert.Equal(t, "password", first.Password)
				})
			}
		})
	}
}

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
