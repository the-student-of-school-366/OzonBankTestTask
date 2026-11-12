package graphql

import (
	"context"
	"fmt"
	"log/slog"

	"OzonBankTestTask/internal/graphql/generated"
	"OzonBankTestTask/internal/graphql/gqlerrors"
	"OzonBankTestTask/internal/graphql/limits"
	"OzonBankTestTask/internal/graphql/loader"

	gql "github.com/99designs/gqlgen/graphql"
	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/99designs/gqlgen/graphql/handler/extension"
	"github.com/99designs/gqlgen/graphql/handler/transport"
	"github.com/vektah/gqlparser/v2/gqlerror"
)

// NewServer собирает GraphQL-исполнитель из проверенных зависимостей и настроек.
// HTTP-ограничения и метаданные запроса устанавливает api.NewHandler.
func NewServer(posts PostUseCase, comments CommentUseCase, budget limits.Config, logger *slog.Logger) *handler.Server {
	server := handler.New(generated.NewExecutableSchema(generated.Config{
		Resolvers: &Resolver{posts: posts, comments: comments},
	}))
	server.AddTransport(transport.Options{})
	server.AddTransport(transport.GET{})
	server.AddTransport(transport.POST{})
	server.Use(extension.Introspection{})
	server.SetParserTokenLimit(10_000)
	present := gqlerrors.NewPresenter(logger)
	server.SetErrorPresenter(present)
	server.SetRecoverFunc(func(ctx context.Context, recovered any) error {
		return fmt.Errorf("resolver panic: %v", recovered)
	})
	server.AroundOperations(func(ctx context.Context, next gql.OperationHandler) gql.ResponseHandler {
		if err := limits.Check(gql.GetOperationContext(ctx), budget); err != nil {
			return func(ctx context.Context) *gql.Response {
				return &gql.Response{Errors: gqlerror.List{present(ctx, err)}}
			}
		}
		ctx = loader.WithContext(ctx, loader.New(posts, comments))
		response := next(ctx)
		return func(ctx context.Context) *gql.Response {
			if err := ctx.Err(); err != nil {
				return &gql.Response{Errors: gqlerror.List{present(ctx, err)}}
			}
			return response(ctx)
		}
	})
	return server
}
