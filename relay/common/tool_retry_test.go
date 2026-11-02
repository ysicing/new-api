package common

import (
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestInitChannelMetaClearsToolUsageButPreservesDeclarations(t *testing.T) {
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Set("claude_web_search_requests", 3)
	info := &RelayInfo{
		WebSearchBillingKey: "deepseek_search", WebFetchBillingKey: "deepseek_fetch",
		VendorToolUsage: func([]byte) map[string]int { return map[string]int{"deepseek_search": 5} },
		ResponsesUsageInfo: &ResponsesUsageInfo{BuiltInTools: map[string]*BuildInToolInfo{
			dto.BuildInToolWebSearchPreview: {ToolName: dto.BuildInToolWebSearchPreview, CallCount: 3, SearchContextSize: "low"},
			"deepseek_search":               {ToolName: "deepseek_search", CallCount: 2},
		}},
	}
	info.InitChannelMeta(ctx)
	assert.Empty(t, info.WebSearchBillingKey)
	assert.Empty(t, info.WebFetchBillingKey)
	assert.Nil(t, info.VendorToolUsage)
	assert.Zero(t, ctx.GetInt("claude_web_search_requests"))
	assert.Zero(t, info.BuiltInTools["deepseek_search"].CallCount)
	assert.Zero(t, info.BuiltInTools[dto.BuildInToolWebSearchPreview].CallCount)
	assert.Equal(t, "low", info.BuiltInTools[dto.BuildInToolWebSearchPreview].SearchContextSize)
}
