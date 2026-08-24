package controller

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/oauth"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDingTalkOAuthMergesExistingLDAPUserByVerifiedEmail(t *testing.T) {
	db := setupOAuthEmailMergeTest(t)
	existing := model.User{
		Username: "ldap-alice", Password: "password", AffCode: "dingtalk-email-aff",
		Email: "alice@example.com", LDAPId: "alice@example.com",
		Role: common.RoleCommonUser, Status: common.UserStatusEnabled,
	}
	require.NoError(t, db.Create(&existing).Error)

	user, err := findOrCreateOAuthUser(nil, &oauth.DingTalkProvider{}, &oauth.OAuthUser{
		ProviderUserID: "union-alice", Email: "Alice@Example.com", EmailVerified: true,
		RequireEmailForRegistration: true,
	}, "")
	require.NoError(t, err)
	assert.Equal(t, existing.Id, user.Id)
	require.NoError(t, db.First(&existing, existing.Id).Error)
	assert.Equal(t, "union-alice", existing.DingTalkId)
}

func TestDingTalkOAuthRequiresEmailOnlyForFirstLogin(t *testing.T) {
	db := setupOAuthEmailMergeTest(t)
	provider := &oauth.DingTalkProvider{}
	existing := model.User{
		Username: "bound-dingtalk", Password: "password", AffCode: "dingtalk-bound-aff",
		DingTalkId: "union-bound", Role: common.RoleCommonUser, Status: common.UserStatusEnabled,
	}
	require.NoError(t, db.Create(&existing).Error)

	user, err := findOrCreateOAuthUser(nil, provider, &oauth.OAuthUser{
		ProviderUserID: "union-bound", RequireEmailForRegistration: true,
	}, "")
	require.NoError(t, err)
	assert.Equal(t, existing.Id, user.Id)

	_, err = findOrCreateOAuthUser(nil, provider, &oauth.OAuthUser{
		ProviderUserID: "union-new", RequireEmailForRegistration: true,
	}, "")
	var emailRequired *OAuthEmailRequiredError
	require.ErrorAs(t, err, &emailRequired)
}
