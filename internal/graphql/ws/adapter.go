package ws

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/99designs/gqlgen/graphql/handler/transport"
	"github.com/coder/websocket"
)

const Protocol = "graphql-transport-ws"

// Adapter ограничивает запись и связывает сокеты с жизненным циклом сервера.
type Adapter struct {
	Lifetime     context.Context
	WriteTimeout time.Duration
	Origins      []string
}

func (a Adapter) Accept(w http.ResponseWriter, r *http.Request, options transport.WebsocketAcceptOptions) (transport.WebsocketConn, error) {
	if a.Lifetime.Err() != nil {
		http.Error(w, "server stopped", http.StatusServiceUnavailable)
		return nil, a.Lifetime.Err()
	}
	supported := false
	for _, value := range r.Header.Values("Sec-WebSocket-Protocol") {
		for _, protocol := range strings.Split(value, ",") {
			supported = supported || strings.TrimSpace(protocol) == Protocol
		}
	}
	if !supported {
		http.Error(w, "expected graphql-transport-ws", http.StatusBadRequest)
		return nil, fmt.Errorf("unsupported websocket subprotocol")
	}
	for key, values := range options.ResponseHeader {
		for _, value := range values {
			w.Header().Add(key, value)
		}
	}
	raw, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		Subprotocols: []string{Protocol}, OriginPatterns: a.Origins,
	})
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithCancel(r.Context())
	c := &connection{raw: raw, ctx: ctx, cancel: cancel, writeTimeout: a.WriteTimeout}
	c.mu.Lock()
	c.stop = context.AfterFunc(a.Lifetime, func() { _ = c.Close() })
	c.mu.Unlock()
	return c, nil
}

type connection struct {
	raw          *websocket.Conn
	ctx          context.Context
	cancel       context.CancelFunc
	writeTimeout time.Duration
	mu           sync.Mutex
	readTimer    *time.Timer
	stop         func() bool
}

var (
	_ transport.WebsocketImplementation = Adapter{}
	_ transport.WebsocketConn           = (*connection)(nil)
	_ transport.WebsocketReadLimiter    = (*connection)(nil)
	_ transport.WebsocketReadDeadliner  = (*connection)(nil)
)

func (c *connection) NextReader() (int, io.Reader, error) {
	kind, data, err := c.raw.Read(c.ctx)
	if websocket.CloseStatus(err) == websocket.StatusNormalClosure {
		err = transport.ErrWebsocketClosed
	}
	return int(kind), bytes.NewReader(data), err
}

func (c *connection) WriteJSON(value any) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(c.ctx, c.writeTimeout)
	defer cancel()
	err = c.raw.Write(ctx, websocket.MessageText, data)
	if err != nil {
		// Закрыть также reader: таймаут до захвата writer-lock сам по себе
		// может не закрыть сокет, и подписки иначе останутся активными.
		_ = c.Close()
	}
	return err
}

func (c *connection) WriteClose(code int, message string) error {
	return c.raw.Close(websocket.StatusCode(code), message)
}

func (c *connection) Close() error {
	c.mu.Lock()
	if c.readTimer != nil {
		c.readTimer.Stop()
	}
	c.stop()
	c.mu.Unlock()
	c.cancel()
	return c.raw.CloseNow()
}

func (c *connection) Subprotocol() string      { return c.raw.Subprotocol() }
func (c *connection) SetReadLimit(limit int64) { c.raw.SetReadLimit(limit) }

func (c *connection) SetReadDeadline(deadline time.Time) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.readTimer != nil {
		c.readTimer.Stop()
	}
	if !deadline.IsZero() && c.ctx.Err() == nil {
		c.readTimer = time.AfterFunc(time.Until(deadline), func() { _ = c.Close() })
	}
	return nil
}
