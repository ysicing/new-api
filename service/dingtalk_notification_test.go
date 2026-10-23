package service

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sync/atomic"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/system_setting"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupDingTalkNotificationServiceDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:dingtalk-notification-service-%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&model.User{}, &model.DingTalkNotification{}, &model.ExternalIdentityClaim{},
	))
	previousDB := model.DB
	model.DB = db
	t.Cleanup(func() { model.DB = previousDB })
	return db
}

func TestDingTalkUserIDFromEmail(t *testing.T) {
	tests := []struct {
		name   string
		email  string
		userID string
		ok     bool
	}{
		{name: "valid email", email: " alice@example.com ", userID: "alice", ok: true},
		{name: "missing email", email: "", ok: false},
		{name: "missing domain", email: "alice", ok: false},
		{name: "missing prefix", email: "@example.com", ok: false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			userID, ok := dingTalkUserIDFromEmail(test.email)
			assert.Equal(t, test.userID, userID)
			assert.Equal(t, test.ok, ok)
		})
	}
}

func TestDingTalkNotifierSendsMarkdownAndReusesAccessToken(t *testing.T) {
	var tokenRequests atomic.Int32
	var messageRequests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1.0/oauth2/accessToken":
			tokenRequests.Add(1)
			var payload struct {
				AppKey    string `json:"appKey"`
				AppSecret string `json:"appSecret"`
			}
			require.NoError(t, common.DecodeJson(r.Body, &payload))
			assert.Equal(t, "app-key", payload.AppKey)
			assert.Equal(t, "app-secret", payload.AppSecret)
			_, _ = fmt.Fprint(w, `{"accessToken":"access-token","expireIn":7200}`)
		case "/v1.0/robot/oToMessages/batchSend":
			messageRequests.Add(1)
			assert.Equal(t, "access-token", r.Header.Get("x-acs-dingtalk-access-token"))
			var payload struct {
				RobotCode string   `json:"robotCode"`
				UserIDs   []string `json:"userIds"`
				MsgKey    string   `json:"msgKey"`
				MsgParam  string   `json:"msgParam"`
			}
			require.NoError(t, common.DecodeJson(r.Body, &payload))
			assert.Equal(t, "app-key", payload.RobotCode)
			assert.Equal(t, "sampleMarkdown", payload.MsgKey)
			assert.True(t, reflect.DeepEqual([]string{"alice"}, payload.UserIDs))
			var markdown struct {
				Title string `json:"title"`
				Text  string `json:"text"`
			}
			require.NoError(t, common.UnmarshalJsonStr(payload.MsgParam, &markdown))
			assert.Equal(t, "体验额度已用完", markdown.Title)
			assert.Equal(t, "### 体验额度已用完\n\n当前体验额度已经用完", markdown.Text)
			_, _ = fmt.Fprint(w, `{"processQueryKey":"query-key"}`)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)

	notifier := newDingTalkNotifier(server.Client(), server.URL)
	notifier.now = func() time.Time { return time.Unix(1_700_000_000, 0) }
	settings := system_setting.DingTalkSettings{ClientId: "app-key", ClientSecret: "app-secret"}

	require.NoError(t, notifier.Send(context.Background(), settings, "alice", "体验额度已用完", "当前体验额度已经用完"))
	require.NoError(t, notifier.Send(context.Background(), settings, "alice", "体验额度已用完", "当前体验额度已经用完"))
	assert.Equal(t, int32(1), tokenRequests.Load())
	assert.Equal(t, int32(2), messageRequests.Load())
}

func TestDingTalkNotifierSendsGroupMarkdown(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1.0/oauth2/accessToken":
			_, _ = fmt.Fprint(w, `{"accessToken":"access-token","expireIn":7200}`)
		case "/v1.0/robot/groupMessages/send":
			assert.Equal(t, "access-token", r.Header.Get("x-acs-dingtalk-access-token"))
			var payload struct {
				RobotCode          string `json:"robotCode"`
				OpenConversationId string `json:"openConversationId"`
				MsgKey             string `json:"msgKey"`
				MsgParam           string `json:"msgParam"`
			}
			require.NoError(t, common.DecodeJson(r.Body, &payload))
			assert.Equal(t, "app-key", payload.RobotCode)
			assert.Equal(t, "cid-group", payload.OpenConversationId)
			assert.Equal(t, "sampleMarkdown", payload.MsgKey)
			var markdown struct {
				Title string `json:"title"`
				Text  string `json:"text"`
			}
			require.NoError(t, common.UnmarshalJsonStr(payload.MsgParam, &markdown))
			assert.Equal(t, "系统公告", markdown.Title)
			assert.Equal(t, "### 系统公告\n\n公告正文", markdown.Text)
			_, _ = fmt.Fprint(w, `{"processQueryKey":"group-query"}`)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)
	notifier := newDingTalkNotifier(server.Client(), server.URL)
	settings := system_setting.DingTalkSettings{ClientId: "app-key", ClientSecret: "app-secret"}

	require.NoError(t, notifier.SendGroup(context.Background(), settings, "cid-group", "系统公告", "公告正文"))
}

func TestDispatchDingTalkNotificationRecordsSkippedAndDeduplicates(t *testing.T) {
	setupDingTalkNotificationServiceDB(t)
	settings := system_setting.GetDingTalkSettings()
	previousSettings := *settings
	*settings = system_setting.DingTalkSettings{ClientId: "app-key", ClientSecret: "app-secret"}
	t.Cleanup(func() { *settings = previousSettings })

	request := DingTalkNotificationRequest{
		EventType: model.DingTalkNotificationEventNewUserQuotaExhausted,
		DedupeKey: "new_user_quota_exhausted:7", UserId: 7,
		Title: "体验额度已用完", Content: "当前体验额度已经用完",
	}
	created, err := DispatchDingTalkNotification(context.Background(), request)
	require.NoError(t, err)
	assert.True(t, created)
	created, err = DispatchDingTalkNotification(context.Background(), request)
	require.NoError(t, err)
	assert.False(t, created)

	var record model.DingTalkNotification
	require.NoError(t, model.DB.First(&record).Error)
	assert.Equal(t, model.DingTalkNotificationStatusSkipped, record.Status)
	assert.Contains(t, record.Error, "email")
}

func TestDispatchDingTalkNotificationRecordsFailedDelivery(t *testing.T) {
	setupDingTalkNotificationServiceDB(t)
	user := model.User{Username: "alice", Email: "alice@example.com", DingTalkId: "union-alice", Password: "password", AffCode: "alice-aff", Status: common.UserStatusEnabled}
	require.NoError(t, model.DB.Create(&user).Error)
	settings := system_setting.GetDingTalkSettings()
	previousSettings := *settings
	*settings = system_setting.DingTalkSettings{ClientId: "app-key", ClientSecret: "app-secret"}
	t.Cleanup(func() { *settings = previousSettings })
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/v1.0/oauth2/accessToken" {
			_, _ = fmt.Fprint(w, `{"accessToken":"access-token","expireIn":7200}`)
			return
		}
		_, _ = fmt.Fprint(w, `{"processQueryKey":"","invalidStaffIdList":["alice"]}`)
	}))
	t.Cleanup(server.Close)
	previousNotifier := defaultDingTalkNotifier
	defaultDingTalkNotifier = newDingTalkNotifier(server.Client(), server.URL)
	t.Cleanup(func() { defaultDingTalkNotifier = previousNotifier })

	created, err := DispatchDingTalkNotification(context.Background(), DingTalkNotificationRequest{
		EventType: model.DingTalkNotificationEventNewUserQuotaExhausted,
		DedupeKey: "new_user_quota_exhausted:alice", UserId: user.Id, UserEmail: user.Email,
		Title: "体验额度已用完", Content: "当前体验额度已经用完",
	})
	assert.True(t, created)
	require.Error(t, err)

	var record model.DingTalkNotification
	require.NoError(t, model.DB.First(&record).Error)
	assert.Equal(t, model.DingTalkNotificationStatusFailed, record.Status)
	assert.Contains(t, record.Error, "invalid staff id")
}

func TestDispatchDingTalkNotificationRecordsSuccessfulDelivery(t *testing.T) {
	setupDingTalkNotificationServiceDB(t)
	user := model.User{Username: "alice", Email: "alice@example.com", DingTalkId: "union-alice-success", Password: "password", AffCode: "alice-success-aff", Status: common.UserStatusEnabled}
	require.NoError(t, model.DB.Create(&user).Error)
	settings := system_setting.GetDingTalkSettings()
	previousSettings := *settings
	*settings = system_setting.DingTalkSettings{ClientId: "app-key", ClientSecret: "app-secret"}
	t.Cleanup(func() { *settings = previousSettings })
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/v1.0/oauth2/accessToken" {
			_, _ = fmt.Fprint(w, `{"accessToken":"access-token","expireIn":7200}`)
			return
		}
		_, _ = fmt.Fprint(w, `{"processQueryKey":"query-key"}`)
	}))
	t.Cleanup(server.Close)
	previousNotifier := defaultDingTalkNotifier
	defaultDingTalkNotifier = newDingTalkNotifier(server.Client(), server.URL)
	t.Cleanup(func() { defaultDingTalkNotifier = previousNotifier })

	created, err := DispatchDingTalkNotification(context.Background(), DingTalkNotificationRequest{
		EventType: model.DingTalkNotificationEventNewUserQuotaExhausted,
		DedupeKey: "new_user_quota_exhausted:alice-success", UserId: user.Id, UserEmail: user.Email,
		Title: "体验额度已用完", Content: "当前体验额度已经用完",
	})
	require.NoError(t, err)
	assert.True(t, created)

	var record model.DingTalkNotification
	require.NoError(t, model.DB.First(&record).Error)
	assert.Equal(t, model.DingTalkNotificationStatusSucceeded, record.Status)
	assert.NotZero(t, record.SentAt)
}
