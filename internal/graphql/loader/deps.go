package loader

import (
	"context"

	"OzonBankTestTask/internal/dto"
)

//go:generate go tool mockery --case=snake --all --exclude=mocks

// PostReader возвращает результаты в порядке ID; отсутствие представлено nil.
type PostReader interface {
	GetMany(context.Context, []string) ([]*dto.Post, error)
}

// CommentReader сохраняет порядок ID/страниц, включая одинаковых владельцев с разными параметрами.
type CommentReader interface {
	GetMany(context.Context, []string) ([]*dto.Comment, error)
	ListRootsBatch(context.Context, []dto.RootPageInput) ([]dto.CommentConnection, error)
	ListRepliesBatch(context.Context, []dto.ReplyPageInput) ([]dto.CommentConnection, error)
}
