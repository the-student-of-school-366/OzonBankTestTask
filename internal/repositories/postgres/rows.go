package postgres

import (
	"fmt"

	"OzonBankTestTask/internal/domain"
	"OzonBankTestTask/internal/domain/comment"
	"OzonBankTestTask/internal/domain/post"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

func UUID(id uuid.UUID) pgtype.UUID { return pgtype.UUID{Bytes: id, Valid: true} }

func NullableUUID(id *uuid.UUID) pgtype.UUID {
	if id == nil {
		return pgtype.UUID{}
	}
	return UUID(*id)
}

func ScanPost(row pgx.Row) (post.Post, error) {
	var value post.Post
	var id pgtype.UUID
	if err := row.Scan(&id, &value.Title, &value.Content, &value.Author, &value.CommentsEnabled, &value.CreatedAt); err != nil {
		return post.Post{}, err
	}
	value.ID = uuid.UUID(id.Bytes)
	value.CreatedAt = value.CreatedAt.UTC()
	if err := value.Validate(); err != nil {
		return post.Post{}, fmt.Errorf("%w: invalid persisted post: %v", domain.ErrStorage, err)
	}
	return value, nil
}

// CommentRow допускает NULL во всех колонках для пустой страницы LEFT JOIN LATERAL.
type CommentRow struct {
	ID, PostID, ParentID pgtype.UUID
	Author, Text         pgtype.Text
	CreatedAt            pgtype.Timestamptz
}

func (r *CommentRow) Destinations() []any {
	return []any{&r.ID, &r.PostID, &r.ParentID, &r.Author, &r.Text, &r.CreatedAt}
}

func (r CommentRow) Value() (comment.Comment, error) {
	value := comment.Comment{ID: uuid.UUID(r.ID.Bytes), PostID: uuid.UUID(r.PostID.Bytes), Author: r.Author.String, Text: r.Text.String, CreatedAt: r.CreatedAt.Time.UTC()}
	if r.ParentID.Valid {
		id := uuid.UUID(r.ParentID.Bytes)
		value.ParentID = &id
	}
	if err := value.Validate(); err != nil {
		return comment.Comment{}, fmt.Errorf("%w: invalid persisted comment: %v", domain.ErrStorage, err)
	}
	return value, nil
}

func ScanComment(row pgx.Row) (comment.Comment, error) {
	var record CommentRow
	if err := row.Scan(record.Destinations()...); err != nil {
		return comment.Comment{}, err
	}
	return record.Value()
}
