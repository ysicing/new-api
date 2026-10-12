package model

import (
	"context"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMagpieChannelStatusAndAbilities(t *testing.T) {
	setupChannelStatusTest(t)
	channel := Channel{Name: "Magpie", Key: "gateway-key", OtherSettings: `{"magpie_enabled":true}`, Models: "gpt-model", Group: "default"}
	require.NoError(t, channel.Insert())
	disabled, err := SyncMagpieChannelStatus(context.Background(), &channel, false)
	require.NoError(t, err)
	require.NotNil(t, disabled)
	assert.Equal(t, common.ChannelStatusAutoDisabled, disabled.Status)
	assert.Positive(t, disabled.MagpieAutoDisabledAt)
	var ability Ability
	require.NoError(t, DB.Where("channel_id = ?", channel.Id).First(&ability).Error)
	assert.False(t, ability.Enabled)
	duplicate, err := SyncMagpieChannelStatus(context.Background(), disabled, false)
	require.NoError(t, err)
	assert.Nil(t, duplicate)
	assert.False(t, UpdateChannelStatusAutomatically(channel.Id, "", common.ChannelStatusEnabled, "health test"))
	assert.False(t, UpdateChannelStatusAutomatically(channel.Id, "", common.ChannelStatusAutoDisabled, "generic error"))
	// 持久化读取模拟进程重启，仍能依据查询结果恢复。
	snapshot, err := GetChannelById(channel.Id, true)
	require.NoError(t, err)
	enabled, err := SyncMagpieChannelStatus(context.Background(), snapshot, true)
	require.NoError(t, err)
	require.NotNil(t, enabled)
	assert.Equal(t, common.ChannelStatusEnabled, enabled.Status)
	assert.Zero(t, enabled.MagpieAutoDisabledAt)
	require.NoError(t, DB.Where("channel_id = ?", channel.Id).First(&ability).Error)
	assert.True(t, ability.Enabled)
}

func TestMagpieChannelDoesNotOverrideManualOrStaleResults(t *testing.T) {
	for _, action := range []string{"manual disable", "manual enable", "switch off", "rotate key", "change URL", "other disabled"} {
		t.Run(action, func(t *testing.T) {
			setupChannelStatusTest(t)
			channel := Channel{Name: "Magpie", Key: "old-key", BaseURL: common.GetPointer("https://magpie.example"), OtherSettings: `{"magpie_enabled":true}`}
			require.NoError(t, DB.Create(&channel).Error)
			disabled, err := SyncMagpieChannelStatus(context.Background(), &channel, false)
			require.NoError(t, err)
			require.NotNil(t, disabled)
			switch action {
			case "manual disable":
				require.True(t, UpdateChannelStatus(channel.Id, "", common.ChannelStatusManuallyDisabled, "manual"))
			case "manual enable":
				require.True(t, UpdateChannelStatus(channel.Id, "", common.ChannelStatusEnabled, "manual"))
			case "switch off":
				require.NoError(t, (&Channel{Id: channel.Id, OtherSettings: `{"magpie_enabled":false}`}).Update())
			case "rotate key":
				require.NoError(t, DB.Model(&Channel{}).Where("id = ?", channel.Id).Update("key", "new-key").Error)
			case "change URL":
				require.NoError(t, DB.Model(&Channel{}).Where("id = ?", channel.Id).Update("base_url", "https://other.example").Error)
			case "other disabled":
				require.True(t, UpdateChannelStatus(channel.Id, "", common.ChannelStatusAutoDisabled, "other"))
			}
			before, err := GetChannelById(channel.Id, true)
			require.NoError(t, err)
			changed, err := SyncMagpieChannelStatus(context.Background(), disabled, true)
			require.NoError(t, err)
			assert.Nil(t, changed)
			after, err := GetChannelById(channel.Id, true)
			require.NoError(t, err)
			assert.Equal(t, before.Status, after.Status)
			assert.Equal(t, before.MagpieAutoDisabledAt, after.MagpieAutoDisabledAt)
		})
	}
}

func TestListMagpieChannelsExcludesOptOutAndManualDisabling(t *testing.T) {
	setupChannelStatusTest(t)
	for _, channel := range []Channel{
		{Name: "ordinary", Key: "key"},
		{Name: "manual", Key: "key", Status: common.ChannelStatusManuallyDisabled, OtherSettings: `{"magpie_enabled":true}`},
		{Name: "other-auto-disabled", Key: "key", Status: common.ChannelStatusAutoDisabled, OtherSettings: `{"magpie_enabled":true}`},
		{Name: "monitored", Key: "key", OtherSettings: `{"magpie_enabled":true}`},
		{Name: "monitor-disabled", Key: "key", Status: common.ChannelStatusAutoDisabled, MagpieAutoDisabledAt: 123, OtherSettings: `{"magpie_enabled":true}`},
	} {
		require.NoError(t, DB.Create(&channel).Error)
	}
	channels, err := ListMagpieChannels(context.Background())
	require.NoError(t, err)
	require.Len(t, channels, 2)
	assert.Equal(t, "monitored", channels[0].Name)
	assert.Equal(t, "monitor-disabled", channels[1].Name)
}

func TestMagpieMonitorKeepsMultiKeyStateAndIgnoresPollingCursorChanges(t *testing.T) {
	setupChannelStatusTest(t)
	channel := Channel{Name: "multi", Key: "disabled\nvalid", OtherSettings: `{"magpie_enabled":true}`, ChannelInfo: ChannelInfo{IsMultiKey: true, MultiKeySize: 2, MultiKeyStatusList: map[int]int{0: common.ChannelStatusManuallyDisabled}}}
	require.NoError(t, DB.Create(&channel).Error)
	info := channel.ChannelInfo
	info.MultiKeyPollingIndex = 1
	require.NoError(t, DB.Model(&Channel{}).Where("id = ?", channel.Id).Update("channel_info", info).Error)
	disabled, err := SyncMagpieChannelStatus(context.Background(), &channel, false)
	require.NoError(t, err)
	require.NotNil(t, disabled)
	assert.Equal(t, 1, disabled.ChannelInfo.MultiKeyPollingIndex)
	restored, err := SyncMagpieChannelStatus(context.Background(), disabled, true)
	require.NoError(t, err)
	require.NotNil(t, restored)
	assert.Equal(t, common.ChannelStatusManuallyDisabled, restored.ChannelInfo.MultiKeyStatusList[0])
}

func TestMagpieRecoveryImmediatelyRestoresCachedRouting(t *testing.T) {
	setupChannelStatusTest(t)
	channelSyncLock.Lock()
	previousGroups, previousChannels, previousCustom := group2model2channels, channelsIDM, channel2advancedCustomConfig
	channelSyncLock.Unlock()
	common.MemoryCacheEnabled = true
	t.Cleanup(func() {
		common.MemoryCacheEnabled = false
		channelSyncLock.Lock()
		group2model2channels, channelsIDM, channel2advancedCustomConfig = previousGroups, previousChannels, previousCustom
		channelSyncLock.Unlock()
	})
	channel := Channel{Name: "routing", Type: 1, Key: "key-a\nkey-b", Models: "gpt-magpie", Group: "default", OtherSettings: `{"magpie_enabled":true}`, ChannelInfo: ChannelInfo{IsMultiKey: true, MultiKeySize: 2, MultiKeyMode: constant.MultiKeyModePolling}}
	require.NoError(t, channel.Insert())
	InitChannelCache()
	cached, err := CacheGetChannel(channel.Id)
	require.NoError(t, err)
	key, _, keyErr := cached.GetNextEnabledKey()
	require.Nil(t, keyErr)
	assert.Equal(t, "key-a", key)
	disabled, err := SyncMagpieChannelStatus(context.Background(), &channel, false)
	require.NoError(t, err)
	require.NotNil(t, disabled)
	selected, err := GetRandomSatisfiedChannel("default", "gpt-magpie", 0, "/v1/responses")
	require.NoError(t, err)
	assert.Nil(t, selected)
	restored, err := SyncMagpieChannelStatus(context.Background(), disabled, true)
	require.NoError(t, err)
	require.NotNil(t, restored)
	selected, err = GetRandomSatisfiedChannel("default", "gpt-magpie", 0, "/v1/responses")
	require.NoError(t, err)
	require.NotNil(t, selected)
	assert.Equal(t, channel.Id, selected.Id)
	// 内存模式的轮询游标尚未落库；启停后应继续轮询，不能被旧数据库游标覆盖。
	key, _, keyErr = selected.GetNextEnabledKey()
	require.Nil(t, keyErr)
	assert.Equal(t, "key-b", key)
}
