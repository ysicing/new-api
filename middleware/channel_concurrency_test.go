package middleware

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestChannelConcurrencyHoldsStreamingSlotUntilCancellation(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:distribute-concurrency-%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.Channel{}))
	channel := model.Channel{Id: 701, Type: constant.ChannelTypeOpenAI, Key: "test-key", Name: "stream", Status: common.ChannelStatusEnabled, MaxConcurrency: common.GetPointer(1)}
	require.NoError(t, db.Create(&channel).Error)
	previousDB, previousRedis := model.DB, common.RedisEnabled
	model.DB, common.RedisEnabled = db, false
	t.Cleanup(func() { model.DB, common.RedisEnabled = previousDB, previousRedis })
	finished := make(chan struct{})
	router := gin.New()
	router.Use(func(c *gin.Context) { common.SetContextKey(c, constant.ContextKeyTokenSpecificChannelId, "701") })
	router.POST("/v1/chat/completions", func(c *gin.Context) {
		if c.Query("stream") == "true" {
			defer close(finished)
		}
		c.Next()
	}, Distribute(), func(c *gin.Context) {
		if c.Query("stream") == "true" {
			c.Header("Content-Type", "text/event-stream")
			_, _ = c.Writer.WriteString("data: hello\n\n")
			c.Writer.Flush()
			<-c.Request.Context().Done()
			return
		}
		if c.Query("fail") == "true" {
			c.Status(http.StatusBadGateway)
			return
		}
		c.Status(http.StatusOK)
	})
	server := httptest.NewServer(router)
	defer server.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, server.URL+"/v1/chat/completions?stream=true", strings.NewReader(`{"model":"chat-model"}`))
	require.NoError(t, err)
	request.Header.Set("Content-Type", "application/json")
	response, err := server.Client().Do(request)
	require.NoError(t, err)
	defer response.Body.Close()
	require.Equal(t, http.StatusOK, response.StatusCode)
	busy, err := server.Client().Post(server.URL+"/v1/chat/completions", "application/json", strings.NewReader(`{"model":"chat-model"}`))
	require.NoError(t, err)
	body, err := io.ReadAll(busy.Body)
	require.NoError(t, err)
	_ = busy.Body.Close()
	assert.Equal(t, http.StatusTooManyRequests, busy.StatusCode, string(body))
	cancel()
	<-finished
	failed, err := server.Client().Post(server.URL+"/v1/chat/completions?fail=true", "application/json", strings.NewReader(`{"model":"chat-model"}`))
	require.NoError(t, err)
	_ = failed.Body.Close()
	assert.Equal(t, http.StatusBadGateway, failed.StatusCode)
	recovered, err := server.Client().Post(server.URL+"/v1/chat/completions", "application/json", strings.NewReader(`{"model":"chat-model"}`))
	require.NoError(t, err)
	_ = recovered.Body.Close()
	assert.Equal(t, http.StatusOK, recovered.StatusCode)
}
