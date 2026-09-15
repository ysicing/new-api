package xunfei

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/require"
)

func TestXunfeiRequestCancellationStopsBlockedResponseDelivery(t *testing.T) {
	sent := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := (&websocket.Upgrader{}).Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		_, _, err = conn.ReadMessage()
		if err != nil {
			return
		}
		_ = conn.WriteMessage(websocket.TextMessage, []byte(`{"payload":{"choices":{"status":1}}}`))
		close(sent)
		_, _, _ = conn.ReadMessage()
	}))
	defer server.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	_, stopped, err := xunfeiMakeRequest(ctx, dto.GeneralOpenAIRequest{}, "general", "ws"+strings.TrimPrefix(server.URL, "http"), "app")
	require.NoError(t, err)
	<-sent
	cancel()
	_, open := <-stopped
	require.False(t, open)
}
