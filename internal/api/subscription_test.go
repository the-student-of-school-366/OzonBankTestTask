package api_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"OzonBankTestTask/internal/api"
	"OzonBankTestTask/internal/domain"
	"OzonBankTestTask/internal/domain/comment"
	"OzonBankTestTask/internal/domain/page"
	"OzonBankTestTask/internal/dto"
	graphmocks "OzonBankTestTask/internal/graphql/mocks"
	"OzonBankTestTask/internal/infrastructure/pubsub"
	commentusecase "OzonBankTestTask/internal/usecase/comment"
	commentmocks "OzonBankTestTask/internal/usecase/comment/mocks"
	"OzonBankTestTask/internal/usecase/subscription"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type observedBroker struct {
	*pubsub.Hub
	registered, canceled chan struct{}
}

func (b *observedBroker) Subscribe(ctx context.Context, id uuid.UUID) (subscription.Stream, error) {
	stream, err := b.Hub.Subscribe(ctx, id)
	if err != nil {
		return stream, err
	}
	b.registered <- struct{}{}
	cancel := stream.Cancel
	var once sync.Once
	stream.Cancel = func() {
		cancel()
		once.Do(func() { b.canceled <- struct{}{} })
	}
	return stream, nil
}

type subscriptionFixture struct {
	handler  *api.Handler
	server   *httptest.Server
	posts    *graphmocks.PostUseCase
	comments *commentmocks.Repository
	broker   *observedBroker
}

func newSubscriptionFixture(t *testing.T, config api.Config) subscriptionFixture {
	t.Helper()
	hub, err := pubsub.New(pubsub.Config{})
	require.NoError(t, err)
	broker := &observedBroker{Hub: hub, registered: make(chan struct{}, 16), canceled: make(chan struct{}, 16)}
	posts, comments := graphmocks.NewPostUseCase(t), commentmocks.NewRepository(t)
	config.Logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	h, err := api.NewHandler(posts, commentusecase.New(comments, hub), subscription.New(posts, broker), config)
	require.NoError(t, err)
	server := httptest.NewServer(h)
	t.Cleanup(func() { h.Close(); hub.Close(); server.Close() })
	return subscriptionFixture{handler: h, server: server, posts: posts, comments: comments, broker: broker}
}

func socketContext(t *testing.T) context.Context {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	t.Cleanup(cancel)
	return ctx
}

func connectSocket(t *testing.T, server *httptest.Server) *websocket.Conn {
	t.Helper()
	conn, response, err := websocket.Dial(socketContext(t), "ws"+strings.TrimPrefix(server.URL, "http"), &websocket.DialOptions{Subprotocols: []string{"graphql-transport-ws"}})
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.CloseNow() })
	require.Equal(t, "graphql-transport-ws", conn.Subprotocol())
	require.NotEmpty(t, response.Header.Get("X-Request-ID"))
	require.NoError(t, wsjson.Write(socketContext(t), conn, map[string]any{"type": "connection_init"}))
	require.Equal(t, "connection_ack", readSocket(t, conn).Type)
	return conn
}

type socketMessage struct {
	Type    string          `json:"type"`
	ID      string          `json:"id"`
	Payload json.RawMessage `json:"payload"`
}

func readSocket(t *testing.T, conn *websocket.Conn) socketMessage {
	t.Helper()
	var message socketMessage
	require.NoError(t, wsjson.Read(socketContext(t), conn, &message))
	return message
}

func subscribe(t *testing.T, conn *websocket.Conn, query string) {
	t.Helper()
	require.NoError(t, wsjson.Write(socketContext(t), conn, map[string]any{"id": "sub", "type": "subscribe", "payload": map[string]any{"query": query}}))
}

func awaitSignal(t *testing.T, signal <-chan struct{}) {
	t.Helper()
	select {
	case <-signal:
	case <-time.After(5 * time.Second):
		t.Fatal("subscription lifecycle signal was not received")
	}
}

func socketResponse(t *testing.T, conn *websocket.Conn) graphResponse {
	t.Helper()
	message := readSocket(t, conn)
	require.Equal(t, "next", message.Type)
	require.Equal(t, "sub", message.ID)
	var response graphResponse
	require.NoError(t, json.Unmarshal(message.Payload, &response))
	return response
}

func TestSubscriptionReceivesCommittedRootAndReplyPastHTTPTimeout(t *testing.T) {
	f := newSubscriptionFixture(t, api.Config{RequestTimeout: 100 * time.Millisecond})
	post := examplePost()
	f.posts.On("GetMany", mock.Anything, []string{post.ID}).Return([]*dto.Post{&post}, nil).Twice()
	clients := []*websocket.Conn{connectSocket(t, f.server), connectSocket(t, f.server)}
	for _, conn := range clients {
		subscribe(t, conn, fmt.Sprintf(`subscription{commentAdded(postId:%q){id postId parentId text authorId}}`, post.ID))
		awaitSignal(t, f.broker.registered)
	}
	time.Sleep(200 * time.Millisecond)
	f.comments.On("CreateComment", mock.Anything, mock.Anything).Return(func(ctx context.Context, value comment.Comment) (comment.Comment, error) { return value, nil }).Twice()
	var parentID *string
	for _, text := range []string{"root", "reply"} {
		_, response := request(t, f.handler, `mutation($input:CreateCommentInput!){createComment(input:$input){id postId parentId text authorId}}`, map[string]any{"input": map[string]any{"postId": post.ID, "parentId": parentID, "text": text}}, "reader")
		require.Empty(t, response.Errors)
		for _, conn := range clients {
			event := socketResponse(t, conn)
			require.Empty(t, event.Errors)
			require.JSONEq(t, string(response.Data["createComment"]), string(event.Data["commentAdded"]))
		}
		var saved dto.Comment
		require.NoError(t, json.Unmarshal(response.Data["createComment"], &saved))
		parentID = &saved.ID
	}
}

func TestSubscriptionUsesFreshLoadersForEachEvent(t *testing.T) {
	f := newSubscriptionFixture(t, api.Config{})
	post := examplePost()
	f.posts.On("GetMany", mock.Anything, []string{post.ID}).Return([]*dto.Post{&post}, nil).Once()
	value := comment.Comment{ID: uuid.New(), PostID: uuid.MustParse(post.ID), Author: "author", Text: "root", CreatedAt: domain.NormalizeTime(time.Now())}
	conn := connectSocket(t, f.server)
	subscribe(t, conn, fmt.Sprintf(`subscription{commentAdded(postId:%q){replies(first:1){nodes{text}}}}`, post.ID))
	awaitSignal(t, f.broker.registered)
	for i := 1; i <= 2; i++ {
		reply := comment.Comment{ID: uuid.New(), PostID: value.PostID, ParentID: &value.ID, Author: "reader", Text: fmt.Sprint(i), CreatedAt: domain.NormalizeTime(time.Now())}
		position := reply.PagePosition()
		f.comments.On("ListRepliesBatch", mock.Anything, mock.Anything).Return([]page.Page[comment.Comment]{{Items: []comment.Comment{reply}, End: &position}}, nil).Once()
		f.broker.Publish(value)
		response := socketResponse(t, conn)
		require.Empty(t, response.Errors)
		require.JSONEq(t, fmt.Sprintf(`{"replies":{"nodes":[{"text":%q}]}}`, reply.Text), string(response.Data["commentAdded"]))
	}
}

func TestSubscriptionCleanup(t *testing.T) {
	for _, action := range []string{"complete", "disconnect", "shutdown", "hub shutdown"} {
		t.Run(action, func(t *testing.T) {
			f := newSubscriptionFixture(t, api.Config{})
			post := examplePost()
			f.posts.On("GetMany", mock.Anything, []string{post.ID}).Return([]*dto.Post{&post}, nil).Once()
			conn := connectSocket(t, f.server)
			subscribe(t, conn, fmt.Sprintf(`subscription{commentAdded(postId:%q){id}}`, post.ID))
			awaitSignal(t, f.broker.registered)
			switch action {
			case "complete":
				require.NoError(t, wsjson.Write(socketContext(t), conn, map[string]string{"id": "sub", "type": "complete"}))
			case "disconnect":
				_ = conn.CloseNow()
			case "shutdown":
				f.handler.Close()
				_, _, err := conn.Read(socketContext(t))
				require.Error(t, err)
			case "hub shutdown":
				f.broker.Close()
				require.Equal(t, "complete", readSocket(t, conn).Type)
			}
			awaitSignal(t, f.broker.canceled)
		})
	}
}

func TestSubscriptionErrorsCompleteOnce(t *testing.T) {
	for _, tc := range []struct {
		name, query, code string
		config            api.Config
	}{
		{name: "bad id", query: `subscription{commentAdded(postId:"bad"){id}}`, code: "BAD_USER_INPUT"},
		{name: "missing post", code: "NOT_FOUND"},
		{name: "depth", query: fmt.Sprintf(`subscription{commentAdded(postId:%q){replies{nodes{id}}}}`, uuid.NewString()), config: api.Config{MaxDepth: 2}, code: "QUERY_TOO_DEEP"},
		{name: "complexity", query: fmt.Sprintf(`subscription{commentAdded(postId:%q){replies(first:100){nodes{id}}}}`, uuid.NewString()), config: api.Config{MaxComplexity: 3}, code: "QUERY_TOO_COMPLEX"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newSubscriptionFixture(t, tc.config)
			if tc.name == "missing post" {
				id := uuid.NewString()
				tc.query = fmt.Sprintf(`subscription{commentAdded(postId:%q){id}}`, id)
				f.posts.On("GetMany", mock.Anything, []string{id}).Return([]*dto.Post{nil}, nil).Once()
			}
			conn := connectSocket(t, f.server)
			subscribe(t, conn, tc.query)
			response := socketResponse(t, conn)
			assertCode(t, response, tc.code)
			require.NotEmpty(t, response.Errors[0].Extensions["requestId"])
			require.Equal(t, "complete", readSocket(t, conn).Type)
		})
	}
}

func TestSubscriptionsRejectPlainHTTP(t *testing.T) {
	h, _, _ := newHandler(t, api.Config{})
	_, response := request(t, h, fmt.Sprintf(`subscription{commentAdded(postId:%q){id}}`, uuid.NewString()), nil, "")
	assertCode(t, response, "BAD_USER_INPUT")
}

func TestShutdownClosesWebsocketBeforeConnectionInit(t *testing.T) {
	f := newSubscriptionFixture(t, api.Config{})
	conn, _, err := websocket.Dial(socketContext(t), "ws"+strings.TrimPrefix(f.server.URL, "http"), &websocket.DialOptions{Subprotocols: []string{"graphql-transport-ws"}})
	require.NoError(t, err)
	defer conn.CloseNow()
	f.handler.Close()
	_, _, err = conn.Read(socketContext(t))
	require.Error(t, err)
}

func TestWebsocketQueryHasOperationTimeout(t *testing.T) {
	f := newSubscriptionFixture(t, api.Config{RequestTimeout: 30 * time.Millisecond})
	f.posts.On("List", mock.Anything, mock.Anything).Return(func(ctx context.Context, input dto.PageInput) (dto.PostConnection, error) {
		<-ctx.Done()
		return dto.PostConnection{}, ctx.Err()
	}).Once()
	conn := connectSocket(t, f.server)
	subscribe(t, conn, `{posts{nodes{id}}}`)
	assertCode(t, socketResponse(t, conn), "TIMEOUT")
	require.Equal(t, "complete", readSocket(t, conn).Type)
}

func TestWebsocketOriginAndProtocol(t *testing.T) {
	for _, tc := range []struct {
		name, origin, protocol string
		allowed                []string
		status                 int
	}{
		{name: "foreign origin", origin: "http://foreign.example", protocol: "graphql-transport-ws", status: http.StatusForbidden},
		{name: "legacy protocol", protocol: "graphql-ws", status: http.StatusBadRequest},
		{name: "explicit origin", origin: "http://allowed.example", protocol: "graphql-transport-ws", allowed: []string{"allowed.example"}, status: http.StatusSwitchingProtocols},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newSubscriptionFixture(t, api.Config{WebsocketOrigins: tc.allowed})
			conn, response, err := websocket.Dial(socketContext(t), "ws"+strings.TrimPrefix(f.server.URL, "http"), &websocket.DialOptions{Subprotocols: []string{tc.protocol}, HTTPHeader: http.Header{"Origin": []string{tc.origin}}})
			if tc.status == http.StatusSwitchingProtocols {
				require.NoError(t, err)
				_ = conn.CloseNow()
			} else {
				require.Error(t, err)
			}
			require.NotNil(t, response)
			require.Equal(t, tc.status, response.StatusCode)
		})
	}
}

func TestWebsocketMessageSizeLimit(t *testing.T) {
	f := newSubscriptionFixture(t, api.Config{MaxBodyBytes: 256})
	conn := connectSocket(t, f.server)
	require.NoError(t, wsjson.Write(socketContext(t), conn, map[string]any{"type": "ping", "payload": strings.Repeat("x", 1024)}))
	_, _, err := conn.Read(socketContext(t))
	require.Error(t, err)
	require.Equal(t, websocket.StatusMessageTooBig, websocket.CloseStatus(err))
}
