package router

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestDingTalkBotTestRoutesAreRegistered(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	SetApiRouter(engine)
	routes := make(map[string]bool)
	for _, route := range engine.Routes() {
		routes[route.Method+" "+route.Path] = true
	}
	assert.True(t, routes["GET /api/dingtalk/test-users"])
	assert.True(t, routes["POST /api/dingtalk/test-message"])
	assert.True(t, routes["POST /api/dingtalk/test-group-message"])
}

func TestDingTalkBotTestRoutesRequireRootAuthentication(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	SetApiRouter(engine)
	for _, endpoint := range []struct {
		method string
		path   string
	}{
		{method: http.MethodGet, path: "/api/dingtalk/test-users"},
		{method: http.MethodPost, path: "/api/dingtalk/test-message"},
		{method: http.MethodPost, path: "/api/dingtalk/test-group-message"},
	} {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(endpoint.method, endpoint.path, nil)
		engine.ServeHTTP(recorder, request)
		assert.Equal(t, http.StatusUnauthorized, recorder.Code)
	}
}
