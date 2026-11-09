package service

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/system_setting"
	"github.com/alicebob/miniredis/v2"
	"github.com/go-redis/redis/v8"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMagpieAccountNextCheckWaitsForAllExhaustedWindows(t *testing.T) {
	now := time.Date(2026, 10, 12, 11, 29, 50, 0, time.UTC)
	for _, test := range []struct {
		name, windows string
		want          time.Duration
	}{
		{"five hours only", `[{"used":100,"resetSecs":600},{"used":32,"resetSecs":3600}]`, 10 * time.Minute},
		{"both exhausted", `[{"used":100,"resetSecs":600},{"used":100,"resetSecs":3600}]`, time.Hour},
		{"absolute reset wins", `[{"used":100,"resetSecs":600,"resetsAt":"2026-10-12T11:49:50Z"}]`, 20 * time.Minute},
		{"no exhausted windows", `[{"used":50,"resetSecs":600}]`, 0},
		{"missing exhausted reset", `[{"used":100,"resetSecs":600},{"used":100}]`, 0},
		{"past reset", `[{"used":100,"resetsAt":"2026-10-12T11:00:00Z"}]`, 0},
		{"unlimited window ignored", `[{"unlimited":true,"used":100},{"used":100,"resetSecs":600}]`, 10 * time.Minute},
		{"overflowing reset", `[{"used":100,"resetSecs":9223372036854775807}]`, 0},
	} {
		t.Run(test.name, func(t *testing.T) {
			var card magpieQuotaCard
			require.NoError(t, common.UnmarshalJsonStr(`{"held":true,"windows":`+test.windows+`}`, &card))
			card.ReadAt = &now
			next := magpieAccountNextCheckAt(card, now)
			if test.want == 0 {
				assert.True(t, next.IsZero())
			} else {
				assert.Equal(t, now.Add(test.want), next)
			}
		})
	}
}

func TestMagpiePoolNextCheckUsesEarliestAccountAndRequiresKnownResets(t *testing.T) {
	for _, test := range []struct {
		name, windows string
		known         bool
	}{
		{"earliest account", `[{"used":100,"resetSecs":600}]`, true},
		{"one unknown reset", `[{"used":100}]`, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			readAt := time.Now().UTC()
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				_, _ = w.Write([]byte(`{"object":"list","data":[{"provider":"codex","kind":"subscription","user":"a","held":true,"readAt":"` + readAt.Format(time.RFC3339Nano) + `","windows":[{"used":100,"resetSecs":3600}]},{"provider":"codex","kind":"subscription","user":"b","held":true,"readAt":"` + readAt.Format(time.RFC3339Nano) + `","windows":` + test.windows + `}]}`))
			}))
			defer server.Close()
			result, err := queryMagpieCodexAvailability(context.Background(), &model.Channel{Key: "secret", BaseURL: common.GetPointer(server.URL), OtherSettings: `{"magpie_enabled":true}`})
			require.NoError(t, err)
			assert.False(t, result.Available)
			if test.known {
				assert.Equal(t, readAt.Add(10*time.Minute), result.NextCheckAt)
			} else {
				assert.True(t, result.NextCheckAt.IsZero())
			}
		})
	}
}

func TestMagpieMonitorCachesOnlyAutomaticDisablingAndRechecksOnExpiryOrEdits(t *testing.T) {
	setupAnnouncementGroupNotificationTest(t)
	require.NoError(t, model.DB.AutoMigrate(&model.Channel{}, &model.Ability{}, &model.User{}, &model.Log{}))
	previousLogDB, previousMemory := model.LOG_DB, common.MemoryCacheEnabled
	model.LOG_DB, common.MemoryCacheEnabled = model.DB, false
	t.Cleanup(func() { model.LOG_DB, common.MemoryCacheEnabled = previousLogDB, previousMemory })
	cache := miniredis.RunT(t)
	previousRedis, previousEnabled := common.RDB, common.RedisEnabled
	client := redis.NewClient(&redis.Options{Addr: cache.Addr(), MaxRetries: -1})
	common.RDB, common.RedisEnabled = client, true
	t.Cleanup(func() { _ = client.Close(); common.RDB, common.RedisEnabled = previousRedis, previousEnabled })
	previousSend := sendMagpieChannelGroupMessage
	notices := 0
	readAt := time.Now().UTC().Truncate(time.Second)
	sendMagpieChannelGroupMessage = func(_ context.Context, _ system_setting.DingTalkSettings, _, title, content string) error {
		notices++
		if title == "Magpie 自建渠道已自动禁用" {
			// 一小时后才可能恢复；Redis 的 30 分钟探测上限不能成为通知里的恢复时间。
			expected := readAt.Add(time.Hour).In(time.FixedZone("Asia/Shanghai", 8*60*60)).Format("2006-01-02 15:04:05")
			assert.Contains(t, content, "可能最早启用时间："+expected+"（北京时间）")
			assert.Contains(t, content, "实际启用以探测到可用账号为准")
		}
		return nil
	}
	t.Cleanup(func() { sendMagpieChannelGroupMessage = previousSend })
	calls, held, fail := 0, true, false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls++
		if fail {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		_, _ = w.Write([]byte(`{"object":"list","data":[{"provider":"codex","kind":"subscription","held":` + fmt.Sprint(held) + `,"readAt":"` + readAt.Format(time.RFC3339Nano) + `","windows":[{"used":100,"resetSecs":3600}]}]}`))
	}))
	defer server.Close()
	channel := &model.Channel{Name: "Cached Magpie", Key: "secret", Models: "model", Group: "default", BaseURL: common.GetPointer(server.URL), OtherSettings: `{"magpie_enabled":true}`}
	require.NoError(t, channel.Insert())
	result, err := syncMagpieChannels(context.Background(), false)
	require.NoError(t, err)
	assert.Equal(t, 1, result.Disabled)
	assert.Equal(t, 2, calls)
	assert.Equal(t, 1, notices)
	require.NotNil(t, result.Channels[0].NextCheckAt)
	assert.InDelta(t, magpieChannelProbeCacheTTL(time.Now()), time.Until(*result.Channels[0].NextCheckAt), float64(time.Second))
	result, err = syncMagpieChannels(context.Background(), false)
	require.NoError(t, err)
	assert.Equal(t, 1, result.Skipped)
	assert.Zero(t, result.Checked)
	assert.Equal(t, "cached", result.Channels[0].Action)
	assert.Nil(t, result.Channels[0].Available)
	assert.Equal(t, 2, calls)
	assert.Equal(t, 1, notices)
	// 旧版本或夜间留下的长缓存，不能延续超过当前时段允许的等待上限。
	storedChannel, err := model.GetChannelById(channel.Id, true)
	require.NoError(t, err)
	longDelay := magpieChannelProbeCacheTTL(time.Now()) + time.Minute
	require.NoError(t, client.Set(context.Background(), magpieProbeCacheKey(storedChannel), time.Now().Add(longDelay).Format(time.RFC3339Nano), longDelay).Err())
	result, err = syncMagpieChannels(context.Background(), false)
	require.NoError(t, err)
	assert.Zero(t, result.Skipped)
	assert.Equal(t, 4, calls)
	for _, edit := range []map[string]any{{"key": "new-secret"}, {"base_url": server.URL + "/prefix"}, {"setting": "{}"}, {"settings": `{"magpie_enabled":true,"note":"edited"}`}} {
		require.NoError(t, model.DB.Model(&model.Channel{}).Where("id = ?", channel.Id).Updates(edit).Error)
		before := calls
		result, err = syncMagpieChannels(context.Background(), false)
		require.NoError(t, err)
		assert.Equal(t, before+2, calls)
		assert.Equal(t, 1, result.Unchanged)
		assert.Zero(t, result.Skipped)
	}
	// 人工启用必须立刻重新探测，不能沿用此前自动禁用的缓存。
	require.NoError(t, model.DB.Model(&model.Channel{}).Where("id = ?", channel.Id).Update("status", common.ChannelStatusEnabled).Error)
	beforeManual := calls
	result, err = syncMagpieChannels(context.Background(), false)
	require.NoError(t, err)
	assert.Equal(t, beforeManual+2, calls)
	assert.Equal(t, 1, result.Disabled)
	assert.Equal(t, 2, notices)
	cache.FastForward(30 * time.Minute)
	before := calls
	result, err = syncMagpieChannels(context.Background(), false)
	require.NoError(t, err)
	assert.Equal(t, before+2, calls)
	assert.Zero(t, result.Skipped)
	cache.SetError("cache unavailable")
	held = false
	result, err = syncMagpieChannels(context.Background(), false)
	require.NoError(t, err)
	assert.Equal(t, 1, result.Enabled)
	assert.Equal(t, 3, notices)
	cache.SetError("")
	// 未知查询结果不能写缓存。
	fail = true
	result, err = syncMagpieChannels(context.Background(), false)
	require.NoError(t, err)
	assert.Equal(t, 1, result.Failed)
	before = calls
	result, err = syncMagpieChannels(context.Background(), false)
	require.NoError(t, err)
	assert.Equal(t, before+1, calls)
	assert.Equal(t, 1, result.Failed)
	require.NoError(t, model.DB.Model(&model.Channel{}).Where("id = ?", channel.Id).Update("status", common.ChannelStatusManuallyDisabled).Error)
	before = calls
	result, err = syncMagpieChannels(context.Background(), false)
	require.NoError(t, err)
	assert.Zero(t, result.Checked)
	assert.Zero(t, result.Skipped)
	assert.Equal(t, before, calls)
}

func TestMagpieManualMonitorBypassesCachedRecovery(t *testing.T) {
	setupAnnouncementGroupNotificationTest(t)
	require.NoError(t, model.DB.AutoMigrate(&model.Channel{}, &model.Ability{}, &model.User{}, &model.Log{}))
	previousLogDB, previousMemory := model.LOG_DB, common.MemoryCacheEnabled
	model.LOG_DB, common.MemoryCacheEnabled = model.DB, false
	t.Cleanup(func() { model.LOG_DB, common.MemoryCacheEnabled = previousLogDB, previousMemory })
	cache := miniredis.RunT(t)
	previousRedis, previousEnabled := common.RDB, common.RedisEnabled
	client := redis.NewClient(&redis.Options{Addr: cache.Addr(), MaxRetries: -1})
	common.RDB, common.RedisEnabled = client, true
	t.Cleanup(func() { _ = client.Close(); common.RDB, common.RedisEnabled = previousRedis, previousEnabled })
	previousSend := sendMagpieChannelGroupMessage
	notices := 0
	sendMagpieChannelGroupMessage = func(context.Context, system_setting.DingTalkSettings, string, string, string) error {
		notices++
		return nil
	}
	t.Cleanup(func() { sendMagpieChannelGroupMessage = previousSend })
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls++
		body, err := common.Marshal(map[string]any{"object": "list", "data": []any{map[string]any{"provider": "codex", "kind": "subscription", "held": false, "readAt": time.Now().UTC(), "windows": []any{map[string]any{"used": 0}}}}})
		require.NoError(t, err)
		_, _ = w.Write(body)
	}))
	defer server.Close()
	channel := &model.Channel{Name: "Early reset", Key: "secret", Models: "model", Group: "default", BaseURL: common.GetPointer(server.URL), OtherSettings: `{"magpie_enabled":true}`, Status: common.ChannelStatusAutoDisabled, MagpieAutoDisabledAt: time.Now().UnixNano()}
	require.NoError(t, channel.Insert())
	require.NoError(t, client.Set(context.Background(), magpieProbeCacheKey(channel), time.Now().Add(4*time.Minute).Format(time.RFC3339Nano), 4*time.Minute).Err())
	// 定时任务仍跳过未过期缓存。
	task, err := model.CreateSystemTask(model.SystemTaskTypeMagpieChannelMonitor, nil, nil)
	require.NoError(t, err)
	claimed, ok, err := model.ClaimSystemTask(task.ID, task.Type, "scheduled", common.GetTimestamp()+60)
	require.NoError(t, err)
	require.True(t, ok)
	magpieChannelMonitorHandler{}.Run(context.Background(), claimed, "scheduled")
	assert.Zero(t, calls)
	stored, err := model.GetSystemTaskByTaskID(task.TaskID)
	require.NoError(t, err)
	var result magpieChannelMonitorResult
	require.NoError(t, common.UnmarshalJsonStr(stored.Result, &result))
	assert.Equal(t, 1, result.Skipped)
	// 官方提前恢复后，手动执行必须查询并恢复，而不是继续等待缓存。
	task, err = model.CreateSystemTask(model.SystemTaskTypeMagpieChannelMonitor, map[string]bool{"manual": true}, nil)
	require.NoError(t, err)
	claimed, ok, err = model.ClaimSystemTask(task.ID, task.Type, "manual", common.GetTimestamp()+60)
	require.NoError(t, err)
	require.True(t, ok)
	magpieChannelMonitorHandler{}.Run(context.Background(), claimed, "manual")
	stored, err = model.GetSystemTaskByTaskID(task.TaskID)
	require.NoError(t, err)
	require.NoError(t, common.UnmarshalJsonStr(stored.Result, &result))
	assert.Equal(t, model.SystemTaskStatusSucceeded, stored.Status)
	assert.Equal(t, 2, calls)
	assert.Equal(t, 1, result.Enabled)
	assert.Zero(t, result.Skipped)
	assert.Equal(t, 1, notices)
	updated, err := model.GetChannelById(channel.Id, true)
	require.NoError(t, err)
	assert.Equal(t, common.ChannelStatusEnabled, updated.Status)
}

func TestMagpieProbeCacheTTLUsesBeijingDayAndNight(t *testing.T) {
	for _, test := range []struct {
		at  string
		ttl time.Duration
	}{
		{"2026-10-15T00:59:59Z", 30 * time.Minute},
		{"2026-10-15T01:00:00Z", 5 * time.Minute},
		{"2026-10-15T13:59:59Z", 5 * time.Minute},
		{"2026-10-15T14:00:00Z", 30 * time.Minute},
	} {
		t.Run(test.at, func(t *testing.T) {
			now, err := time.Parse(time.RFC3339, test.at)
			require.NoError(t, err)
			assert.Equal(t, test.ttl, magpieChannelProbeCacheTTL(now))
		})
	}
}
