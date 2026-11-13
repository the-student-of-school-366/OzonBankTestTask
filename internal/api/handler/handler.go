package handler

import (
	"fmt"
	"net/http"

	"OzonBankTestTask/internal/api"
	graph "OzonBankTestTask/internal/graphql"
	"OzonBankTestTask/internal/graphql/limits"
	"OzonBankTestTask/internal/infrastructure/middleware"
)

// NewHandler создаёт обработчик GraphQL, который можно смонтировать на /graphql.
// X-User-ID — демонстрационный способ передачи actor ID, не аутентификация.
// Доверенный middleware может установить actor через WithActor до вызова обработчика.
func NewHandler(posts api.PostUseCase, comments api.CommentUseCase, subscriptions api.SubscriptionUseCase, config api.Config) (*Handler, error) {
	if posts == nil || comments == nil || subscriptions == nil {
		return nil, fmt.Errorf("api: post, comment and subscription usecases are required")
	}
	config, err := config.WithDefaults()
	if err != nil {
		return nil, err
	}

	server := graph.NewServer(posts, comments, subscriptions, graph.Config{
		Limits: limits.Config{MaxDepth: config.MaxDepth, MaxComplexity: config.MaxComplexity},
		Logger: config.Logger, RequestTimeout: config.RequestTimeout,
		MaxMessageBytes: config.MaxBodyBytes, Origins: config.WebsocketOrigins,
	})

	ordinary := newHTTPHandler(server, config.MaxBodyBytes)
	timed := middleware.Timeout(config.RequestTimeout, timeoutBody)(ordinary)
	var wrapped http.Handler = routeTransport(server, timed, config.MaxBodyBytes)
	wrapped = middleware.DemoActor(wrapped)
	wrapped = middleware.RequestID(wrapped)
	return &Handler{server: server, handler: responseHeaders(wrapped)}, nil
}

type Handler struct {
	handler http.Handler
	server  *graph.Server
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) { h.handler.ServeHTTP(w, r) }
func (h *Handler) Close()                                           { h.server.Close() }
