package post

import (
	"context"
	"errors"

	"OzonBankTestTask/internal/domain"
	"OzonBankTestTask/internal/domain/post"
	"OzonBankTestTask/internal/repositories/postgres"
	"OzonBankTestTask/internal/repositories/postgres/queries"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func (r *Repository) SetCommentsEnabled(ctx context.Context, id uuid.UUID, actorID string, enabled bool) (post.Post, error) {
	if err := ctx.Err(); err != nil {
		return post.Post{}, err
	}
	if id == uuid.Nil {
		return post.Post{}, domain.ErrInvalidID
	}
	if err := domain.ValidateAuthor(actorID); err != nil {
		return post.Post{}, err
	}
	value, err := postgres.InTransaction(ctx, r.pool, func(tx pgx.Tx) (post.Post, error) {
		value, err := postgres.ScanPost(tx.QueryRow(ctx, queries.PostForUpdate, postgres.UUID(id)))
		if errors.Is(err, pgx.ErrNoRows) {
			return post.Post{}, domain.ErrPostNotFound
		}
		if err != nil {
			return post.Post{}, err
		}
		if err = value.SetCommentsEnabled(actorID, enabled); err != nil {
			return post.Post{}, err
		}
		return postgres.ScanPost(tx.QueryRow(ctx, queries.PostSetComments, postgres.UUID(id), value.CommentsEnabled))
	})
	return value, postgres.Error("set comments enabled", err)
}
