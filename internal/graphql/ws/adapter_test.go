package ws_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"OzonBankTestTask/internal/graphql/ws"

	"github.com/99designs/gqlgen/graphql/handler/transport"
	"github.com/coder/websocket"
	"github.com/stretchr/testify/require"
)

func TestConnectionDeadlinesAndLifetime(t *testing.T) {
	for _, mode := range []string{"write deadline", "read deadline", "server lifetime"} {
		t.Run(mode, func(t *testing.T) {
			lifetime, cancel := context.WithCancel(context.Background())
			defer cancel()
			writeTimeout := time.Second
			if mode == "write deadline" {
				writeTimeout = -time.Second
			}
			adapter := ws.Adapter{Lifetime: lifetime, WriteTimeout: writeTimeout}
			accepted := make(chan transport.WebsocketConn, 1)
			failures := make(chan error, 1)
			finished := make(chan struct{})
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				defer close(finished)
				conn, err := adapter.Accept(w, r, transport.WebsocketAcceptOptions{})
				if err != nil {
					failures <- err
					return
				}
				defer conn.Close()
				accepted <- conn
				<-lifetime.Done()
			}))
			t.Cleanup(func() { cancel(); server.Close() })
			ctx, stop := context.WithTimeout(t.Context(), 5*time.Second)
			defer stop()
			client, _, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(server.URL, "http"), &websocket.DialOptions{Subprotocols: []string{ws.Protocol}})
			require.NoError(t, err)
			defer client.CloseNow()
			var conn transport.WebsocketConn
			select {
			case conn = <-accepted:
			case err = <-failures:
				t.Fatal(err)
			case <-ctx.Done():
				t.Fatal(ctx.Err())
			}
			switch mode {
			case "write deadline":
				require.ErrorIs(t, conn.WriteJSON(map[string]string{"type": "next"}), context.DeadlineExceeded)
			case "read deadline":
				require.NoError(t, conn.(transport.WebsocketReadDeadliner).SetReadDeadline(time.Now().Add(-time.Second)))
			case "server lifetime":
				cancel()
			}
			_, _, err = client.Read(ctx)
			require.Error(t, err)
			require.NoError(t, ctx.Err(), "connection must close before the test deadline")
			cancel()
			select {
			case <-finished:
			case <-ctx.Done():
				t.Fatal("server handler did not stop")
			}
		})
	}
}
