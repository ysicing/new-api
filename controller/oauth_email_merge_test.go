package controller

import (
	"context"
	"fmt"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/oauth"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
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
	dsn := fmt.Sprintf("file:oauth-email-%s?mode=memory&cache=shared", t.Name())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.User{}, &model.UserOAuthBinding{}))
	previousDB, previousRegister, previousRedis := model.DB, common.RegisterEnabled, common.RedisEnabled
	model.DB, common.RegisterEnabled, common.RedisEnabled = db, true, false
	t.Cleanup(func() {
		model.DB, common.RegisterEnabled, common.RedisEnabled = previousDB, previousRegister, previousRedis
	})
	return db
}

func TestFindOrCreateOAuthUserMergesOnlyVerifiedEmail(t *testing.T) {
	db := setupOAuthEmailMergeTest(t)
	existing := model.User{Username: "existing-oauth", Password: "password", AffCode: "oauth-aff", Email: "alice@example.com", Role: common.RoleCommonUser, Status: common.UserStatusEnabled}
	require.NoError(t, db.Create(&existing).Error)
	provider := verifiedEmailTestProvider{}

	user, err := findOrCreateOAuthUser(nil, provider, &oauth.OAuthUser{ProviderUserID: "provider-1", Email: "Alice@Example.com", EmailVerified: true}, "")

	require.NoError(t, err)
	assert.Equal(t, existing.Id, user.Id)
	require.NoError(t, db.First(&existing, existing.Id).Error)
	assert.Equal(t, "provider-1", existing.GitHubId)

	_, err = findOrCreateOAuthUser(nil, provider, &oauth.OAuthUser{ProviderUserID: "provider-2", Email: "alice@example.com", EmailVerified: false}, "")
	var conflict *OAuthEmailAlreadyTakenError
	assert.ErrorAs(t, err, &conflict)
}

func TestFindOrCreateOAuthUserNotifiesOnlyNewRegistration(t *testing.T) {
	setupOAuthEmailMergeTest(t)
	provider := verifiedEmailTestProvider{}
	previousNotify := notifyNewSelfRegisteredUser
	notifiedUserIds := make([]int, 0, 1)
	notifyNewSelfRegisteredUser = func(userId int) { notifiedUserIds = append(notifiedUserIds, userId) }
	t.Cleanup(func() { notifyNewSelfRegisteredUser = previousNotify })

	created, err := findOrCreateOAuthUser(nil, provider, &oauth.OAuthUser{
		ProviderUserID: "new-provider-user", Username: "new-oauth-user",
		Email: "new-oauth-user@example.com", EmailVerified: true,
	}, "")
	require.NoError(t, err)
	assert.Equal(t, []int{created.Id}, notifiedUserIds)

	existing, err := findOrCreateOAuthUser(nil, provider, &oauth.OAuthUser{
		ProviderUserID: "new-provider-user", Email: "new-oauth-user@example.com", EmailVerified: true,
	}, "")
	require.NoError(t, err)
	assert.Equal(t, created.Id, existing.Id)
	assert.Equal(t, []int{created.Id}, notifiedUserIds)
}
