package deepseek

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/relay/channel"
	"github.com/QuantumNous/new-api/relay/channel/claude"
	"github.com/QuantumNous/new-api/relay/channel/openai"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/relaykit/dto"
	kitreasoning "github.com/QuantumNous/new-api/relaykit/relayconvert/reasoning"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/QuantumNous/new-api/setting/model_setting"
	"github.com/QuantumNous/new-api/setting/reasoning"
	"github.com/gin-gonic/gin"
)

type Adaptor struct {
}

func (a *Adaptor) ConvertGeminiRequest(*gin.Context, *relaycommon.RelayInfo, *dto.GeminiChatRequest) (any, error) {
	//TODO implement me
	return nil, errors.New("not implemented")
}

func (a *Adaptor) ConvertClaudeRequest(c *gin.Context, info *relaycommon.RelayInfo, req *dto.ClaudeRequest) (any, error) {
	adaptor := claude.Adaptor{}
	convertedRequest, err := adaptor.ConvertClaudeRequest(c, info, req)
	if err != nil {
		return nil, err
	}
	claudeRequest, ok := convertedRequest.(*dto.ClaudeRequest)
	if !ok {
		return convertedRequest, nil
	}
	if err := applyDeepSeekV4ClaudeThinkingSuffix(info, claudeRequest); err != nil {
		return nil, err
	}
	return claudeRequest, nil
}

func (a *Adaptor) ConvertAudioRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.AudioRequest) (io.Reader, error) {
	//TODO implement me
	return nil, errors.New("not implemented")
}

func (a *Adaptor) ConvertImageRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.ImageRequest) (any, error) {
	//TODO implement me
	return nil, errors.New("not implemented")
}

func (a *Adaptor) Init(info *relaycommon.RelayInfo) {
	info.WebSearchBillingKey = SearchToolPriceKey
	info.WebFetchBillingKey = FetchToolPriceKey
	info.VendorToolUsage = (&responsesToolCounter{}).Observe
}

func (a *Adaptor) GetRequestURL(info *relaycommon.RelayInfo) (string, error) {
	fimBaseUrl := info.ChannelBaseUrl
	switch info.RelayFormat {
	case types.RelayFormatClaude:
		return fmt.Sprintf("%s/anthropic/v1/messages", info.ChannelBaseUrl), nil
	default:
		if !strings.HasSuffix(info.ChannelBaseUrl, "/beta") {
			fimBaseUrl += "/beta"
		}
		switch info.RelayMode {
		case constant.RelayModeCompletions:
			return fmt.Sprintf("%s/completions", fimBaseUrl), nil
		case constant.RelayModeResponses:
			return fmt.Sprintf("%s/responses", info.ChannelBaseUrl), nil
		case constant.RelayModeToolsSearch:
			return fmt.Sprintf("%s/v1/tools/search", info.ChannelBaseUrl), nil
		case constant.RelayModeToolsFetch:
			return fmt.Sprintf("%s/v1/tools/fetch", info.ChannelBaseUrl), nil
		default:
			return fmt.Sprintf("%s/v1/chat/completions", info.ChannelBaseUrl), nil
		}
	}
}

func (a *Adaptor) SetupRequestHeader(c *gin.Context, req *http.Header, info *relaycommon.RelayInfo) error {
	channel.SetupApiRequestHeader(info, c, req)
	req.Set("Authorization", "Bearer "+info.ApiKey)
	return nil
}

func (a *Adaptor) ConvertOpenAIRequest(c *gin.Context, info *relaycommon.RelayInfo, request *dto.GeneralOpenAIRequest) (any, error) {
	if request == nil {
		return nil, errors.New("request is nil")
	}
	if err := applyDeepSeekV4OpenAIThinkingSuffix(info, request); err != nil {
		return nil, err
	}

	return request, nil
}

func applyDeepSeekV4OpenAIThinkingSuffix(info *relaycommon.RelayInfo, request *dto.GeneralOpenAIRequest) error {
	modelName := request.Model
	if info != nil && info.ChannelMeta != nil && info.UpstreamModelName != "" {
		modelName = info.UpstreamModelName
	}
	modifiers, err := deepSeekThinking(info, modelName)
	if err != nil {
		return err
	}
	request.Model = modifiers.Model
	if info != nil && info.ChannelMeta != nil {
		info.UpstreamModelName = modifiers.Model
	}
	if modifiers.Temperature != nil {
		request.Temperature = modifiers.Temperature
	}
	baseModel, thinkingType, effort := modifiers.Model, modifiers.Thinking, modifiers.Effort
	ok := thinkingType != ""
	if !ok {
		return nil
	}
	thinking, err := common.Marshal(map[string]string{
		"type": thinkingType,
	})
	if err != nil {
		return fmt.Errorf("error marshalling thinking: %w", err)
	}
	request.Model = baseModel
	request.THINKING = thinking
	request.ReasoningEffort = effort
	request.Reasoning = nil
	if info != nil {
		info.SetReasoningEffort(deepSeekRunEffort(thinkingType, effort))
	}
	return nil
}

func applyDeepSeekV4ClaudeThinkingSuffix(info *relaycommon.RelayInfo, request *dto.ClaudeRequest) error {
	modelName := request.Model
	if info != nil && info.ChannelMeta != nil && info.UpstreamModelName != "" {
		modelName = info.UpstreamModelName
	}
	modifiers, err := deepSeekThinking(info, modelName)
	if err != nil {
		return err
	}
	request.Model = modifiers.Model
	if info != nil && info.ChannelMeta != nil {
		info.UpstreamModelName = modifiers.Model
	}
	if modifiers.Temperature != nil {
		request.Temperature = modifiers.Temperature
	}
	baseModel, thinkingType, effort := modifiers.Model, modifiers.Thinking, modifiers.Effort
	if thinkingType == "" {
		return nil
	}
	request.Model = baseModel
	request.Thinking = &dto.Thinking{Type: thinkingType}
	outputConfig := make(map[string]any)
	if len(request.OutputConfig) > 0 {
		if err := common.Unmarshal(request.OutputConfig, &outputConfig); err != nil || outputConfig == nil {
			return fmt.Errorf("invalid output_config")
		}
	}
	delete(outputConfig, "effort")
	if effort != "" {
		outputConfig["effort"] = effort
	}
	request.OutputConfig = nil
	if len(outputConfig) > 0 {
		encoded, err := common.Marshal(outputConfig)
		if err != nil {
			return fmt.Errorf("error marshalling output_config: %w", err)
		}
		request.OutputConfig = encoded
	}
	if info != nil {
		info.SetReasoningEffort(deepSeekRunEffort(thinkingType, effort))
	}
	return nil
}

func (a *Adaptor) ConvertRerankRequest(c *gin.Context, relayMode int, request dto.RerankRequest) (any, error) {
	return nil, nil
}

func (a *Adaptor) ConvertEmbeddingRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.EmbeddingRequest) (any, error) {
	//TODO implement me
	return nil, errors.New("not implemented")
}

func (a *Adaptor) ConvertOpenAIResponsesRequest(_ *gin.Context, info *relaycommon.RelayInfo, request dto.OpenAIResponsesRequest) (any, error) {
	if err := applyDeepSeekV4ResponsesThinkingSuffix(info, &request); err != nil {
		return nil, err
	}
	return request, nil
}

func applyDeepSeekV4ResponsesThinkingSuffix(info *relaycommon.RelayInfo, request *dto.OpenAIResponsesRequest) error {
	modelName := request.Model
	if info != nil && info.ChannelMeta != nil && info.UpstreamModelName != "" {
		modelName = info.UpstreamModelName
	}
	modifiers, err := deepSeekThinking(info, modelName)
	if err != nil {
		return err
	}
	request.Model = modifiers.Model
	if info != nil && info.ChannelMeta != nil {
		info.UpstreamModelName = modifiers.Model
	}
	if modifiers.Temperature != nil {
		request.Temperature = modifiers.Temperature
	}
	baseModel, thinkingType, effort := modifiers.Model, modifiers.Thinking, modifiers.Effort
	if thinkingType != "" {
		if thinkingType == "disabled" {
			effort = "none"
		}
		request.Model = baseModel
		if request.Reasoning == nil && effort != "" {
			request.Reasoning = &dto.Reasoning{}
		}
		if request.Reasoning != nil {
			request.Reasoning.Effort = effort
		}
		if info != nil {
			info.SetReasoningEffort(deepSeekRunEffort(thinkingType, effort))
		}
		return nil
	}
	if info != nil && request.Reasoning != nil {
		info.SetReasoningEffort(request.Reasoning.Effort)
	}
	return nil
}

func deepSeekThinking(info *relaycommon.RelayInfo, modelName string) (kitreasoning.DeepSeekModifiers, error) {
	if model_setting.ShouldPreserveThinkingSuffix(modelName) ||
		info != nil && model_setting.ShouldPreserveThinkingSuffix(info.OriginModelName) {
		return kitreasoning.DeepSeekModifiers{Model: modelName}, nil
	}
	modifiers, err := kitreasoning.ParseDeepSeekModifiers(modelName)
	if err != nil {
		return modifiers, err
	}
	if info != nil && info.OriginModelName != modelName && strings.Contains(info.OriginModelName, "@") {
		original, err := kitreasoning.ParseDeepSeekModifiers(info.OriginModelName)
		if err != nil {
			return modifiers, err
		}
		if original.Thinking != "" {
			modifiers.Thinking, modifiers.Effort = original.Thinking, original.Effort
		}
		if original.Temperature != nil {
			modifiers.Temperature = original.Temperature
		}
	}
	if base, thinking, effort, ok := reasoning.ParseDeepSeekV4ThinkingSuffix(modifiers.Model); ok {
		modifiers.Model = base
		if modifiers.Thinking == "" {
			modifiers.Thinking, modifiers.Effort = thinking, effort
		}
	}
	return modifiers, nil
}

func deepSeekRunEffort(thinking, effort string) string {
	switch {
	case thinking == "disabled":
		return "none"
	case effort == "" || effort == "medium" || effort == "xhigh":
		return "high"
	case effort == "minimal":
		return "low"
	default:
		return effort
	}
}

func (a *Adaptor) DoRequest(c *gin.Context, info *relaycommon.RelayInfo, requestBody io.Reader) (any, error) {
	return channel.DoApiRequest(a, c, info, requestBody)
}

func (a *Adaptor) DoResponse(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (usage any, err *types.NewAPIError) {
	switch info.RelayFormat {
	case types.RelayFormatClaude:
		adaptor := claude.Adaptor{}
		return adaptor.DoResponse(c, resp, info)
	default:
		adaptor := openai.Adaptor{}
		return adaptor.DoResponse(c, resp, info)
	}
}

func (a *Adaptor) GetModelList() []string {
	return ModelList
}

func (a *Adaptor) GetChannelName() string {
	return ChannelName
}
