package subscription

import (
	"context"

	"OzonBankTestTask/internal/dto"
)

//go:generate go tool mockery --case=snake --all --exclude=mocks

type PostReader interface {
	GetMany(context.Context, []string) ([]*dto.Post, error)
}
