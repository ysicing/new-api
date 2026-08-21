package router

import (
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
		"POST /api/quota_pool/:id/members/:user_id/recharge",
		"POST /api/quota_pool/:id/members/:user_id/reclaim",
		"GET /api/quota_pool/:id/transactions", "GET /api/quota_pool/:id/operation_logs",
		"GET /api/quota_pool/:id/stats", "GET /api/quota_pool/candidates",
		"GET /api/quota_pool/self/", "PUT /api/quota_pool/self/",
		"GET /api/quota_pool/self/members", "GET /api/quota_pool/self/transactions",
	}
	for _, key := range expected {
		_, ok := routes[key]
		assert.True(t, ok, key)
	}
}
