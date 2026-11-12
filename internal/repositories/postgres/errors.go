package postgres

import (
	"context"
	"errors"
	"fmt"

	"OzonBankTestTask/internal/domain"
)

func Error(operation string, err error) error {
	if err == nil {
		return nil
	}
	var business domain.Error
	if errors.As(err, &business) || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return fmt.Errorf("postgres %s: %w", operation, err)
	}
	return fmt.Errorf("postgres %s: %w: %w", operation, domain.ErrStorage, err)
}
