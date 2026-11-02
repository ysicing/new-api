package relay

import (
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/QuantumNous/new-api/constant"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/relay/helper"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/QuantumNous/new-api/service"
	"github.com/tidwall/gjson"

	"github.com/gin-gonic/gin"
)

func DeepSeekToolsHelper(c *gin.Context, info *relaycommon.RelayInfo) *types.NewAPIError {
	info.InitChannelMeta(c)
	switch info.ChannelType {
	case constant.ChannelTypeDeepSeek, constant.ChannelTypeNewAPI,
		constant.ChannelTypeSub2API, constant.ChannelTypeAdvancedCustom:
	default:
		return types.NewError(fmt.Errorf("channel does not support %s", c.Request.URL.Path), types.ErrorCodeInvalidRequest)
	}
	request, ok := info.Request.(*dto.DeepSeekToolsRequest)
	if !ok {
		return types.NewErrorWithStatusCode(errors.New("invalid DeepSeek tools request"), types.ErrorCodeInvalidRequest,
			http.StatusBadRequest, types.ErrOptionWithSkipRetry())
	}
	if err := helper.ModelMappedHelper(c, info, request); err != nil {
		return types.NewError(err, types.ErrorCodeChannelModelMappedError, types.ErrOptionWithSkipRetry())
	}
	jsonData := []byte(request.RawBody)
	if len(info.ParamOverride) > 0 {
		var err error
		jsonData, err = relaycommon.ApplyParamOverrideWithRelayInfo(jsonData, info)
		if err != nil {
			return newAPIErrorFromParamOverride(err)
		}
	}
	body, closer, err := relaycommon.NewOutboundJSONBody(jsonData)
	if err != nil {
		return types.NewError(err, types.ErrorCodeConvertRequestFailed, types.ErrOptionWithSkipRetry())
	}
	defer closer.Close()
	adaptor := GetAdaptor(info.ApiType)
	if adaptor == nil {
		return types.NewError(fmt.Errorf("invalid api type: %d", info.ApiType), types.ErrorCodeInvalidApiType, types.ErrOptionWithSkipRetry())
	}
	adaptor.Init(info)
	response, err := adaptor.DoRequest(c, info, body)
	if err != nil {
		return types.NewOpenAIError(err, types.ErrorCodeDoRequestFailed, http.StatusInternalServerError)
	}
	httpResponse, ok := response.(*http.Response)
	if !ok || httpResponse == nil {
		return types.NewOpenAIError(errors.New("invalid http response"), types.ErrorCodeDoRequestFailed, http.StatusInternalServerError)
	}
	defer httpResponse.Body.Close()
	if httpResponse.StatusCode < 200 || httpResponse.StatusCode >= 300 {
		apiError := service.RelayErrorHandler(c.Request.Context(), httpResponse, false)
		service.ResetStatusCode(apiError, c.GetString("status_code_mapping"))
		return apiError
	}
	responseBody, err := io.ReadAll(httpResponse.Body)
	if err != nil {
		return types.NewOpenAIError(err, types.ErrorCodeReadResponseBodyFailed, http.StatusInternalServerError, types.ErrOptionWithSkipRetry())
	}
	service.IOCopyBytesGracefully(c, httpResponse, responseBody)
	if !deepSeekToolResponseBillable(info.RelayMode, responseBody) {
		if info.Billing != nil {
			info.Billing.Refund(c)
		}
		return nil
	}
	service.PostTextConsumeQuota(c, info, &dto.Usage{}, nil)
	return nil
}

func deepSeekToolResponseBillable(mode int, body []byte) bool {
	if !gjson.ValidBytes(body) || gjson.GetBytes(body, "error").Exists() {
		return false
	}
	if mode == relayconstant.RelayModeToolsFetch {
		content := gjson.GetBytes(body, "content")
		return content.Type == gjson.String && content.String() != ""
	}
	results := gjson.GetBytes(body, "results")
	return results.IsArray() && len(results.Array()) > 0
}
