package gqlerrors

import (
	"context"
	"errors"
	"log/slog"

	"OzonBankTestTask/internal/domain"
	"OzonBankTestTask/internal/requestctx"
	"OzonBankTestTask/internal/usecase/subscription"

	"github.com/99designs/gqlgen/graphql"
	"github.com/vektah/gqlparser/v2/gqlerror"
)

// NewPresenter преобразует ошибки приложения в безопасный GraphQL-ответ.
func NewPresenter(logger *slog.Logger) graphql.ErrorPresenterFunc {
	return func(ctx context.Context, err error) *gqlerror.Error {
		presented := graphql.DefaultErrorPresenter(ctx, err)
		// Не менять исходную ошибку: loader может вернуть один error нескольким резолверам параллельно.
		result := &gqlerror.Error{Path: presented.Path, Locations: presented.Locations}
		code, message := "INTERNAL", "внутренняя ошибка сервера"
		var business domain.Error
		var clientError *gqlerror.Error
		switch {
		case errors.Is(err, subscription.ErrLimitReached):
			code, message = "RESOURCE_EXHAUSTED", subscription.ErrLimitReached.Error()
		case errors.Is(err, subscription.ErrClosed):
			code, message = "UNAVAILABLE", subscription.ErrClosed.Error()
		case errors.Is(err, requestctx.ErrUnauthenticated):
			code, message = "UNAUTHENTICATED", requestctx.ErrUnauthenticated.Error()
		case errors.Is(err, context.DeadlineExceeded):
			code, message = "TIMEOUT", "превышено время выполнения запроса"
		case errors.Is(err, context.Canceled):
			code, message = "CANCELED", "запрос отменён"
		case errors.Is(err, domain.ErrStorage):
			// Любые детали ошибки хранилища остаются в журнале.
		case errors.As(err, &business):
			message = business.Error()
			switch business {
			case domain.ErrPostNotFound, domain.ErrCommentNotFound:
				code = "NOT_FOUND"
			case domain.ErrNotAuthor:
				code = "FORBIDDEN"
			case domain.ErrCommentsDisabled:
				code = "COMMENTS_DISABLED"
			case domain.ErrParentPostMismatch:
				code = "PARENT_POST_MISMATCH"
			case domain.ErrEmptyCommentText, domain.ErrCommentTooLong, domain.ErrEmptyPostTitle,
				domain.ErrEmptyPostContent, domain.ErrEmptyAuthor, domain.ErrInvalidText,
				domain.ErrInvalidID, domain.ErrSelfParent, domain.ErrInvalidPageSize, domain.ErrInvalidCursor:
				code = "BAD_USER_INPUT"
			default:
				message = "внутренняя ошибка сервера"
			}
		case errors.As(err, &clientError):
			// Ошибки парсинга/валидации до исполнения резолверов безопасны для клиента.
			if len(clientError.Path) == 0 {
				code, message = "BAD_USER_INPUT", clientError.Message
			}
			if value, ok := clientError.Extensions["code"].(string); ok {
				switch value {
				case "GRAPHQL_PARSE_FAILED", "GRAPHQL_VALIDATION_FAILED", "QUERY_TOO_COMPLEX", "QUERY_TOO_DEEP":
					code, message = value, clientError.Message
				}
			}
		}
		if code == "INTERNAL" {
			logger.ErrorContext(ctx, "GraphQL operation failed", "request_id", requestctx.RequestID(ctx), "error", err)
		}
		result.Message = message
		result.Extensions = map[string]any{"code": code, "requestId": requestctx.RequestID(ctx)}
		return result
	}
}

// New создаёт ошибку проверки GraphQL-операции до вызова резолверов.
func New(code, message string) *gqlerror.Error {
	return &gqlerror.Error{Message: message, Extensions: map[string]any{"code": code}}
}
