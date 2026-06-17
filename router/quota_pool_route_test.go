package router

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestQuotaPoolSelfRouteTakesPrecedenceOverIdRoute(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/api/quota_pool/:id", func(c *gin.Context) {
		c.String(http.StatusOK, "id:"+c.Param("id"))
	})
	router.GET("/api/quota_pool/self", func(c *gin.Context) {
		c.String(http.StatusOK, "self")
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/quota_pool/self", nil)
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
	if recorder.Body.String() != "self" {
		t.Fatalf("route response = %q, want self", recorder.Body.String())
	}
}
