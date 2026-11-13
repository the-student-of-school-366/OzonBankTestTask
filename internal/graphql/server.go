package graphql

import (
	"context"
	"fmt"
	"time"

	"OzonBankTestTask/internal/graphql/generated"
	"OzonBankTestTask/internal/graphql/gqlerrors"
	"OzonBankTestTask/internal/graphql/limits"
	"OzonBankTestTask/internal/graphql/loader"
	"OzonBankTestTask/internal/graphql/ws"

	gql "github.com/99designs/gqlgen/graphql"
	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/99designs/gqlgen/graphql/handler/extension"
	"github.com/99designs/gqlgen/graphql/handler/transport"
	"github.com/vektah/gqlparser/v2/ast"
	"github.com/vektah/gqlparser/v2/gqlerror"
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
	server.AroundResponses(func(ctx context.Context, next gql.ResponseHandler) *gql.Response {
		operation := gql.GetOperationContext(ctx).Operation
		if operation != nil && operation.Operation == ast.Subscription {
			// Для обычного канала подписки middleware выполняется до вложенных
			// резолверов каждого события; кэш не живёт всё время соединения.
			ctx = loader.WithContext(ctx, loader.New(posts, comments))
		}
		return next(ctx)
	})
	server.AroundOperations(func(ctx context.Context, next gql.OperationHandler) gql.ResponseHandler {
		operation := gql.GetOperationContext(ctx)
		if err := limits.Check(operation, config.Limits); err != nil {
			return gql.OneShot(&gql.Response{Errors: gqlerror.List{present(ctx, err)}})
		}
		if operation.Operation.Operation == ast.Subscription {
			if connected, _ := ctx.Value(websocketKey{}).(bool); !connected {
				return gql.OneShot(&gql.Response{Errors: gqlerror.List{present(ctx, gqlerrors.New("BAD_USER_INPUT", "подписки доступны через WebSocket"))}})
			}
			return next(ctx)
		}
		ctx, cancel := context.WithTimeout(ctx, config.RequestTimeout)
		ctx = loader.WithContext(ctx, loader.New(posts, comments))
		response := next(ctx)
		done := false
		return func(ctx context.Context) *gql.Response {
			if done {
				return nil
			}
			done = true
			defer cancel()
			result := response(ctx)
			if err := ctx.Err(); err != nil {
				return &gql.Response{Errors: gqlerror.List{present(ctx, err)}}
			}
			return result
		}
	})
	return &Server{Server: server, cancel: cancel}
}
