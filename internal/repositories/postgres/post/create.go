package post

import (
	"context"

	"OzonBankTestTask/internal/domain/post"
	"OzonBankTestTask/internal/repositories/postgres"
	"OzonBankTestTask/internal/repositories/postgres/queries"
)

func (r *Repository) CreatePost(ctx context.Context, value post.Post) (post.Post, error) {
	if err := ctx.Err(); err != nil {
		return post.Post{}, err
	}
	if err := value.Validate(); err != nil {
		return post.Post{}, err
	}
	saved, err := postgres.ScanPost(r.pool.QueryRow(ctx, queries.PostCreate,
		postgres.UUID(value.ID), value.Title, value.Content, value.Author, value.CommentsEnabled, value.CreatedAt.UTC()))
	return saved, postgres.Error("create post", err)
}
