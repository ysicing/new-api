package deepseek

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/relay/channel/openai"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDeepSeekResponsesToolBillingAcrossNativeAndConvertedHandlers(t *testing.T) {
	timeout := constant.StreamingTimeout
	constant.StreamingTimeout = 30
	t.Cleanup(func() { constant.StreamingTimeout = timeout })
	type handler func(*gin.Context, *relaycommon.RelayInfo, *http.Response) (*dto.Usage, *types.NewAPIError)
	for _, test := range []struct {
		name    string
		handler handler
		stream  bool
	}{
		{"responses", openai.OaiResponsesHandler, false},
		{"responses stream", openai.OaiResponsesStreamHandler, true},
		{"chat", openai.OaiResponsesToChatHandler, false},
		{"chat stream", openai.OaiResponsesToChatStreamHandler, true},
		{"chat buffered", openai.OaiResponsesToChatBufferedStreamHandler, true},
	} {
		for _, output := range []struct {
			name, items   string
			search, fetch int
		}{
			{"mixed", `[{"type":"web_search_call","id":"s","status":"completed","action":{"type":"search"}},{"type":"web_search_call","id":"f","status":"completed","action":{"type":"open_page"}},{"type":"web_search_call","id":"bad","status":"failed","action":{"type":"search"}}]`, 1, 1},
			{"failed tools", `[{"type":"web_search_call","id":"bad","status":"failed","action":{"type":"search"}}]`, 0, 0},
			{"no tools", `[{"type":"message","role":"assistant","content":[{"type":"output_text","text":"hello"}]}]`, 0, 0},
		} {
			t.Run(test.name+"/"+output.name, func(t *testing.T) {
				body := `{"id":"resp_ds","model":"deepseek-flash","status":"completed","output":` + output.items + `,"usage":{"input_tokens":1,"output_tokens":1,"total_tokens":2}}`
				if test.stream {
					body = "data: {\"type\":\"response.completed\",\"response\":" + body + "}\n\ndata: [DONE]\n\n"
				}
				ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
				ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
				ctx.Set(common.RequestIdKey, "deepseek-tools")
				info := &relaycommon.RelayInfo{
					OriginModelName: "deepseek-flash", RelayFormat: types.RelayFormatOpenAI,
					ChannelMeta: &relaycommon.ChannelMeta{UpstreamModelName: "deepseek-flash"},
					IsStream:    test.stream, DisablePing: true,
				}
				(&Adaptor{}).Init(info)
				resp := &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body)), Header: http.Header{"Content-Type": {"application/json"}}}
				usage, apiError := test.handler(ctx, info, resp)
				require.Nil(t, apiError)
				require.NotNil(t, usage)
				require.NotNil(t, info.ResponsesUsageInfo)
				require.Contains(t, info.BuiltInTools, SearchToolPriceKey)
				require.Contains(t, info.BuiltInTools, FetchToolPriceKey)
				assert.Equal(t, output.search, info.BuiltInTools[SearchToolPriceKey].CallCount)
				assert.Equal(t, output.fetch, info.BuiltInTools[FetchToolPriceKey].CallCount)
				if generic := info.BuiltInTools[dto.BuildInToolWebSearchPreview]; generic != nil {
					assert.Zero(t, generic.CallCount)
				}
			})
		}
	}
}

func TestDeepSeekResponsesStreamUsesCompletedItemsUntilAuthoritativeOutput(t *testing.T) {
	timeout := constant.StreamingTimeout
	constant.StreamingTimeout = 30
	t.Cleanup(func() { constant.StreamingTimeout = timeout })
	for _, test := range []struct {
		name, output  string
		search, fetch int
	}{
		{"empty final preserves completed calls", `[]`, 1, 1},
		{"complete output replaces provisional counts", `[{"type":"web_search_call","status":"completed","action":{"type":"open_page"}}]`, 0, 1},
		{"non-tool final clears provisional counts", `[{"type":"message"}]`, 0, 0},
	} {
		t.Run(test.name, func(t *testing.T) {
			ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
			ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
			info := &relaycommon.RelayInfo{OriginModelName: "deepseek-flash", IsStream: true, DisablePing: true,
				ChannelMeta: &relaycommon.ChannelMeta{UpstreamModelName: "deepseek-flash"}}
			(&Adaptor{}).Init(info)
			search := `data: {"type":"response.output_item.done","item":{"id":"s","type":"web_search_call","status":"completed","action":{"type":"search"}}}`
			body := strings.Join([]string{
				search, "", search, "",
				`data: {"type":"response.output_item.done","item":{"id":"bad","type":"web_search_call","status":"failed","action":{"type":"search"}}}`, "",
				`data: {"type":"response.output_item.done","item":{"id":"f","type":"web_search_call","status":"completed","action":{"type":"open_page"}}}`, "",
				`data: {"type":"response.completed","response":{"output":` + test.output + `,"usage":{"input_tokens":1,"output_tokens":1,"total_tokens":2}}}`, "",
				"data: [DONE]", "", "",
			}, "\n")
			resp := &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body)), Header: http.Header{"Content-Type": {"text/event-stream"}}}
			_, apiError := openai.OaiResponsesStreamHandler(ctx, info, resp)
			require.Nil(t, apiError)
			require.NotNil(t, info.ResponsesUsageInfo)
			assert.Equal(t, test.search, info.BuiltInTools[SearchToolPriceKey].CallCount)
			assert.Equal(t, test.fetch, info.BuiltInTools[FetchToolPriceKey].CallCount)
		})
	}
}
