package channel

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestChannelConcurrencyContextCancelsUpstreamHTTPRequest(t *testing.T) {
	previous := common.RedisEnabled
	common.RedisEnabled = false
	t.Cleanup(func() { common.RedisEnabled = previous })
	started, stopped := make(chan struct{}), make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		close(started)
		<-r.Context().Done()
		close(stopped)
	}))
	defer server.Close()
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{}`)).WithContext(ctx)
	_, apiErr := service.ReserveChannelForRequest(c, &model.Channel{Id: 901, MaxConcurrency: common.GetPointer(1)})
	require.Nil(t, apiErr)
	defer service.ReleaseChannelConcurrency(c)
	request, err := http.NewRequest(http.MethodPost, server.URL, strings.NewReader(`{}`))
	require.NoError(t, err)
	completed := make(chan error, 1)
	go func() {
		_, err := doRequest(c, request, &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{}})
		completed <- err
	}()
	<-started
	cancel()
	require.Error(t, <-completed)
	<-stopped
}
