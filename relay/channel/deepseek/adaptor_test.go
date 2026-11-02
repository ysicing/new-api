package deepseek

import (
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relay/helper"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/setting/model_setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDeepSeekThinkingAcrossProtocols(t *testing.T) {
	for _, test := range []struct{ model, upstream, thinking, effort, logged string }{
		{"deepseek-flash-none", "deepseek-flash", "disabled", "", "none"},
		{"deepseek-flash-low", "deepseek-flash", "enabled", "low", "low"},
		{"deepseek-v4-pro-max", "deepseek-v4-pro", "enabled", "max", "max"},
		{"deepseek-flash@thinking:on", "deepseek-flash", "enabled", "", "high"},
		{"deepseek-flash@thinking:off", "deepseek-flash", "disabled", "", "none"},
		{"deepseek-flash@thinking:0", "deepseek-flash", "disabled", "", "none"},
		{"deepseek-flash@thinking:8192", "deepseek-flash", "enabled", "medium", "high"},
		{"deepseek-flash@thinking:adaptive", "deepseek-flash", "enabled", "", "high"},
		{"deepseek-flash-none@effort:max", "deepseek-flash", "enabled", "max", "max"},
		{"deepseek-v4-pro@temperature:0.3@effort:low", "deepseek-v4-pro", "enabled", "low", "low"},
	} {
		t.Run(test.model, func(t *testing.T) {
			info := &relaycommon.RelayInfo{OriginModelName: test.model, ChannelMeta: &relaycommon.ChannelMeta{UpstreamModelName: test.model}}
			chat := &dto.GeneralOpenAIRequest{Model: test.model, ReasoningEffort: "high", THINKING: []byte(`{"type":"disabled"}`)}
			_, err := (&Adaptor{}).ConvertOpenAIRequest(nil, info, chat)
			require.NoError(t, err)
			assert.Equal(t, test.upstream, chat.Model)
			assert.JSONEq(t, `{"type":"`+test.thinking+`"}`, string(chat.THINKING))
			assert.Equal(t, test.effort, chat.ReasoningEffort)
			assert.Equal(t, test.logged, info.ReasoningEffort)
			if chat.Temperature != nil {
				assert.Equal(t, 0.3, *chat.Temperature)
			}

			info.UpstreamModelName = test.model
			claude := &dto.ClaudeRequest{Model: test.model, MaxTokens: common.GetPointer[uint](64), Temperature: common.GetPointer(0.7),
				OutputConfig: []byte(`{"effort":"high","format":{"type":"json_schema"}}`)}
			_, err = (&Adaptor{}).ConvertClaudeRequest(nil, info, claude)
			require.NoError(t, err)
			assert.Equal(t, test.upstream, claude.Model)
			require.NotNil(t, claude.Thinking)
			assert.Equal(t, test.thinking, claude.Thinking.Type)
			assert.Nil(t, claude.Thinking.BudgetTokens)
			assert.Equal(t, uint(64), *claude.MaxTokens)
			var config map[string]any
			require.NoError(t, common.Unmarshal(claude.OutputConfig, &config))
			assert.Contains(t, config, "format")
			if test.effort == "" {
				assert.NotContains(t, config, "effort")
			} else {
				assert.Equal(t, test.effort, config["effort"])
			}

			info.UpstreamModelName = test.model
			value, err := (&Adaptor{}).ConvertOpenAIResponsesRequest(nil, info, dto.OpenAIResponsesRequest{Model: test.model, Reasoning: &dto.Reasoning{Effort: "high"}})
			require.NoError(t, err)
			responses := value.(dto.OpenAIResponsesRequest)
			assert.Equal(t, test.upstream, responses.Model)
			effort := test.effort
			if test.thinking == "disabled" {
				effort = "none"
			}
			assert.Equal(t, effort, responses.Reasoning.Effort)
		})
	}
}

func TestDeepSeekMappingPreservesModifiers(t *testing.T) {
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Set("model_mapping", `{"deepseek-flash":"deepseek-v4-pro"}`)
	info := &relaycommon.RelayInfo{OriginModelName: "deepseek-flash@effort:max", ChannelMeta: &relaycommon.ChannelMeta{UpstreamModelName: "deepseek-flash@effort:max"}}
	request := &dto.GeneralOpenAIRequest{Model: info.OriginModelName}
	require.NoError(t, helper.ModelMappedHelper(ctx, info, request))
	_, err := (&Adaptor{}).ConvertOpenAIRequest(nil, info, request)
	require.NoError(t, err)
	assert.Equal(t, "deepseek-v4-pro", request.Model)
	assert.Equal(t, "max", request.ReasoningEffort)
}

func TestDeepSeekPreservesBlacklistedAndBareModelFields(t *testing.T) {
	settings := model_setting.GetGlobalSettings()
	original := settings.ThinkingModelBlacklist
	settings.ThinkingModelBlacklist = []string{"deepseek-flash-max"}
	t.Cleanup(func() { settings.ThinkingModelBlacklist = original })
	for _, model := range []string{"deepseek-flash", "deepseek-flash-max"} {
		info := &relaycommon.RelayInfo{OriginModelName: model}
		request := &dto.GeneralOpenAIRequest{Model: model, ReasoningEffort: "low", THINKING: []byte(`{"type":"disabled"}`)}
		_, err := (&Adaptor{}).ConvertOpenAIRequest(nil, info, request)
		require.NoError(t, err)
		assert.Equal(t, model, request.Model)
		assert.Equal(t, "low", request.ReasoningEffort)
		assert.JSONEq(t, `{"type":"disabled"}`, string(request.THINKING))
	}
}

func TestDeepSeekTemperatureOnlyModifierPreservesReasoningAndExplicitZero(t *testing.T) {
	modelName := "deepseek-flash@temperature:0"
	info := &relaycommon.RelayInfo{OriginModelName: modelName,
		ChannelMeta: &relaycommon.ChannelMeta{UpstreamModelName: modelName}}
	chat := &dto.GeneralOpenAIRequest{Model: modelName, ReasoningEffort: "low", THINKING: []byte(`{"type":"enabled"}`)}
	_, err := (&Adaptor{}).ConvertOpenAIRequest(nil, info, chat)
	require.NoError(t, err)
	assert.Equal(t, "deepseek-flash", info.UpstreamModelName)
	assert.Equal(t, "low", chat.ReasoningEffort)
	assert.JSONEq(t, `{"type":"enabled"}`, string(chat.THINKING))
	encoded, err := common.Marshal(chat)
	require.NoError(t, err)
	assert.Contains(t, string(encoded), `"temperature":0`)
	assert.Contains(t, string(encoded), `"model":"deepseek-flash"`)

	info.UpstreamModelName = modelName
	claude := &dto.ClaudeRequest{Model: modelName, Thinking: &dto.Thinking{Type: "enabled"},
		OutputConfig: []byte(`{"effort":"low"}`)}
	_, err = (&Adaptor{}).ConvertClaudeRequest(nil, info, claude)
	require.NoError(t, err)
	assert.Equal(t, "deepseek-flash", info.UpstreamModelName)
	assert.Equal(t, "enabled", claude.Thinking.Type)
	assert.JSONEq(t, `{"effort":"low"}`, string(claude.OutputConfig))
	require.NotNil(t, claude.Temperature)
	assert.Zero(t, *claude.Temperature)

	info.UpstreamModelName = modelName
	value, err := (&Adaptor{}).ConvertOpenAIResponsesRequest(nil, info,
		dto.OpenAIResponsesRequest{Model: modelName, Reasoning: &dto.Reasoning{Effort: "low"}})
	require.NoError(t, err)
	response := value.(dto.OpenAIResponsesRequest)
	assert.Equal(t, "deepseek-flash", info.UpstreamModelName)
	assert.Equal(t, "low", response.Reasoning.Effort)
	require.NotNil(t, response.Temperature)
	assert.Zero(t, *response.Temperature)
}
