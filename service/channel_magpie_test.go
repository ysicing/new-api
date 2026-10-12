package service

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/system_setting"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMagpieCodexAvailability(t *testing.T) {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	available := `{"provider":"codex","kind":"subscription","readAt":"` + now + `","windows":[{"used":10}]}`
	held := `{"provider":"codex","kind":"subscription","held":true,"readAt":"` + now + `","windows":[{"name":"5 hours","used":100},{"name":"7 days","used":100}]}`
	for _, test := range []struct {
		name, cards        string
		available, unknown bool
	}{
		{"available", available, true, false},
		{"five-hour exhausted", `{"provider":"codex","kind":"subscription","held":true,"readAt":"` + now + `","windows":[{"name":"5 hours","used":100},{"name":"7 days","used":50}]}`, false, false},
		{"weekly exhausted after five hours", held, false, false},
		{"credits bypass exhausted windows", `{"provider":"codex","kind":"subscription","readAt":"` + now + `","windows":[{"used":100},{"used":100}]}`, true, false},
		{"one account recovers", held + "," + available, true, false},
		{"no Codex accounts", `{"provider":"claude","kind":"subscription","windows":[{"used":0}]}`, false, false},
		{"empty pool", "", false, false},
		{"unknown account", `{"provider":"codex","kind":"subscription","error":"usage unavailable"}`, false, true},
		{"stale reading", `{"provider":"codex","kind":"subscription","readAt":"2020-01-01T00:00:00Z","windows":[{"used":0}]}`, false, true},
		{"historical fallback", `{"provider":"codex","kind":"subscription","readAt":"` + now + `","asOf":"` + now + `","windows":[{"used":0}]}`, false, true},
		{"missing windows", `{"provider":"codex","kind":"subscription","readAt":"` + now + `"}`, false, true},
		{"missing percent", `{"provider":"codex","kind":"subscription","readAt":"` + now + `","windows":[{}]}`, false, true},
		{"unknown plus available", `{"provider":"codex","kind":"subscription","error":"unavailable"},` + available, true, false},
		{"unknown plus held", `{"provider":"codex","kind":"subscription","error":"unavailable"},` + held, false, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			calls := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				assert.Equal(t, "Bearer gateway-key", r.Header.Get("Authorization"))
				if r.Method == http.MethodGet {
					assert.Equal(t, "/prefix/v1/magpie/quotas", r.URL.Path)
				} else {
					assert.Equal(t, http.MethodPost, r.Method)
					assert.Equal(t, "/prefix/v1/magpie/quotas/refresh", r.URL.Path)
					provider := "codex"
					if test.name == "no Codex accounts" || test.name == "empty pool" {
						provider = ""
					}
					assert.Equal(t, provider, r.URL.Query().Get("provider"))
				}
				_, _ = w.Write([]byte(`{"object":"list","data":[` + test.cards + `]}`))
			}))
			defer server.Close()
			channel := &model.Channel{Key: "gateway-key", BaseURL: common.GetPointer(server.URL + "/prefix/v1/"), OtherSettings: `{"magpie_enabled":true}`}
			actual, err := queryMagpieCodexAvailability(context.Background(), channel)
			if test.unknown {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			assert.Equal(t, test.available, actual)
			assert.Equal(t, 2, calls)
		})
	}
}

func TestMagpieQueryFailuresPreserveStatus(t *testing.T) {
	for _, test := range []struct {
		name, body string
		status     int
	}{
		{"denied", `{"object":"list","data":[]}`, 403},
		{"rate limited", `{"object":"list","data":[]}`, 429},
		{"invalid JSON", `invalid`, 200},
		{"missing data", `{"object":"list"}`, 200},
		{"null data", `{"object":"list","data":null}`, 200},
		{"wrong object", `{"object":"error","data":[]}`, 200},
	} {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(test.status)
				_, _ = w.Write([]byte(test.body))
			}))
			defer server.Close()
			channel := &model.Channel{Status: common.ChannelStatusEnabled, Key: "secret", BaseURL: common.GetPointer(server.URL), OtherSettings: `{"magpie_enabled":true}`}
			_, err := queryMagpieCodexAvailability(context.Background(), channel)
			require.Error(t, err)
			assert.NotContains(t, err.Error(), "secret")
			assert.Equal(t, common.ChannelStatusEnabled, channel.Status)
		})
	}
}

func TestMagpieQueryRefusesCredentialRedirect(t *testing.T) {
	destinationCalls := 0
	destination := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) { destinationCalls++ }))
	defer destination.Close()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, destination.URL, http.StatusTemporaryRedirect)
	}))
	defer server.Close()
	_, err := queryMagpieCodexAvailability(context.Background(), &model.Channel{Key: "secret", BaseURL: common.GetPointer(server.URL), OtherSettings: `{"magpie_enabled":true}`})
	require.Error(t, err)
	assert.Zero(t, destinationCalls)
}

func TestMagpieMonitorNotifiesOnlyTransitions(t *testing.T) {
	setupAnnouncementGroupNotificationTest(t)
	require.NoError(t, model.DB.AutoMigrate(&model.Channel{}, &model.Ability{}))
	previousCache := common.MemoryCacheEnabled
	common.MemoryCacheEnabled = false
	t.Cleanup(func() { common.MemoryCacheEnabled = previousCache })
	held := true
	fail := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if fail {
			w.WriteHeader(503)
			return
		}
		body, err := common.Marshal(map[string]any{"object": "list", "data": []any{map[string]any{"provider": "codex", "kind": "subscription", "held": held, "readAt": time.Now().UTC(), "windows": []any{map[string]any{"used": 100}}}}})
		require.NoError(t, err)
		_, _ = w.Write(body)
	}))
	defer server.Close()
	channel := model.Channel{Name: "Magpie upstream", Key: "secret-not-in-message", BaseURL: common.GetPointer(server.URL), Models: "model", Group: "default", OtherSettings: `{"magpie_enabled":true}`}
	require.NoError(t, channel.Insert())
	previousSend := sendMagpieChannelGroupMessage
	titles := []string{}
	sendMagpieChannelGroupMessage = func(_ context.Context, _ system_setting.DingTalkSettings, recipient, title, content string) error {
		assert.Equal(t, "cid-announcement", recipient)
		assert.NotContains(t, content, channel.Key)
		titles = append(titles, title)
		return nil
	}
	t.Cleanup(func() { sendMagpieChannelGroupMessage = previousSend })
	changed, err := syncMagpieChannels(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 1, changed)
	changed, err = syncMagpieChannels(context.Background())
	require.NoError(t, err)
	assert.Zero(t, changed)
	fail = true
	changed, err = syncMagpieChannels(context.Background())
	require.NoError(t, err)
	assert.Zero(t, changed)
	stored, err := model.GetChannelById(channel.Id, true)
	require.NoError(t, err)
	assert.Equal(t, common.ChannelStatusAutoDisabled, stored.Status)
	fail, held = false, false
	changed, err = syncMagpieChannels(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 1, changed)
	changed, err = syncMagpieChannels(context.Background())
	require.NoError(t, err)
	assert.Zero(t, changed)
	assert.Equal(t, []string{"Magpie 渠道已自动禁用", "Magpie 渠道已自动启用"}, titles)
	var records []model.DingTalkNotification
	require.NoError(t, model.DB.Find(&records).Error)
	assert.Len(t, records, 2)
	require.True(t, model.UpdateChannelStatus(channel.Id, "", common.ChannelStatusManuallyDisabled, "manual"))
	changed, err = syncMagpieChannels(context.Background())
	require.NoError(t, err)
	assert.Zero(t, changed)
}

func TestMagpieQueryUsesEnabledKeysAndChannelProxy(t *testing.T) {
	keys := []string{}
	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "magpie.internal", r.URL.Host)
		key := r.Header.Get("Authorization")
		keys = append(keys, key)
		if key == "Bearer invalid-key" {
			w.WriteHeader(401)
			return
		}
		body, err := common.Marshal(map[string]any{"object": "list", "data": []any{map[string]any{"provider": "codex", "kind": "subscription", "readAt": time.Now().UTC(), "windows": []any{map[string]any{"used": 10}}}}})
		require.NoError(t, err)
		_, _ = w.Write(body)
	}))
	defer proxy.Close()
	settings, err := common.Marshal(map[string]string{"proxy": proxy.URL})
	require.NoError(t, err)
	channel := model.Channel{Key: "disabled-key\ninvalid-key\nvalid-key", BaseURL: common.GetPointer("http://magpie.internal"), Setting: common.GetPointer(string(settings)), OtherSettings: `{"magpie_enabled":true}`, ChannelInfo: model.ChannelInfo{IsMultiKey: true, MultiKeySize: 3, MultiKeyPollingIndex: 2, MultiKeyStatusList: map[int]int{0: common.ChannelStatusManuallyDisabled}}}
	available, err := queryMagpieCodexAvailability(context.Background(), &channel)
	require.NoError(t, err)
	assert.True(t, available)
	assert.Equal(t, []string{"Bearer invalid-key", "Bearer valid-key", "Bearer valid-key"}, keys)
	assert.Equal(t, common.ChannelStatusManuallyDisabled, channel.ChannelInfo.MultiKeyStatusList[0])
	assert.Equal(t, 2, channel.ChannelInfo.MultiKeyPollingIndex)
}

func TestMagpieMissingRefreshedCardsAndCancellationAreUnknown(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			_, _ = w.Write([]byte(`{"object":"list","data":[{"provider":"codex","kind":"subscription"}]}`))
			return
		}
		_, _ = w.Write([]byte(`{"object":"list","data":[]}`))
	}))
	defer server.Close()
	channel := model.Channel{Key: "key", BaseURL: common.GetPointer(server.URL), OtherSettings: `{"magpie_enabled":true}`}
	_, err := queryMagpieCodexAvailability(context.Background(), &channel)
	require.Error(t, err)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = queryMagpieCodexAvailability(ctx, &channel)
	require.Error(t, err)
}

func TestMagpieMissingAccountMakesExhaustedPoolUnknown(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			_, _ = w.Write([]byte(`{"object":"list","data":[{"provider":"codex","kind":"subscription","user":"a"},{"provider":"codex","kind":"subscription","user":"b"}]}`))
			return
		}
		body, err := common.Marshal(map[string]any{"object": "list", "data": []any{map[string]any{"provider": "codex", "kind": "subscription", "user": "a", "held": true, "readAt": time.Now().UTC(), "windows": []any{map[string]any{"used": 100}}}}})
		require.NoError(t, err)
		_, _ = w.Write(body)
	}))
	defer server.Close()
	_, err := queryMagpieCodexAvailability(context.Background(), &model.Channel{Key: "key", BaseURL: common.GetPointer(server.URL), OtherSettings: `{"magpie_enabled":true}`})
	require.Error(t, err)
}

func TestMagpieInitialEmptyCacheRequiresFullRefresh(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Magpie 首次 GET 超时后返回空卡片；按 provider 刷新无法初始化 nil 缓存。
		if r.Method == http.MethodGet || r.URL.Query().Get("provider") != "" {
			_, _ = w.Write([]byte(`{"object":"list","data":[]}`))
			return
		}
		body, err := common.Marshal(map[string]any{"object": "list", "data": []any{map[string]any{"provider": "codex", "kind": "subscription", "readAt": time.Now().UTC(), "windows": []any{map[string]any{"used": 10}}}}})
		require.NoError(t, err)
		_, _ = w.Write(body)
	}))
	defer server.Close()
	available, err := queryMagpieCodexAvailability(context.Background(), &model.Channel{Key: "key", BaseURL: common.GetPointer(server.URL), OtherSettings: `{"magpie_enabled":true}`})
	require.NoError(t, err)
	assert.True(t, available)
}
