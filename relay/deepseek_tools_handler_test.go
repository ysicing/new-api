package relay

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relay/helper"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type deepSeekSettlement struct {
	settled  []int
	refunded bool
}

func (settlement *deepSeekSettlement) Settle(quota int) error {
	settlement.settled = append(settlement.settled, quota)
	return nil
}
func (settlement *deepSeekSettlement) Refund(*gin.Context) { settlement.refunded = true }
func (*deepSeekSettlement) NeedsRefund() bool              { return false }
func (*deepSeekSettlement) GetPreConsumedQuota() int       { return 375 }
func (*deepSeekSettlement) Reserve(int) error              { return nil }

func TestDeepSeekToolsRelayAndSettlement(t *testing.T) {
	service.InitHttpClient()
	gin.SetMode(gin.TestMode)
	batch, logConsume := common.BatchUpdateEnabled, common.LogConsumeEnabled
	common.BatchUpdateEnabled, common.LogConsumeEnabled = true, false
	t.Cleanup(func() { common.BatchUpdateEnabled, common.LogConsumeEnabled = batch, logConsume })
	for _, test := range []struct {
		name, path, body, response string
		channelType, status        int
		settled                    []int
		refunded                   bool
	}{
		{"search", "/v1/tools/search", `{"query":"deepseek","future":{"nested":true}}`, `{"results":[{"title":"DeepSeek"}]}`, constant.ChannelTypeDeepSeek, 200, []int{375}, false},
		{"empty search", "/v1/tools/search", `{"query":"none"}`, `{"results":[]}`, constant.ChannelTypeDeepSeek, 200, nil, true},
		{"fetch", "/v1/tools/fetch", `{"url":"https://example.com"}`, `{"content":"page"}`, constant.ChannelTypeDeepSeek, 200, []int{375}, false},
		{"empty fetch", "/v1/tools/fetch", `{"url":"https://example.com"}`, `{"content":""}`, constant.ChannelTypeDeepSeek, 200, nil, true},
		{"fetch error", "/v1/tools/fetch", `{"url":"https://example.com"}`, `{"error":{"message":"missing","type":"not_found_error"}}`, constant.ChannelTypeDeepSeek, 404, nil, false},
		{"200 error", "/v1/tools/fetch", `{"url":"https://example.com"}`, `{"error":"failed","content":"error"}`, constant.ChannelTypeDeepSeek, 200, nil, true},
		{"new api gateway", "/v1/tools/search", `{"query":"deepseek","model":"free-model"}`, `{"results":[{"title":"DeepSeek"}]}`, constant.ChannelTypeNewAPI, 200, []int{375}, false},
		{"unsupported channel", "/v1/tools/search", `{"query":"deepseek"}`, "", constant.ChannelTypeOpenAI, 0, nil, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			var gotPath, gotBody string
			upstream := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
				gotPath = request.URL.Path
				body, err := io.ReadAll(request.Body)
				assert.NoError(t, err)
				gotBody = string(body)
				writer.Header().Set("Content-Type", "application/json")
				writer.WriteHeader(test.status)
				_, _ = io.WriteString(writer, test.response)
			}))
			defer upstream.Close()
			recorder := httptest.NewRecorder()
			ctx, _ := gin.CreateTestContext(recorder)
			ctx.Request = httptest.NewRequest(http.MethodPost, test.path, strings.NewReader(test.body))
			ctx.Request.Header.Set("Content-Type", "application/json")
			modelName := constant.DeepSeekSearchModel
			if test.path == "/v1/tools/fetch" {
				modelName = constant.DeepSeekFetchModel
			}
			common.SetContextKey(ctx, constant.ContextKeyOriginalModel, modelName)
			common.SetContextKey(ctx, constant.ContextKeyUsingGroup, "default")
			common.SetContextKey(ctx, constant.ContextKeyUserGroup, "default")
			common.SetContextKey(ctx, constant.ContextKeyChannelType, test.channelType)
			common.SetContextKey(ctx, constant.ContextKeyChannelBaseUrl, upstream.URL)
			common.SetContextKey(ctx, constant.ContextKeyChannelKey, "test")
			request, err := helper.GetAndValidateRequest(ctx, types.RelayFormatDeepSeekTools)
			require.NoError(t, err)
			info, err := relaycommon.GenRelayInfo(ctx, types.RelayFormatDeepSeekTools, request, nil)
			require.NoError(t, err)
			price, err := helper.ModelPriceHelper(ctx, info, 0, request.GetTokenCountMeta())
			require.NoError(t, err)
			assert.Equal(t, 375, price.QuotaToPreConsume)
			settlement := &deepSeekSettlement{}
			info.Billing = settlement
			apiError := DeepSeekToolsHelper(ctx, info)
			if test.status == 0 {
				require.NotNil(t, apiError)
				assert.False(t, types.IsSkipRetryError(apiError))
				assert.Empty(t, gotPath)
				return
			}
			assert.Equal(t, test.path, gotPath)
			assert.JSONEq(t, test.body, gotBody)
			assert.Equal(t, test.settled, settlement.settled)
			assert.Equal(t, test.refunded, settlement.refunded)
			if test.status != 200 {
				require.NotNil(t, apiError)
				assert.Equal(t, test.status, apiError.StatusCode)
				return
			}
			require.Nil(t, apiError)
			assert.Equal(t, test.response, recorder.Body.String())
		})
	}
}
