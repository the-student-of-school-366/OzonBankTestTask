package graphql

import (
	"context"

	"OzonBankTestTask/internal/dto"
	"OzonBankTestTask/internal/graphql/loader"
)

//go:generate go tool mockery --case=snake --all --exclude=generated --exclude=loader --exclude=mocks

// PostUseCase возвращает результаты GetMany в порядке входных ID, включая nil для отсутствующих постов.
type PostUseCase interface {
	loader.PostReader
	Create(context.Context, string, dto.CreatePostInput) (dto.Post, error)
	List(context.Context, dto.PageInput) (dto.PostConnection, error)
	SetCommentsEnabled(context.Context, string, dto.SetCommentsEnabledInput) (dto.Post, error)
}

// CommentUseCase сохраняет порядок результатов batch-методов, включая повторные ID с разными страницами.
type CommentUseCase interface {
	loader.CommentReader
	Create(context.Context, string, dto.CreateCommentInput) (dto.Comment, error)
}
