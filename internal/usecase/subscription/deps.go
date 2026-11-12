package subscription

import (
	"context"

	"OzonBankTestTask/internal/domain/comment"
	"OzonBankTestTask/internal/dto"

	"github.com/google/uuid"
)

//go:generate go tool mockery --case=snake --all --exclude=mocks

type PostReader interface {
	GetMany(context.Context, []string) ([]*dto.Post, error)
}

// Stream принадлежит брокеру. Cancel идемпотентен, Done закрывается при любой отписке.
type Stream struct {
	Comments <-chan comment.Comment
	Done     <-chan struct{}
	Cancel   func()
}

type Broker interface {
	Subscribe(context.Context, uuid.UUID) (Stream, error)
}
