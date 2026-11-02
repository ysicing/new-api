package claude

import (
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDeepSeekClaudeServerUsageReplacesProvisionalCounts(t *testing.T) {
	for _, streaming := range []bool{false, true} {
		t.Run(map[bool]string{false: "nonstream", true: "stream"}[streaming], func(t *testing.T) {
			ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
			info := &relaycommon.RelayInfo{OriginModelName: "deepseek-flash", RelayFormat: types.RelayFormatClaude,
				WebSearchBillingKey: "deepseek_search", WebFetchBillingKey: "deepseek_fetch"}
			body := `{"type":"message","content":[
				{"type":"server_tool_use","name":"web_search","input":{}},
				{"type":"web_search_tool_result","content":[{"type":"web_search_result","url":"https://example.com"}]},
				{"type":"web_search_tool_result","content":[]},
				{"type":"web_search_tool_result","content":{"type":"web_search_tool_result_error"}}
			],"usage":{"input_tokens":1,"output_tokens":1,"server_tool_use":{"web_search_requests":0,"web_fetch_requests":2}}}`
			if streaming {
				var response dto.ClaudeResponse
				require.NoError(t, common.UnmarshalJsonStr(body, &response))
				for _, block := range response.Content {
					countClaudeStreamBillableTools(ctx, info, &dto.ClaudeResponse{Type: "content_block_start", ContentBlock: &block})
				}
				require.NotNil(t, info.ResponsesUsageInfo)
				assert.Equal(t, 1, info.BuiltInTools["deepseek_search"].CallCount)
				countClaudeStreamBillableTools(ctx, info, &dto.ClaudeResponse{Type: "message_delta", Usage: response.Usage})
			} else {
				apiError := HandleClaudeResponseData(ctx, info, &ClaudeResponseInfo{Usage: &dto.Usage{}}, nil, []byte(body))
				require.Nil(t, apiError)
			}
			require.NotNil(t, info.ResponsesUsageInfo)
			assert.Zero(t, info.BuiltInTools["deepseek_search"].CallCount)
			assert.Equal(t, 2, info.BuiltInTools["deepseek_fetch"].CallCount)
			assert.Zero(t, ctx.GetInt("claude_web_search_requests"))
			assert.NotContains(t, info.BuiltInTools, "web_search")
		})
	}
}

func TestHandleClaudeResponseDataCountsToolUse(t *testing.T) {
	gin.SetMode(gin.TestMode)
	operation_setting.SetToolPriceForTest("lookup_fn", 3.0)
	t.Cleanup(func() {
		operation_setting.DeleteToolPriceForTest("lookup_fn")
	})

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	info := &relaycommon.RelayInfo{
		OriginModelName: "claude-3-7-sonnet",
		RelayFormat:     types.RelayFormatClaude,
	}
	claudeInfo := &ClaudeResponseInfo{Usage: &dto.Usage{}}

	data := []byte(`{
		"type":"message",
		"content":[
			{"type":"text","text":"hi"},
			{"type":"tool_use","id":"tu1","name":"lookup_fn","input":{}},
			{"type":"server_tool_use","id":"stu1","name":"web_search","input":{}}
		],
		"usage":{"input_tokens":1,"output_tokens":1}
	}`)

	err := HandleClaudeResponseData(c, info, claudeInfo, nil, data)
	require.Nil(t, err)
	require.NotNil(t, info.ResponsesUsageInfo)
	require.Contains(t, info.ResponsesUsageInfo.BuiltInTools, "lookup_fn")
	assert.Equal(t, 1, info.ResponsesUsageInfo.BuiltInTools["lookup_fn"].CallCount)
	assert.NotContains(t, info.ResponsesUsageInfo.BuiltInTools, "web_search")
}

func TestCountClaudeStreamBillableToolsSetsWebSearchRequests(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	info := &relaycommon.RelayInfo{OriginModelName: "claude-3-7-sonnet"}

	countClaudeStreamBillableTools(c, info, &dto.ClaudeResponse{
		Type: "message_delta",
		Usage: &dto.ClaudeUsage{
			ServerToolUse: &dto.ClaudeServerToolUse{WebSearchRequests: 3},
		},
	})
	assert.Equal(t, 3, c.GetInt("claude_web_search_requests"))

	operation_setting.SetToolPriceForTest("stream_fn", 2.0)
	t.Cleanup(func() {
		operation_setting.DeleteToolPriceForTest("stream_fn")
	})
	countClaudeStreamBillableTools(c, info, &dto.ClaudeResponse{
		Type: "content_block_start",
		ContentBlock: &dto.ClaudeMediaMessage{
			Type: "tool_use",
			Name: "stream_fn",
		},
	})
	require.Contains(t, info.ResponsesUsageInfo.BuiltInTools, "stream_fn")
	assert.Equal(t, 1, info.ResponsesUsageInfo.BuiltInTools["stream_fn"].CallCount)
}
