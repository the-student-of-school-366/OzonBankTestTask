package comment

import (
	"context"
	"errors"

	"OzonBankTestTask/internal/domain"
	"OzonBankTestTask/internal/domain/comment"
	"OzonBankTestTask/internal/repositories/postgres"
	"OzonBankTestTask/internal/repositories/postgres/queries"

	"github.com/jackc/pgx/v5"
)

func (r *Repository) CreateComment(ctx context.Context, value comment.Comment) (comment.Comment, error) {
	if err := ctx.Err(); err != nil {
		return comment.Comment{}, err
	}
	if err := value.Validate(); err != nil {
		return comment.Comment{}, err
	}
	saved, err := postgres.InTransaction(ctx, r.pool, func(tx pgx.Tx) (comment.Comment, error) {
		post, err := postgres.ScanPost(tx.QueryRow(ctx, queries.PostForShare, postgres.UUID(value.PostID)))
		if errors.Is(err, pgx.ErrNoRows) {
			return comment.Comment{}, domain.ErrPostNotFound
		}
		if err != nil {
			return comment.Comment{}, err
		}
		if err = post.EnsureCommentsEnabled(); err != nil {
			return comment.Comment{}, err
		}
		var parent *comment.Comment
		if value.ParentID != nil {
			found, err := postgres.ScanComment(tx.QueryRow(ctx, queries.CommentByID, postgres.UUID(*value.ParentID)))
			if err != nil && !errors.Is(err, pgx.ErrNoRows) {
				return comment.Comment{}, err
			}
			if err == nil {
				parent = &found
			}
		}
		if err = comment.ValidateCommentPlacement(&post, parent, value); err != nil {
			return comment.Comment{}, err
		}
		return postgres.ScanComment(tx.QueryRow(ctx, queries.CommentCreate,
			postgres.UUID(value.ID), postgres.UUID(value.PostID), postgres.NullableUUID(value.ParentID), value.Author, value.Text, value.CreatedAt.UTC()))
	})
	return saved, postgres.Error("create comment", err)
}
