package router

import (
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestDingTalkNotificationOperationsRouteIsRegistered(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	SetApiRouter(engine)

	registered := false
	for _, route := range engine.Routes() {
		if route.Method == "GET" && route.Path == "/api/dingtalk-notifications/" {
			registered = true
			break
		}
	}
	assert.True(t, registered)
}
