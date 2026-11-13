package graphql

import (
	"context"
	"fmt"
	"time"

	"OzonBankTestTask/internal/graphql/generated"
	"OzonBankTestTask/internal/graphql/gqlerrors"
	"OzonBankTestTask/internal/graphql/ws"

	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/99designs/gqlgen/graphql/handler/extension"
	"github.com/99designs/gqlgen/graphql/handler/transport"
)

type Server struct {
	*handler.Server
	cancel context.CancelFunc
}

// Close прерывает в том числе WebSocket-соединения, которыми http.Server.Shutdown не управляет.
func (s *Server) Close() { s.cancel() }

type websocketKey struct{}

// NewServer собирает GraphQL-исполнитель из проверенных зависимостей и настроек.
// HTTP-ограничения и метаданные запроса устанавливает api/handler.NewHandler.
func NewServer(posts PostUseCase, comments CommentUseCase, subscriptions SubscriptionUseCase, config Config) *Server {
	lifetime, cancel := context.WithCancel(context.Background())
	server := handler.New(generated.NewExecutableSchema(generated.Config{
		Resolvers: &Resolver{posts: posts, comments: comments, subscriptions: subscriptions},
	}))
	server.AddTransport(transport.Websocket{
		Implementation:   ws.Adapter{Lifetime: lifetime, WriteTimeout: 10 * time.Second, Origins: append([]string(nil), config.Origins...)},
		InitTimeout:      5 * time.Second,
		PingPongInterval: 15 * time.Second,
		PayloadReadLimit: &config.MaxMessageBytes,
		InitFunc: func(ctx context.Context, _ transport.InitPayload) (context.Context, *transport.InitPayload, error) {
			return context.WithValue(ctx, websocketKey{}, true), nil, nil
		},
	})
	server.AddTransport(transport.Options{})
	server.AddTransport(transport.GET{})
	server.AddTransport(transport.POST{})
	server.Use(extension.Introspection{})
	server.SetParserTokenLimit(10_000)
	present := gqlerrors.NewPresenter(config.Logger)
	server.SetErrorPresenter(present)
	server.SetRecoverFunc(func(ctx context.Context, recovered any) error {
		return fmt.Errorf("resolver panic: %v", recovered)
	})
	server.AroundResponses(subscriptionLoaders(posts, comments))
	server.AroundOperations(guardOperation(present, config, posts, comments))
	return &Server{Server: server, cancel: cancel}
}
