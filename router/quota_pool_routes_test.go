package router

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestQuotaPoolCompatibilityRoutesAreRegistered(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	SetApiRouter(engine)
	routes := make(map[string]struct{})
	for _, route := range engine.Routes() {
		routes[route.Method+" "+route.Path] = struct{}{}
	}

	expected := []string{
		"GET /api/quota_pool/", "POST /api/quota_pool/",
		"GET /api/quota_pool/:id", "PUT /api/quota_pool/:id", "DELETE /api/quota_pool/:id",
		"GET /api/quota_pool/:id/members", "POST /api/quota_pool/:id/members",
		"DELETE /api/quota_pool/:id/members/:user_id",
		"POST /api/quota_pool/:id/members/:user_id/recharge",
		"POST /api/quota_pool/:id/members/:user_id/reclaim",
		"GET /api/quota_pool/:id/transactions", "GET /api/quota_pool/:id/operation_logs",
		"GET /api/quota_pool/:id/stats", "GET /api/quota_pool/:id/stats/export", "GET /api/quota_pool/candidates",
		"GET /api/quota_pool/recharge_query/records", "POST /api/quota_pool/recharge_query/eligibility",
		"GET /api/quota_pool/self/", "PUT /api/quota_pool/self/",
		"GET /api/quota_pool/self/members", "GET /api/quota_pool/self/transactions", "GET /api/quota_pool/self/stats/export",
		"DELETE /api/quota_pool/self/members/:user_id", "PUT /api/quota_pool/self/members/:user_id",
	}
	for _, key := range expected {
		_, ok := routes[key]
		assert.True(t, ok, key)
	}
}

func TestQuotaPoolRechargeQueryRoutesRequireRootAuthentication(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	SetApiRouter(engine)
	for _, endpoint := range []struct {
		method string
		path   string
	}{
		{method: http.MethodGet, path: "/api/quota_pool/recharge_query/records"},
		{method: http.MethodPost, path: "/api/quota_pool/recharge_query/eligibility"},
	} {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(endpoint.method, endpoint.path, nil)
		engine.ServeHTTP(recorder, request)
		assert.Equal(t, http.StatusUnauthorized, recorder.Code, endpoint.path)
	}
}
