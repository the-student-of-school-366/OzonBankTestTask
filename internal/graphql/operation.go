package graphql

import (
	"context"

	"OzonBankTestTask/internal/graphql/gqlerrors"
	"OzonBankTestTask/internal/graphql/limits"
	"OzonBankTestTask/internal/graphql/loader"

	gql "github.com/99designs/gqlgen/graphql"
	"github.com/vektah/gqlparser/v2/ast"
	"github.com/vektah/gqlparser/v2/gqlerror"
)

// subscriptionLoaders создаёт кэш загрузчиков перед вложенными резолверами
// каждого события. Для обычного канала подписки middleware выполняется до этих
// резолверов, поэтому кэш не живёт всё время соединения.
func subscriptionLoaders(posts PostUseCase, comments CommentUseCase) gql.ResponseMiddleware {
	return func(ctx context.Context, next gql.ResponseHandler) *gql.Response {
		operation := gql.GetOperationContext(ctx).Operation
		if operation != nil && operation.Operation == ast.Subscription {
			ctx = loader.WithContext(ctx, loader.New(posts, comments))
		}
		return next(ctx)
	}
}

// guardOperation проверяет бюджет запроса до резолверов.
// Подписка допустима только после WebSocket upgrade и не получает таймаут операции.
// Query и mutation получают deadline и кэш загрузчиков на один ответ.
func guardOperation(present gql.ErrorPresenterFunc, config Config, posts PostUseCase, comments CommentUseCase) gql.OperationMiddleware {
	return func(ctx context.Context, next gql.OperationHandler) gql.ResponseHandler {
		operation := gql.GetOperationContext(ctx)
		if err := limits.Check(operation, config.Limits); err != nil {
			return rejectOperation(present, ctx, err)
		}
		if operation.Operation.Operation == ast.Subscription {
			if connected, _ := ctx.Value(websocketKey{}).(bool); !connected {
				return rejectOperation(present, ctx, gqlerrors.New("BAD_USER_INPUT", "подписки доступны через WebSocket"))
			}
			return next(ctx)
		}
		ctx, cancel := context.WithTimeout(ctx, config.RequestTimeout)
		ctx = loader.WithContext(ctx, loader.New(posts, comments))
		return finishOnce(cancel, present, next(ctx))
	}
}

func rejectOperation(present gql.ErrorPresenterFunc, ctx context.Context, err error) gql.ResponseHandler {
	return gql.OneShot(&gql.Response{Errors: gqlerror.List{present(ctx, err)}})
}

// finishOnce выполняет ответ один раз и снимает deadline операции.
// Ошибку проверяет контекст, которым gqlgen вызывает ответ, а не контекст резолверов.
func finishOnce(cancel context.CancelFunc, present gql.ErrorPresenterFunc, response gql.ResponseHandler) gql.ResponseHandler {
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
}
