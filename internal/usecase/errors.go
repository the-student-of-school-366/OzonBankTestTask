package usecase

import (
	"context"
	"errors"
	"fmt"

	"OzonBankTestTask/internal/domain"
)

// StorageError сохраняет бизнес-ошибки и отмену контекста, а технические ошибки
// дополнительно помечает ErrStorage для последующего отображения в INTERNAL.
func StorageError(operation string, err error) error {
	var businessError domain.Error
	if errors.As(err, &businessError) || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return fmt.Errorf("%s: %w", operation, err)
	}
	return fmt.Errorf("%s: %w: %w", operation, domain.ErrStorage, err)
}
