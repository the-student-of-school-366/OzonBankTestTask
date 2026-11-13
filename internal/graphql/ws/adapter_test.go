package ws_test

import (
	"bufio"
	"context"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
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

// stalledConn моделирует заблокированную сеть без зависимости от размера
// системных буферов; закрытие соединения освобождает ожидающую запись.
type stalledConn struct {
	net.Conn
	started   chan struct{}
	closed    chan struct{}
	startOnce sync.Once
	closeOnce sync.Once
}

func (c *stalledConn) Write([]byte) (int, error) {
	c.startOnce.Do(func() { close(c.started) })
	<-c.closed
	return 0, net.ErrClosed
}

func (c *stalledConn) Close() error {
	c.closeOnce.Do(func() { close(c.closed) })
	return c.Conn.Close()
}

type stalledResponseWriter struct {
	http.ResponseWriter
	started chan struct{}
}

func (w stalledResponseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	conn, buffered, err := w.ResponseWriter.(http.Hijacker).Hijack()
	if err != nil {
		return nil, nil, err
	}
	stalled := &stalledConn{Conn: conn, started: w.started, closed: make(chan struct{})}
	buffered.Writer.Reset(stalled)
	return stalled, buffered, nil
}

// Проверяем положительный таймаут во время уже начавшейся записи.
func TestBlockedWriteTimesOutAndClosesConnection(t *testing.T) {
	lifetime, cancel := context.WithCancel(context.Background())
	accepted := make(chan transport.WebsocketConn, 1)
	started := make(chan struct{})
	adapter := ws.Adapter{Lifetime: lifetime, WriteTimeout: 150 * time.Millisecond}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := adapter.Accept(stalledResponseWriter{ResponseWriter: w, started: started}, r, transport.WebsocketAcceptOptions{})
		if err != nil {
			return
		}
		defer conn.Close()
		accepted <- conn
		<-lifetime.Done()
	}))
	t.Cleanup(func() { cancel(); server.Close() })
	ctx, stop := context.WithTimeout(t.Context(), 10*time.Second)
	defer stop()
	client, _, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(server.URL, "http"), &websocket.DialOptions{Subprotocols: []string{ws.Protocol}})
	require.NoError(t, err)
	defer client.CloseNow()
	var conn transport.WebsocketConn
	select {
	case conn = <-accepted:
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	writeResult := make(chan error, 1)
	go func() { writeResult <- conn.WriteJSON(map[string]string{"payload": "blocked"}) }()
	select {
	case <-started:
	case <-ctx.Done():
		t.Fatal("write did not start")
	}
	select {
	case err = <-writeResult:
		require.ErrorIs(t, err, context.DeadlineExceeded)
	case <-ctx.Done():
		t.Fatal("write did not time out")
	}
	_, _, err = client.Read(ctx)
	require.Error(t, err)
	require.NoError(t, ctx.Err(), "connection must close before the test deadline")
	cancel()
}

func TestReadDeadlineCanBeCleared(t *testing.T) {
	lifetime, cancel := context.WithCancel(context.Background())
	accepted := make(chan transport.WebsocketConn, 1)
	adapter := ws.Adapter{Lifetime: lifetime, WriteTimeout: time.Second}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := adapter.Accept(w, r, transport.WebsocketAcceptOptions{})
		if err != nil {
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
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	deadliner := conn.(transport.WebsocketReadDeadliner)
	require.NoError(t, deadliner.SetReadDeadline(time.Now().Add(30*time.Millisecond)))
	require.NoError(t, deadliner.SetReadDeadline(time.Time{}))
	select {
	case <-time.After(80 * time.Millisecond):
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	require.NoError(t, client.Write(ctx, websocket.MessageText, []byte("hello")))
	_, reader, err := conn.NextReader()
	require.NoError(t, err)
	value, err := io.ReadAll(reader)
	require.NoError(t, err)
	require.Equal(t, "hello", string(value))
	cancel()
}
