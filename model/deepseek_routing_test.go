package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDeepSeekModifiersRouteWithAndWithoutCache(t *testing.T) {
	truncateTables(t)
	original := common.MemoryCacheEnabled
	t.Cleanup(func() { common.MemoryCacheEnabled = original; InitChannelCache() })
	channel := Channel{Id: 930001, Type: constant.ChannelTypeDeepSeek, Status: common.ChannelStatusEnabled,
		Name: "deepseek", Models: "deepseek-flash", Group: "default"}
	require.NoError(t, channel.Insert())
	for _, cached := range []bool{false, true} {
		common.MemoryCacheEnabled = cached
		InitChannelCache()
		for _, retry := range []int{0, 1} {
			selected, err := GetRandomSatisfiedChannel("default", "deepseek-flash@effort:max", retry, "/v1/chat/completions")
			require.NoError(t, err)
			require.NotNil(t, selected)
			assert.Equal(t, channel.Id, selected.Id)
			selected, err = GetRandomSatisfiedChannel("default", "deepseek-v4-pro@effort:max", 0, "/v1/chat/completions")
			require.NoError(t, err)
			assert.Nil(t, selected)
		}
	}
}

func TestDeepSeekRoutingFallsBackAfterAdvancedCustomPathFiltering(t *testing.T) {
	truncateTables(t)
	original := common.MemoryCacheEnabled
	t.Cleanup(func() { common.MemoryCacheEnabled = original; InitChannelCache() })
	modelName := "deepseek-flash@effort:max"
	for _, test := range []struct {
		id          int
		model, path string
		priority    int64
	}{
		{930101, modelName, "/v1/messages", 30},
		{930102, "deepseek-flash", "/v1/chat/completions", 20},
		{930103, "deepseek-flash", "/v1/chat/completions", 10},
	} {
		channel := Channel{Id: test.id, Type: constant.ChannelTypeAdvancedCustom,
			Status: common.ChannelStatusEnabled, Name: "deepseek-route",
			Models: test.model, Group: "default", Priority: &test.priority}
		channel.SetOtherSettings(dto.ChannelOtherSettings{AdvancedCustom: &dto.AdvancedCustomConfig{
			Routes: []dto.AdvancedCustomRoute{{
				IncomingPath: test.path, UpstreamPath: test.path, Converter: "none", Models: []string{modelName},
			}},
		}})
		require.NoError(t, channel.Insert())
	}
	for _, cached := range []bool{false, true} {
		common.MemoryCacheEnabled = cached
		InitChannelCache()
		for _, test := range []struct{ retry, id int }{{0, 930102}, {1, 930103}, {5, 930103}} {
			selected, err := GetRandomSatisfiedChannel("default", modelName, test.retry, "/v1/chat/completions")
			require.NoError(t, err)
			require.NotNil(t, selected)
			assert.Equal(t, test.id, selected.Id)
		}
		selected, err := GetRandomSatisfiedChannel("default", modelName, 0, "/v1/messages")
		require.NoError(t, err)
		require.NotNil(t, selected)
		assert.Equal(t, 930101, selected.Id)
	}
}
