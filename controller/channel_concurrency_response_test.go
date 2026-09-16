package controller

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/synctest"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTaskConcurrencyRejectionKeepsExistingEnvelope(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		previous := common.RedisEnabled
		common.RedisEnabled = false
		t.Cleanup(func() { common.RedisEnabled = previous })
		channel := &model.Channel{Id: 1201, MaxConcurrency: common.GetPointer(1)}
		occupying, _ := gin.CreateTestContext(httptest.NewRecorder())
		occupying.Request = httptest.NewRequest(http.MethodPost, "/v1/videos", nil)
		_, apiErr := service.ReserveChannelForRequest(occupying, channel)
		require.Nil(t, apiErr)
		defer service.ReleaseChannelConcurrency(occupying)
		recorder := httptest.NewRecorder()
		request, _ := gin.CreateTestContext(recorder)
		request.Request = httptest.NewRequest(http.MethodPost, "/v1/videos", nil)
		_, apiErr = service.ReserveChannelForRequest(request, channel)
		require.NotNil(t, apiErr)
		respondTaskError(request, service.TaskErrorWrapperLocal(apiErr.Err, string(apiErr.GetErrorCode()), apiErr.StatusCode))
		assert.Equal(t, http.StatusTooManyRequests, recorder.Code)
		var body map[string]any
		require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &body))
		assert.Equal(t, map[string]any{"code": "channel_concurrency_limit", "message": "Concurrency limit exceeded for account, please retry later", "data": nil}, body)
	})
}

func TestOtherTask429KeepsExistingResponse(t *testing.T) {
	recorder := httptest.NewRecorder()
	request, _ := gin.CreateTestContext(recorder)
	request.Request = httptest.NewRequest(http.MethodPost, "/v1/videos", nil)
	respondTaskError(request, service.TaskErrorWrapperLocal(errors.New("Concurrency limit exceeded for account, please retry later"), "channel_concurrency_limit", http.StatusTooManyRequests))
	var body map[string]any
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &body))
	assert.Equal(t, 429, recorder.Code)
	assert.Equal(t, map[string]any{"code": "channel_concurrency_limit", "message": "当前分组上游负载已饱和，请稍后再试", "data": nil}, body)
}
