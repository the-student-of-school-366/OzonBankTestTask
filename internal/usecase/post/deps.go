package post

import (
	"context"

	"OzonBankTestTask/internal/domain/page"
	"OzonBankTestTask/internal/domain/post"

	"github.com/google/uuid"
)

//go:generate go tool mockery --case=snake --all

type Repository interface {
	CreatePost(ctx context.Context, value post.Post) (post.Post, error)
	PostsByID(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]post.Post, error)
	ListPosts(ctx context.Context, request page.Request) (page.Page[post.Post], error)
	SetCommentsEnabled(ctx context.Context, postID uuid.UUID, actorID string, enabled bool) (post.Post, error)
}
