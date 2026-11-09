package service

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/system_setting"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSendDingTalkTestMessageBindsAndRecordsEveryDelivery(t *testing.T) {
	setupDingTalkNotificationServiceDB(t)
	user := model.User{
		Username: "alice", DisplayName: "Alice", Email: "alice@example.com",
		Password: "password", AffCode: "test-message-alice", Status: common.UserStatusEnabled,
	}
	require.NoError(t, model.DB.Create(&user).Error)
	settings := system_setting.GetDingTalkSettings()
	previousSettings := *settings
	*settings = system_setting.DingTalkSettings{ClientId: "app-key", ClientSecret: "app-secret"}
	t.Cleanup(func() { *settings = previousSettings })
	var detailRequests atomic.Int32
	server := newDingTalkTestServer(t, &detailRequests)
	t.Cleanup(server.Close)
	previousNotifier := defaultDingTalkNotifier
	defaultDingTalkNotifier = newDingTalkNotifier(server.Client(), server.URL)
	t.Cleanup(func() { defaultDingTalkNotifier = previousNotifier })

	first, err := SendDingTalkTestMessage(context.Background(), user.Id)
	require.NoError(t, err)
	assert.True(t, first.BoundNow)
	second, err := SendDingTalkTestMessage(context.Background(), user.Id)
	require.NoError(t, err)
	assert.False(t, second.BoundNow)
	assert.Equal(t, int32(1), detailRequests.Load())

	var records []model.DingTalkNotification
	require.NoError(t, model.DB.Order("id ASC").Find(&records).Error)
	require.Len(t, records, 2)
	for _, record := range records {
		assert.Equal(t, model.DingTalkNotificationEventTest, record.EventType)
		assert.Equal(t, model.DingTalkNotificationStatusSucceeded, record.Status)
		assert.Equal(t, "alice", record.Recipient)
		assert.True(t, strings.HasPrefix(record.DedupeKey, "test:"))
	}
}

func TestSendDingTalkTestMessageRequiresConfiguration(t *testing.T) {
	setupDingTalkNotificationServiceDB(t)
	settings := system_setting.GetDingTalkSettings()
	previousSettings := *settings
	*settings = system_setting.DingTalkSettings{}
	t.Cleanup(func() { *settings = previousSettings })

	_, err := SendDingTalkTestMessage(context.Background(), 1)

	assert.ErrorIs(t, err, ErrDingTalkNotConfigured)
}

func newDingTalkTestServer(t *testing.T, detailRequests *atomic.Int32) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/gettoken":
			_, _ = fmt.Fprint(w, `{"errcode":0,"access_token":"legacy-token","expires_in":7200}`)
		case "/topapi/v2/user/get":
			detailRequests.Add(1)
			_, _ = fmt.Fprint(w, `{"errcode":0,"result":{"userid":"alice","unionid":"union-alice","email":"alice@example.com"}}`)
		case "/v1.0/oauth2/accessToken":
			_, _ = fmt.Fprint(w, `{"accessToken":"access-token","expireIn":7200}`)
		case "/v1.0/robot/oToMessages/batchSend":
			var payload struct {
				RobotCode string   `json:"robotCode"`
				UserIDs   []string `json:"userIds"`
			}
			require.NoError(t, common.DecodeJson(r.Body, &payload))
			assert.Equal(t, "app-key", payload.RobotCode)
			assert.Equal(t, []string{"alice"}, payload.UserIDs)
			_, _ = fmt.Fprint(w, `{"processQueryKey":"query-key"}`)
		default:
			http.NotFound(w, r)
		}
	}))
}
