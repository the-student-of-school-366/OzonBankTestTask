package api

import (
	"context"

	"OzonBankTestTask/internal/requestctx"
)

// WithActor передаёт идентичность от доверенного middleware. Приоритет выше демонстрационного заголовка.
func WithActor(ctx context.Context, actorID string) context.Context {
	return requestctx.WithActor(ctx, actorID)
}
