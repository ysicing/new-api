package router

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestActiveUserStatsRouteRequiresAdminAuthentication(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	SetApiRouter(engine)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/data/active-users", nil)

	engine.ServeHTTP(recorder, request)

	assert.Equal(t, http.StatusUnauthorized, recorder.Code)
}
