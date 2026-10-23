package service

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
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
	cachedAt := time.Now().Add(-5*time.Minute - 24*time.Second).UTC().Format(time.RFC3339Nano)
	for _, test := range []struct {
		name, cards string
		available   bool
		reason      string
	}{
		{"available", available, true, ""},
		{"cached account available", `{"provider":"codex","kind":"subscription","readAt":"` + cachedAt + `","windows":[{"used":2},{"used":32}]}`, true, ""},
		{"cached account held", `{"provider":"codex","kind":"subscription","held":true,"readAt":"` + cachedAt + `","windows":[{"used":100}]}`, false, ""},
		{"nine-minute cache available", `{"provider":"codex","kind":"subscription","readAt":"` + time.Now().Add(-9*time.Minute).UTC().Format(time.RFC3339Nano) + `","windows":[{"used":0}]}`, true, ""},
		{"eleven-minute cache unknown", `{"provider":"codex","kind":"subscription","readAt":"` + time.Now().Add(-11*time.Minute).UTC().Format(time.RFC3339Nano) + `","windows":[{"used":0}]}`, false, "读取时间已超过 10 分钟"},
		// 真实 Team 号池响应：周额度尚余，但两个账号均被五小时窗口暂停。
		{"five-hour exhausted", `{"provider":"codex","name":"Codex","icon":"codex-color","plan":"team","user":"1 · Team","windows":[{"name":"5 hours","used":100,"resetsAt":"2026-10-12T15:58:42Z","resetSecs":15809},{"name":"7 days","used":32,"resetsAt":"2026-10-19T05:04:49Z","resetSecs":581376}],"until":"2026-10-28T10:41:22Z","readAt":"` + now + `","held":true,"kind":"subscription"},{"provider":"codex","name":"Codex","icon":"codex-color","plan":"team","user":"2 · Team","windows":[{"name":"5 hours","used":100,"resetsAt":"2026-10-12T14:27:12Z","resetSecs":10319},{"name":"7 days","used":32,"resetsAt":"2026-10-19T02:29:07Z","resetSecs":572034}],"until":"2026-10-28T10:41:22Z","readAt":"` + now + `","held":true,"kind":"subscription"}`, false, ""},
		{"weekly exhausted after five hours", held, false, ""},
		{"credits bypass exhausted windows", `{"provider":"codex","kind":"subscription","readAt":"` + now + `","windows":[{"used":100},{"used":100}]}`, true, ""},
		{"one account recovers", held + "," + available, true, ""},
		{"no Codex accounts", `{"provider":"claude","kind":"subscription","windows":[{"used":0}]}`, false, ""},
		{"empty pool", "", false, ""},
		{"unknown account", `{"provider":"codex","kind":"subscription","user":"private@example.com","error":"gateway-key private@example.com"}`, false, "error 非空"},
		{"stale reading", `{"provider":"codex","kind":"subscription","readAt":"2020-01-01T00:00:00Z","windows":[{"used":0}]}`, false, "读取时间已超过 10 分钟"},
		{"historical fallback", `{"provider":"codex","kind":"subscription","readAt":"` + now + `","asOf":"` + now + `","windows":[{"used":0}]}`, false, "asOf 非空"},
		{"missing read time", `{"provider":"codex","kind":"subscription","windows":[{"used":0}]}`, false, "缺少读取时间（readAt）"},
		{"future reading", `{"provider":"codex","kind":"subscription","readAt":"` + time.Now().Add(10*time.Minute).UTC().Format(time.RFC3339Nano) + `","windows":[{"used":0}]}`, false, "请检查时钟"},
		{"missing windows", `{"provider":"codex","kind":"subscription","readAt":"` + now + `"}`, false, "缺少用量窗口（windows）"},
		{"missing percent", `{"provider":"codex","kind":"subscription","readAt":"` + now + `","windows":[{}]}`, false, "缺少有效 used 值"},
		{"unknown plus available", `{"provider":"codex","kind":"subscription","error":"unavailable"},` + available, true, ""},
		{"unknown plus held", `{"provider":"codex","kind":"subscription","error":"unavailable"},` + held, false, "error 非空"},
		{"multiple query errors", `{"provider":"codex","kind":"subscription","user":"a","error":"unavailable"},{"provider":"codex","kind":"subscription","user":"b","error":"unavailable"}`, false, "error 非空）（2）"},
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
			if test.reason != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), test.reason)
				assert.NotContains(t, err.Error(), "gateway-key")
				assert.NotContains(t, err.Error(), "private@example.com")
			} else {
				require.NoError(t, err)
			}
			assert.Equal(t, test.available, actual.Available)
			assert.Equal(t, 2, calls)
		})
	}
}

func TestMagpieUnavailablePoolReportsWeeklyQuotaPerAccount(t *testing.T) {
	for _, test := range []struct {
		name     string
		accounts []string
		want     string
	}{
		{
			name:     "five-hour limit preserves weekly remaining",
			accounts: []string{`[{"name":"5 hours","used":100},{"name":"7 days","used":32}]`, `[{"name":"7 days","used":32},{"name":"5 hours","used":100}]`},
			want:     "68%",
		},
		{
			name:     "different weekly limits form a range",
			accounts: []string{`[{"name":"5 hours","used":100},{"name":"7 days","used":32}]`, `[{"name":"7 days","used":100}]`},
			want:     "0%～68%",
		},
		{
			name:     "weekly overshoot cannot show negative quota",
			accounts: []string{`[{"name":"7 days","used":103}]`},
			want:     "0%",
		},
		{
			name:     "fractional weekly usage",
			accounts: []string{`[{"name":"5 hours","used":100},{"name":"7 days","used":32.25}]`},
			want:     "67.75%",
		},
		{
			name:     "missing weekly window is unknown",
			accounts: []string{`[{"name":"5 hours","used":100}]`},
			want:     "暂无法确定",
		},
		{
			name:     "known unlimited and unknown remain distinct",
			accounts: []string{`[{"name":"7 days","used":32}]`, `[{"name":"5 hours","used":100},{"name":"7 days","unlimited":true}]`, `[{"name":"5 hours","used":100}]`},
			want:     "68%；无限额；暂无法确定",
		},
		{
			name: "no Codex subscription accounts",
			want: "无 Codex 订阅账号",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			cards := []string{`{"provider":"claude","kind":"subscription","windows":[{"name":"7 days","used":0}]}`, `{"provider":"codex","kind":"apikey","windows":[{"name":"7 days","used":0}]}`}
			for _, windows := range test.accounts {
				// user 是展示名称，可能重复；不能用它去重账号或带入群通知。
				cards = append(cards, fmt.Sprintf(`{"provider":"codex","kind":"subscription","user":"private@example.com","held":true,"readAt":"%s","windows":%s}`, time.Now().UTC().Format(time.RFC3339Nano), windows))
			}
			body := `{"object":"list","data":[` + strings.Join(cards, ",") + `]}`
			calls := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				calls++
				_, _ = w.Write([]byte(body))
			}))
			defer server.Close()
			channel := &model.Channel{
				Key: "first-key\nsecond-key", BaseURL: common.GetPointer(server.URL), OtherSettings: `{"magpie_enabled":true}`,
				ChannelInfo: model.ChannelInfo{IsMultiKey: true, MultiKeySize: 2},
			}
			availability, err := queryMagpieCodexAvailability(context.Background(), channel)
			require.NoError(t, err)
			assert.False(t, availability.Available)
			assert.Equal(t, test.want, availability.WeeklyQuota)
			assert.NotContains(t, availability.WeeklyQuota, "private@example.com")
			assert.Equal(t, 4, calls)
		})
	}
}

func TestMagpieReadingTimeDiagnosticsPreserveUnknownAndProtectCredentials(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	for _, test := range []struct {
		name, reason   string
		oldest, newest time.Time
	}{
		{"stale", "读取时间已超过 10 分钟（2）", now.Add(-12 * time.Minute), now.Add(-11 * time.Minute)},
		{"future", "请检查时钟（2）", now.Add(5 * time.Minute), now.Add(10 * time.Minute)},
	} {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				// 顺序与时间顺序相反，诊断范围不能依赖上游账号排序。
				_, _ = fmt.Fprintf(w, `{"object":"list","data":[{"provider":"codex","kind":"subscription","user":"private@example.com","readAt":"%s","held":true,"windows":[{"used":100}]},{"provider":"codex","kind":"subscription","user":"another@example.com","readAt":"%s","held":true,"windows":[{"used":100}]}]}`, test.newest.Format(time.RFC3339Nano), test.oldest.Format(time.RFC3339Nano))
			}))
			defer server.Close()
			result, err := queryMagpieCodexAvailability(context.Background(), &model.Channel{Key: "private-gateway-key", BaseURL: common.GetPointer(server.URL), OtherSettings: `{"magpie_enabled":true}`})
			require.Error(t, err)
			assert.False(t, result.Available)
			assert.True(t, result.NextCheckAt.IsZero())
			message := err.Error()
			assert.Contains(t, message, test.reason)
			assert.Contains(t, message, "readAt 范围（UTC）："+test.oldest.Format(time.RFC3339Nano)+" ~ "+test.newest.Format(time.RFC3339Nano))
			checked := regexp.MustCompile(`检查时间（UTC）：([^;]+)`).FindStringSubmatch(message)
			require.Len(t, checked, 2)
			checkedAt, parseErr := time.Parse(time.RFC3339Nano, checked[1])
			require.NoError(t, parseErr)
			assert.Contains(t, message, fmt.Sprintf("数据年龄范围：%s ~ %s", checkedAt.Sub(test.newest).Round(time.Millisecond), checkedAt.Sub(test.oldest).Round(time.Millisecond)))
			assert.Contains(t, message, "允许 -1m0s 至 10m0s")
			for _, private := range []string{"private-gateway-key", "private@example.com", "another@example.com", server.URL} {
				assert.NotContains(t, message, private)
			}
		})
	}
}

func TestMagpieMonitorIntervalUsesBeijingTime(t *testing.T) {
	for _, test := range []struct {
		name     string
		instant  string
		interval time.Duration
	}{
		{"before morning boundary", "2026-10-12T08:59:59+08:00", 30 * time.Minute},
		{"morning boundary in UTC", "2026-10-12T01:00:00Z", 2 * time.Minute},
		{"before evening boundary", "2026-10-12T21:59:59+08:00", 2 * time.Minute},
		{"evening boundary in UTC", "2026-10-12T14:00:00Z", 30 * time.Minute},
		{"after midnight", "2026-10-13T00:00:00+08:00", 30 * time.Minute},
		{"daytime with other timezone", "2026-10-11T18:00:00-07:00", 2 * time.Minute},
	} {
		t.Run(test.name, func(t *testing.T) {
			now, err := time.Parse(time.RFC3339, test.instant)
			require.NoError(t, err)
			assert.Equal(t, test.interval, magpieChannelMonitorInterval(now))
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

func TestMagpieDisabledKeysReportUnknownWithoutRequests(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) { calls++ }))
	defer server.Close()
	channel := &model.Channel{
		Key: "secret", BaseURL: common.GetPointer(server.URL), OtherSettings: `{"magpie_enabled":true}`,
		ChannelInfo: model.ChannelInfo{IsMultiKey: true, MultiKeyStatusList: map[int]int{0: common.ChannelStatusManuallyDisabled}},
	}
	available, err := queryMagpieCodexAvailability(context.Background(), channel)
	require.Error(t, err)
	assert.False(t, available.Available)
	assert.Contains(t, err.Error(), "没有已启用的网关密钥")
	assert.NotContains(t, err.Error(), channel.Key)
	assert.Zero(t, calls)
}

func TestMagpieMonitorNotifiesOnlyTransitions(t *testing.T) {
	setupAnnouncementGroupNotificationTest(t)
	previousLogDB := model.LOG_DB
	model.LOG_DB = model.DB
	t.Cleanup(func() { model.LOG_DB = previousLogDB })
	require.NoError(t, model.DB.AutoMigrate(&model.Channel{}, &model.Ability{}, &model.Log{}, &model.User{}))
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
		body, err := common.Marshal(map[string]any{"object": "list", "data": []any{map[string]any{"provider": "codex", "kind": "subscription", "user": "private@example.com", "held": held, "readAt": time.Now().UTC(), "windows": []any{map[string]any{"name": "5 hours", "used": 100}, map[string]any{"name": "7 days", "used": 32}}}}})
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
		assert.NotContains(t, content, "渠道：")
		assert.NotContains(t, content, channel.Name)
		assert.NotContains(t, content, "private@example.com")
		titles = append(titles, title)
		if title == "Magpie 自建渠道已自动启用" {
			assert.Equal(t, "原因：检测到可用账号，渠道已自动启用，可继续使用。", content)
			assert.NotContains(t, content, "可能最早启用时间")
			return errors.New("notification unavailable")
		}
		assert.Contains(t, content, "原因：无可用账号，渠道已自动禁用，等待小时或周限制恢复后启用。")
		assert.Contains(t, content, "周额度剩余（按账号）：68%\n")
		assert.NotContains(t, content, "个账号")
		assert.NotContains(t, content, "耗尽")
		assert.Contains(t, content, "可能最早启用时间：暂无法确定")
		return nil
	}
	t.Cleanup(func() { sendMagpieChannelGroupMessage = previousSend })
	result, err := syncMagpieChannels(context.Background(), false)
	require.NoError(t, err)
	assert.Equal(t, 1, result.Checked)
	assert.Equal(t, 1, result.Changed)
	assert.Equal(t, 1, result.Disabled)
	require.Len(t, result.Channels, 1)
	assert.Equal(t, channel.Id, result.Channels[0].ChannelID)
	assert.Equal(t, channel.Name, result.Channels[0].ChannelName)
	require.NotNil(t, result.Channels[0].Available)
	assert.False(t, *result.Channels[0].Available)
	assert.Equal(t, "disabled", result.Channels[0].Action)
	result, err = syncMagpieChannels(context.Background(), false)
	require.NoError(t, err)
	assert.Zero(t, result.Changed)
	assert.Equal(t, 1, result.Unchanged)
	assert.Equal(t, "unchanged", result.Channels[0].Action)
	fail = true
	result, err = syncMagpieChannels(context.Background(), false)
	require.NoError(t, err)
	assert.Zero(t, result.Changed)
	assert.Equal(t, 1, result.Failed)
	assert.Nil(t, result.Channels[0].Available)
	assert.Equal(t, "query_failed", result.Channels[0].Action)
	assert.Contains(t, result.Channels[0].Error, "HTTP 503")
	stored, err := model.GetChannelById(channel.Id, true)
	require.NoError(t, err)
	assert.Equal(t, common.ChannelStatusAutoDisabled, stored.Status)
	fail, held = false, false
	result, err = syncMagpieChannels(context.Background(), false)
	require.NoError(t, err)
	assert.Equal(t, 1, result.Changed)
	assert.Equal(t, 1, result.Enabled)
	assert.Zero(t, result.Disabled)
	assert.Equal(t, 1, result.NotificationFailed)
	assert.Equal(t, "enabled", result.Channels[0].Action)
	assert.Equal(t, "notification unavailable", result.Channels[0].NotificationError)
	serialized, err := common.Marshal(result)
	require.NoError(t, err)
	assert.NotContains(t, string(serialized), channel.Key)
	result, err = syncMagpieChannels(context.Background(), false)
	require.NoError(t, err)
	assert.Zero(t, result.Changed)
	assert.Equal(t, []string{"Magpie 自建渠道已自动禁用", "Magpie 自建渠道已自动启用"}, titles)
	var records []model.DingTalkNotification
	require.NoError(t, model.DB.Find(&records).Error)
	require.Len(t, records, 2)
	assert.Equal(t, model.DingTalkNotificationStatusSucceeded, records[0].Status)
	assert.Equal(t, model.DingTalkNotificationStatusFailed, records[1].Status)
	require.True(t, model.UpdateChannelStatus(channel.Id, "", common.ChannelStatusManuallyDisabled, "manual"))
	result, err = syncMagpieChannels(context.Background(), false)
	require.NoError(t, err)
	assert.Zero(t, result.Checked)
	assert.Zero(t, result.Changed)
	assert.Empty(t, result.Channels)
	var logs []model.Log
	require.NoError(t, model.LOG_DB.Where("type = ?", model.LogTypeSystem).Order("id ASC").Find(&logs).Error)
	require.Len(t, logs, 2)
	assert.Equal(t, fmt.Sprintf("渠道「%s」（#%d）：Magpie 无可用 Codex 账号，自动禁用", channel.Name, channel.Id), logs[0].Content)
	assert.Equal(t, fmt.Sprintf("渠道「%s」（#%d）：Magpie 已有可用 Codex 账号，自动启用", channel.Name, channel.Id), logs[1].Content)
	for index, log := range logs {
		assert.Zero(t, log.UserId)
		assert.Positive(t, log.CreatedAt)
		assert.NotContains(t, log.Content+log.Other, channel.Key)
		var metadata struct {
			AdminInfo struct {
				ChannelID   int    `json:"channel_id"`
				ChannelName string `json:"channel_name"`
				Status      int    `json:"status"`
			} `json:"admin_info"`
		}
		require.NoError(t, common.UnmarshalJsonStr(log.Other, &metadata))
		assert.Equal(t, channel.Id, metadata.AdminInfo.ChannelID)
		assert.Equal(t, channel.Name, metadata.AdminInfo.ChannelName)
		assert.Equal(t, []int{common.ChannelStatusAutoDisabled, common.ChannelStatusEnabled}[index], metadata.AdminInfo.Status)
	}
}

func TestMagpieMonitorPersistsPartialResultsOnCancellation(t *testing.T) {
	setupAnnouncementGroupNotificationTest(t)
	require.NoError(t, model.DB.AutoMigrate(&model.Channel{}, &model.Ability{}))
	previousCache := common.MemoryCacheEnabled
	common.MemoryCacheEnabled = false
	t.Cleanup(func() { common.MemoryCacheEnabled = previousCache })
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests++
		if requests == 3 {
			// 第一个渠道完成后，在第二个渠道查询期间取消任务。
			cancel()
			return
		}
		body, err := common.Marshal(map[string]any{"object": "list", "data": []any{map[string]any{"provider": "codex", "kind": "subscription", "readAt": time.Now().UTC(), "windows": []any{map[string]any{"used": 10}}}}})
		require.NoError(t, err)
		_, _ = w.Write(body)
	}))
	defer server.Close()
	for index := 0; index < 3; index++ {
		channel := model.Channel{Name: fmt.Sprintf("pool-%d", index), Key: "private-gateway-key", BaseURL: common.GetPointer(server.URL), Models: "model", Group: "default", OtherSettings: `{"magpie_enabled":true}`}
		require.NoError(t, channel.Insert())
	}
	task, err := model.CreateSystemTask(model.SystemTaskTypeMagpieChannelMonitor, nil, nil)
	require.NoError(t, err)
	claimedTask, claimed, err := model.ClaimSystemTask(task.ID, task.Type, "runner-magpie", common.GetTimestamp()+60)
	require.NoError(t, err)
	require.True(t, claimed)

	magpieChannelMonitorHandler{}.Run(ctx, claimedTask, "runner-magpie")

	var stored model.SystemTask
	require.NoError(t, model.DB.Where("task_id = ?", task.TaskID).First(&stored).Error)
	assert.Equal(t, model.SystemTaskStatusFailed, stored.Status)
	assert.Equal(t, context.Canceled.Error(), stored.Error)
	assert.NotContains(t, stored.Result, "private-gateway-key")
	var result magpieChannelMonitorResult
	require.NoError(t, common.UnmarshalJsonStr(stored.Result, &result))
	assert.Equal(t, 2, result.Checked)
	assert.Equal(t, 1, result.Unchanged)
	assert.Equal(t, 1, result.Failed)
	assert.Zero(t, result.Changed)
	require.Len(t, result.Channels, 2)
	assert.Equal(t, "unchanged", result.Channels[0].Action)
	assert.Equal(t, "query_failed", result.Channels[1].Action)
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
	assert.True(t, available.Available)
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
	assert.Contains(t, err.Error(), "刷新响应缺少原有 Codex 账号（1）")
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
	assert.Contains(t, err.Error(), "刷新响应缺少原有 Codex 账号（1）")
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
	assert.True(t, available.Available)
}
