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
	require.NoError(t, db.Model(&model.User{}).Where("id = ?", existing.Id).UpdateColumn("dingtalk_id", nil).Error)

	user, _, err := findOrCreateOAuthUser(nil, &oauth.DingTalkProvider{}, &oauth.OAuthUser{
		ProviderUserID: "union-alice", Email: "Alice@Example.com", EmailVerified: true,
		RequireEmailForRegistration: true,
	}, nil, "")
	require.NoError(t, err)
	assert.Equal(t, existing.Id, user.Id)
	require.NoError(t, db.First(&existing, existing.Id).Error)
	assert.Equal(t, "union-alice", existing.DingTalkId)
	var claim model.ExternalIdentityClaim
	require.NoError(t, db.Where("provider = ? AND subject = ?", model.ExternalIdentityProviderDingTalk, "union-alice").First(&claim).Error)
	assert.Equal(t, existing.Id, claim.UserId)
}

func TestDingTalkEmailMergeRejectsIdentityClaimOwnedByAnotherUser(t *testing.T) {
	db := setupOAuthEmailMergeTest(t)
	first := model.User{Username: "claim-owner", Email: "owner@example.com", AffCode: "claim-owner"}
	second := model.User{Username: "merge-target", Email: "target@example.com", AffCode: "merge-target"}
	require.NoError(t, db.Create(&first).Error)
	require.NoError(t, db.Create(&second).Error)
	require.NoError(t, db.Create(&model.ExternalIdentityClaim{
		Provider: model.ExternalIdentityProviderDingTalk, Subject: "claimed-union", UserId: first.Id,
	}).Error)
	_, _, err := findOrCreateOAuthUser(nil, &oauth.DingTalkProvider{}, &oauth.OAuthUser{
		ProviderUserID: "claimed-union", Email: second.Email, EmailVerified: true,
	}, nil, "")
	require.ErrorIs(t, err, model.ErrDingTalkIdentityConflict)
	require.NoError(t, db.First(&second, second.Id).Error)
	assert.Empty(t, second.DingTalkId)
}

func TestDingTalkOAuthRequiresEmailOnlyForFirstLogin(t *testing.T) {
	db := setupOAuthEmailMergeTest(t)
	provider := &oauth.DingTalkProvider{}
	existing := model.User{
		Username: "bound-dingtalk", Password: "password", AffCode: "dingtalk-bound-aff",
		DingTalkId: "union-bound", Role: common.RoleCommonUser, Status: common.UserStatusEnabled,
	}
	require.NoError(t, db.Create(&existing).Error)

	user, _, err := findOrCreateOAuthUser(nil, provider, &oauth.OAuthUser{
		ProviderUserID: "union-bound", RequireEmailForRegistration: true,
	}, nil, "")
	require.NoError(t, err)
	assert.Equal(t, existing.Id, user.Id)

	_, _, err = findOrCreateOAuthUser(nil, provider, &oauth.OAuthUser{
		ProviderUserID: "union-new", RequireEmailForRegistration: true,
	}, nil, "")
	var emailRequired *OAuthEmailRequiredError
	require.ErrorAs(t, err, &emailRequired)
}
