package controller

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/oauth"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDingTalkOAuthMergesVerifiedEmailWithExistingLDAPUser(t *testing.T) {
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
	require.NotNil(t, user)
	assert.Equal(t, existing.Id, user.Id)
	require.NoError(t, db.First(&existing, existing.Id).Error)
	assert.Equal(t, "union-alice", existing.DingTalkId)
	assert.Equal(t, "alice@example.com", existing.LDAPId)
	var claim model.ExternalIdentityClaim
	require.NoError(t, db.Where("provider = ? AND subject = ?", model.ExternalIdentityProviderDingTalk, "union-alice").First(&claim).Error)
	assert.Equal(t, existing.Id, claim.UserId)
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

func TestDingTalkOAuthEmailMergePreservesIdentityClaims(t *testing.T) {
	for _, conflict := range []string{"subject", "provider slot"} {
		t.Run(conflict, func(t *testing.T) {
			db := setupOAuthEmailMergeTest(t)
			existing := model.User{
				Username: "local-alice", AffCode: "dingtalk-conflict-aff", Email: "alice@example.com",
				Role: common.RoleCommonUser, Status: common.UserStatusEnabled,
			}
			require.NoError(t, db.Create(&existing).Error)
			claim := model.ExternalIdentityClaim{
				Provider: model.ExternalIdentityProviderDingTalk, Subject: "original-union", UserId: existing.Id,
			}
			if conflict == "subject" {
				other := model.User{
					Username: "other-owner", AffCode: "other-owner-aff",
					Role: common.RoleCommonUser, Status: common.UserStatusEnabled,
				}
				require.NoError(t, db.Create(&other).Error)
				claim.Subject, claim.UserId = "union-alice", other.Id
			}
			require.NoError(t, db.Create(&claim).Error)
			user, _, err := findOrCreateOAuthUser(nil, &oauth.DingTalkProvider{}, &oauth.OAuthUser{
				ProviderUserID: "union-alice", Email: "alice@example.com", EmailVerified: true,
				RequireEmailForRegistration: true,
			}, nil, "")
			require.ErrorIs(t, err, model.ErrDingTalkIdentityConflict)
			assert.Nil(t, user)
			require.NoError(t, db.First(&existing, existing.Id).Error)
			assert.Empty(t, existing.DingTalkId)
			var persisted model.ExternalIdentityClaim
			require.NoError(t, db.First(&persisted, claim.Id).Error)
			assert.Equal(t, claim.UserId, persisted.UserId)
			assert.Equal(t, claim.Subject, persisted.Subject)
		})
	}
}
