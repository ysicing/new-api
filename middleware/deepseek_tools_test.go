package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/constant"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDeepSeekToolsRouteIgnoresBodyModel(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, test := range []struct{ path, body, model string }{
		{"/v1/tools/search", `{"query":"deepseek"}`, constant.DeepSeekSearchModel},
		{"/v1/tools/search", `{"query":"deepseek","model":"free-model"}`, constant.DeepSeekSearchModel},
		{"/v1/tools/fetch", `{"url":"https://example.com"}`, constant.DeepSeekFetchModel},
		{"/v1/tools/fetch", `{"url":"https://example.com","model":"free-model"}`, constant.DeepSeekFetchModel},
	} {
		t.Run(test.path+test.body, func(t *testing.T) {
			ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
			ctx.Request = httptest.NewRequest(http.MethodPost, test.path, strings.NewReader(test.body))
			ctx.Request.Header.Set("Content-Type", "application/json")
			request, selectChannel, err := getModelRequest(ctx)
			require.NoError(t, err)
			assert.True(t, selectChannel)
			assert.Equal(t, test.model, request.Model)
		})
	}
}
