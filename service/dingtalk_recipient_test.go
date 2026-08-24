package service

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/system_setting"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolveDingTalkRecipientSkipsDirectoryForBoundUser(t *testing.T) {
	setupDingTalkNotificationServiceDB(t)
	user := model.User{
		Username: "bound", Email: "bound@example.com", DingTalkId: "union-bound",
		Password: "password", AffCode: "bound-aff", Status: common.UserStatusEnabled,
	}
	require.NoError(t, model.DB.Create(&user).Error)

	notifier := newDingTalkNotifier(http.DefaultClient, "http://unused")
	recipient, err := notifier.resolveRecipient(context.Background(), system_setting.DingTalkSettings{}, DingTalkRecipientRequest{UserId: user.Id, UserEmail: user.Email})

	require.NoError(t, err)
	assert.Equal(t, "bound", recipient.StaffUserId)
	assert.False(t, recipient.BoundNow)
}

func TestResolveDingTalkRecipientQueriesAndBindsUnionID(t *testing.T) {
	setupDingTalkNotificationServiceDB(t)
	user := model.User{
		Username: "alice", Email: "alice@example.com", Password: "password",
		AffCode: "alice-recipient-aff", Status: common.UserStatusEnabled,
	}
	require.NoError(t, model.DB.Create(&user).Error)
	var tokenRequests atomic.Int32
	var detailRequests atomic.Int32
	server := newDingTalkDirectoryServer(t, &tokenRequests, &detailRequests, "alice@example.com", "union-alice")
	t.Cleanup(server.Close)

	notifier := newDingTalkNotifier(server.Client(), server.URL)
	recipient, err := notifier.resolveRecipient(context.Background(), system_setting.DingTalkSettings{ClientId: "app-key", ClientSecret: "app-secret"}, DingTalkRecipientRequest{UserId: user.Id, UserEmail: user.Email})

	require.NoError(t, err)
	assert.Equal(t, "alice", recipient.StaffUserId)
	assert.True(t, recipient.BoundNow)
	assert.Equal(t, int32(1), tokenRequests.Load())
	assert.Equal(t, int32(1), detailRequests.Load())
	require.NoError(t, model.DB.First(&user, user.Id).Error)
	assert.Equal(t, "union-alice", user.DingTalkId)
}

func TestResolveDingTalkRecipientRejectsEmailMismatch(t *testing.T) {
	setupDingTalkNotificationServiceDB(t)
	user := model.User{
		Username: "mismatch", Email: "alice@example.com", Password: "password",
		AffCode: "mismatch-aff", Status: common.UserStatusEnabled,
	}
	require.NoError(t, model.DB.Create(&user).Error)
	server := newDingTalkDirectoryServer(t, nil, nil, "other@example.com", "union-other")
	t.Cleanup(server.Close)

	notifier := newDingTalkNotifier(server.Client(), server.URL)
	_, err := notifier.resolveRecipient(context.Background(), system_setting.DingTalkSettings{ClientId: "app-key", ClientSecret: "app-secret"}, DingTalkRecipientRequest{UserId: user.Id, UserEmail: user.Email})

	assert.ErrorIs(t, err, ErrDingTalkIdentityMismatch)
	require.NoError(t, model.DB.First(&user, user.Id).Error)
	assert.Empty(t, user.DingTalkId)
}

func TestResolveDingTalkRecipientRejectsUnionIDConflict(t *testing.T) {
	setupDingTalkNotificationServiceDB(t)
	owner := model.User{
		Username: "owner", Email: "owner@example.com", DingTalkId: "union-owned",
		Password: "password", AffCode: "owner-recipient-aff", Status: common.UserStatusEnabled,
	}
	target := model.User{
		Username: "target", Email: "target@example.com", Password: "password",
		AffCode: "target-recipient-aff", Status: common.UserStatusEnabled,
	}
	require.NoError(t, model.DB.Create(&owner).Error)
	require.NoError(t, model.DB.Create(&target).Error)
	server := newDingTalkDirectoryServer(t, nil, nil, target.Email, owner.DingTalkId)
	t.Cleanup(server.Close)

	notifier := newDingTalkNotifier(server.Client(), server.URL)
	_, err := notifier.resolveRecipient(context.Background(), system_setting.DingTalkSettings{ClientId: "app-key", ClientSecret: "app-secret"}, DingTalkRecipientRequest{UserId: target.Id, UserEmail: target.Email})

	assert.ErrorIs(t, err, model.ErrDingTalkIdentityConflict)
}

func newDingTalkDirectoryServer(
	t *testing.T,
	tokenRequests, detailRequests *atomic.Int32,
	email, unionId string,
) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/gettoken":
			if tokenRequests != nil {
				tokenRequests.Add(1)
			}
			assert.Equal(t, "app-key", r.URL.Query().Get("appkey"))
			assert.Equal(t, "app-secret", r.URL.Query().Get("appsecret"))
			_, _ = fmt.Fprint(w, `{"errcode":0,"access_token":"legacy-token","expires_in":7200}`)
		case "/topapi/v2/user/get":
			if detailRequests != nil {
				detailRequests.Add(1)
			}
			assert.Equal(t, "legacy-token", r.URL.Query().Get("access_token"))
			var payload struct {
				UserId string `json:"userid"`
			}
			require.NoError(t, common.DecodeJson(r.Body, &payload))
			_, _ = fmt.Fprintf(w, `{"errcode":0,"result":{"userid":%q,"unionid":%q,"email":%q}}`, payload.UserId, unionId, email)
		default:
			http.NotFound(w, r)
		}
	}))
}
