package usecase

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"OzonBankTestTask/internal/domain"
	"OzonBankTestTask/internal/requestctx"
)

// StorageError сохраняет бизнес-ошибки и отмену контекста, а технические ошибки
// дополнительно помечает ErrStorage для последующего отображения в INTERNAL.
// Технический сбой хранилища записывается в журнал вместе с идентификатором запроса.
func StorageError(ctx context.Context, operation string, err error) error {
	var businessError domain.Error
	if errors.As(err, &businessError) || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return fmt.Errorf("%s: %w", operation, err)
	}
	slog.ErrorContext(ctx, "storage operation failed",
		"operation", operation,
		"request_id", requestctx.RequestID(ctx),
		"error", err,
	)
	return fmt.Errorf("%s: %w: %w", operation, domain.ErrStorage, err)
}
