package controller

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/oauth"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/system_setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

type verifiedEmailTestProvider struct{}

func (verifiedEmailTestProvider) GetName() string { return "VerifiedTest" }
func (verifiedEmailTestProvider) IsEnabled() bool { return true }
func (verifiedEmailTestProvider) ExchangeToken(context.Context, string, *gin.Context) (*oauth.OAuthToken, error) {
	return nil, nil
}
func (verifiedEmailTestProvider) GetUserInfo(context.Context, *oauth.OAuthToken) (*oauth.OAuthUser, error) {
	return nil, nil
}
func (verifiedEmailTestProvider) IsUserIDTaken(id string) bool {
	var count int64
	_ = model.DB.Model(&model.User{}).Where("github_id = ?", id).Count(&count).Error
	return count > 0
}
func (verifiedEmailTestProvider) FillUserByProviderID(user *model.User, id string) error {
	return model.DB.Where("github_id = ?", id).First(user).Error
}
func (verifiedEmailTestProvider) SetProviderUserID(user *model.User, id string) { user.GitHubId = id }
func (verifiedEmailTestProvider) GetProviderPrefix() string                     { return "github_" }
func (verifiedEmailTestProvider) ProviderUserIDColumn() string                  { return "github_id" }

func setupOAuthEmailMergeTest(t *testing.T) *gorm.DB {
	t.Helper()
	db := setupManageUserTestDB(t)
	require.NoError(t, db.AutoMigrate(&model.User{}, &model.UserOAuthBinding{}, &model.ExternalIdentityClaim{}))
	previousRegister, previousPool := common.RegisterEnabled, common.QuotaPoolEnabled
	previousNotify := notifyNewSelfRegisteredUser
	common.RegisterEnabled, common.QuotaPoolEnabled = true, false
	notifyNewSelfRegisteredUser = func(int) {}
	t.Cleanup(func() {
		common.RegisterEnabled, common.QuotaPoolEnabled = previousRegister, previousPool
		notifyNewSelfRegisteredUser = previousNotify
	})
	return db
}

func TestFindOrCreateOAuthUserMergesVerifiedEmail(t *testing.T) {
	for _, scenario := range []string{"password", "passwordless", "ldap", "other provider", "disabled", "deleted"} {
		t.Run(scenario, func(t *testing.T) {
			db := setupOAuthEmailMergeTest(t)
			existing := model.User{Username: "existing-oauth", Password: "password", DisplayName: "Unchanged", AffCode: "oauth-aff", Email: "alice@example.com", Role: common.RoleCommonUser, Status: common.UserStatusEnabled}
			switch scenario {
			case "passwordless":
				existing.Password = ""
			case "ldap":
				existing.LDAPId = "directory-alice"
			case "other provider":
				existing.GitHubId = "original-provider-id"
			case "disabled":
				existing.Status = common.UserStatusDisabled
			}
			require.NoError(t, db.Create(&existing).Error)
			if scenario == "deleted" {
				require.NoError(t, db.Delete(&existing).Error)
			}
			before := existing
			for _, verified := range []bool{false, true} {
				user, _, err := findOrCreateOAuthUser(nil, verifiedEmailTestProvider{}, &oauth.OAuthUser{
					ProviderUserID: "new-provider-id", Email: "Alice@Example.com", EmailVerified: verified,
				}, nil, "")
				if verified && (scenario == "password" || scenario == "passwordless" || scenario == "ldap") {
					require.NoError(t, err)
					require.NotNil(t, user)
					assert.Equal(t, existing.Id, user.Id)
					assert.Equal(t, "new-provider-id", user.GitHubId)
				} else {
					require.Error(t, err)
					assert.Nil(t, user)
				}
				require.NoError(t, db.Unscoped().First(&existing, existing.Id).Error)
				assert.Equal(t, before.Password, existing.Password)
				if verified && (scenario == "password" || scenario == "passwordless" || scenario == "ldap") {
					assert.Equal(t, "new-provider-id", existing.GitHubId)
				} else {
					assert.Equal(t, before.GitHubId, existing.GitHubId)
				}
				assert.Equal(t, before.LDAPId, existing.LDAPId)
				assert.Equal(t, before.DisplayName, existing.DisplayName)
			}
			var count int64
			require.NoError(t, db.Unscoped().Model(&model.User{}).Count(&count).Error)
			assert.EqualValues(t, 1, count)
			require.NoError(t, db.Model(&model.ExternalIdentityClaim{}).Count(&count).Error)
			assert.Zero(t, count)
			require.NoError(t, db.Model(&model.UserOAuthBinding{}).Count(&count).Error)
			assert.Zero(t, count)
		})
	}
}

func TestOAuthEmailMergeReusesAccountWithRegistrationDisabled(t *testing.T) {
	db := setupOAuthEmailMergeTest(t)
	existing := model.User{
		Username: "existing-local", Password: "password", AffCode: "merge-closed-aff",
		Email: "alice@example.com", Quota: 1234, UsedQuota: 56,
		Role: common.RoleCommonUser, Status: common.UserStatusEnabled,
	}
	require.NoError(t, db.Create(&existing).Error)
	common.RegisterEnabled = false
	notifyNewSelfRegisteredUser = func(int) { t.Error("email merge must not notify a new registration") }
	for range 2 {
		user, migration, err := findOrCreateOAuthUser(nil, verifiedEmailTestProvider{}, &oauth.OAuthUser{
			ProviderUserID: "merged-subject", Email: " Alice@Example.com ", EmailVerified: true,
		}, nil, "")
		require.NoError(t, err)
		require.NotNil(t, user)
		assert.Nil(t, migration)
		assert.Equal(t, existing.Id, user.Id)
		assert.Equal(t, existing.Quota, user.Quota)
		assert.Equal(t, existing.UsedQuota, user.UsedQuota)
		assert.Equal(t, existing.Username, user.Username)
	}
}

func TestOAuthEmailMergeRejectsAmbiguousEmail(t *testing.T) {
	db := setupOAuthEmailMergeTest(t)
	for index, email := range []string{"alice@example.com", "ALICE@example.com"} {
		user := model.User{
			Username: []string{"first-alice", "second-alice"}[index],
			AffCode:  []string{"first-alice-aff", "second-alice-aff"}[index],
			Email:    email, Role: common.RoleCommonUser, Status: common.UserStatusEnabled,
		}
		require.NoError(t, db.Create(&user).Error)
	}
	user, _, err := findOrCreateOAuthUser(nil, verifiedEmailTestProvider{}, &oauth.OAuthUser{
		ProviderUserID: "ambiguous-subject", Email: "alice@example.com", EmailVerified: true,
	}, nil, "")
	require.Error(t, err)
	assert.Nil(t, user)
	var bound int64
	require.NoError(t, db.Model(&model.User{}).Where("github_id <> ?", "").Count(&bound).Error)
	assert.Zero(t, bound)
}

func TestOAuthEmailMergeCustomProviderPreservesBinding(t *testing.T) {
	for _, existingSubject := range []string{"", "same-subject", "other-subject"} {
		t.Run("binding="+existingSubject, func(t *testing.T) {
			db := setupOAuthEmailMergeTest(t)
			existing := model.User{
				Username: "custom-alice", AffCode: "custom-alice-aff", Email: "alice@example.com",
				Role: common.RoleCommonUser, Status: common.UserStatusEnabled,
			}
			require.NoError(t, db.Create(&existing).Error)
			if existingSubject != "" {
				require.NoError(t, model.CreateUserOAuthBinding(&model.UserOAuthBinding{
					UserId: existing.Id, ProviderId: 42, ProviderUserId: existingSubject,
				}))
			}
			provider := oauth.NewGenericOAuthProvider(&model.CustomOAuthProvider{Id: 42, Name: "Enterprise", Slug: "enterprise"})
			user, _, err := findOrCreateOAuthUser(nil, provider, &oauth.OAuthUser{
				ProviderUserID: "same-subject", Email: "alice@example.com", EmailVerified: true,
			}, nil, "")
			if existingSubject == "other-subject" {
				require.Error(t, err)
				assert.Nil(t, user)
			} else {
				require.NoError(t, err)
				require.NotNil(t, user)
				assert.Equal(t, existing.Id, user.Id)
				existingSubject = "same-subject"
			}
			binding, err := model.GetUserOAuthBinding(existing.Id, 42)
			require.NoError(t, err)
			assert.Equal(t, existingSubject, binding.ProviderUserId)
		})
	}
}

func TestOAuthEmailMergePreservesLoginVerification(t *testing.T) {
	for _, factor := range []string{"2fa", "passkey"} {
		t.Run(factor, func(t *testing.T) {
			db := setupOAuthEmailMergeTest(t)
			existing := model.User{
				Username: "mfa-alice", AffCode: "mfa-alice-aff", Email: "alice@example.com",
				Role: common.RoleCommonUser, Status: common.UserStatusEnabled,
			}
			require.NoError(t, db.Create(&existing).Error)
			if factor == "2fa" {
				require.NoError(t, db.Create(&model.TwoFA{UserId: existing.Id, Secret: "JBSWY3DPEHPK3PXP", IsEnabled: true}).Error)
			} else {
				previous := *system_setting.GetPasskeySettings()
				system_setting.GetPasskeySettings().Enabled = true
				t.Cleanup(func() { *system_setting.GetPasskeySettings() = previous })
				newSecurityLoginPasskey(t, existing.Id)
			}
			response := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(response)
			c.Request = httptest.NewRequest(http.MethodGet, "/api/oauth/verified-test", nil)
			c.Set("login_method", "oauth")
			user, migration, err := findOrCreateOAuthUser(c, verifiedEmailTestProvider{}, &oauth.OAuthUser{
				ProviderUserID: "mfa-subject", Email: "alice@example.com", EmailVerified: true,
			}, nil, "")
			require.NoError(t, err)
			setupLogin(user, migration, c)
			var result struct {
				Success bool                   `json:"success"`
				Data    service.LoginChallenge `json:"data"`
			}
			require.NoError(t, common.Unmarshal(response.Body.Bytes(), &result))
			require.True(t, result.Success, response.Body.String())
			assert.True(t, result.Data.RequireVerification)
			assert.NotEmpty(t, result.Data.FlowToken)
			assert.Equal(t, []service.VerificationMethodOption{{Method: factor, Available: true}}, result.Data.Methods)
			assert.Empty(t, response.Header().Values("Set-Cookie"))
			var sessions int64
			require.NoError(t, db.Model(&model.UserSession{}).Where("user_id = ?", existing.Id).Count(&sessions).Error)
			assert.Zero(t, sessions)
			var audit model.AuditLog
			require.NoError(t, model.LOG_DB.Where("action = ? AND user_id = ?", "user.binding_bind", existing.Id).Last(&audit).Error)
			assert.True(t, audit.Success)
		})
	}
}

func TestFindOrCreateOAuthUserNotifiesOnlyNewRegistration(t *testing.T) {
	setupOAuthEmailMergeTest(t)
	provider := verifiedEmailTestProvider{}
	previousNotify := notifyNewSelfRegisteredUser
	notifiedUserIds := make([]int, 0, 1)
	notifyNewSelfRegisteredUser = func(userId int) { notifiedUserIds = append(notifiedUserIds, userId) }
	t.Cleanup(func() { notifyNewSelfRegisteredUser = previousNotify })

	created, _, err := findOrCreateOAuthUser(nil, provider, &oauth.OAuthUser{
		ProviderUserID: "new-provider-user", Username: "new-oauth-user",
		Email: "new-oauth-user@example.com", EmailVerified: true,
	}, nil, "")
	require.NoError(t, err)
	assert.Equal(t, []int{created.Id}, notifiedUserIds)

	existing, _, err := findOrCreateOAuthUser(nil, provider, &oauth.OAuthUser{
		ProviderUserID: "new-provider-user", Email: "new-oauth-user@example.com", EmailVerified: true,
	}, nil, "")
	require.NoError(t, err)
	assert.Equal(t, created.Id, existing.Id)
	assert.Equal(t, []int{created.Id}, notifiedUserIds)
}
