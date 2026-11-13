package comment

import (
	"context"

	"OzonBankTestTask/internal/domain/comment"
	"OzonBankTestTask/internal/repositories/postgres"
	"OzonBankTestTask/internal/repositories/postgres/queries"

	"github.com/google/uuid"
)

func (r *Repository) CommentsByID(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]comment.Comment, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	args, err := postgres.IDs(ids)
	if err != nil {
		return nil, err
	}
	result := make(map[uuid.UUID]comment.Comment, len(ids))
	if len(ids) == 0 {
		return result, nil
	}
	rows, err := r.pool.Query(ctx, queries.CommentsByID, args)
	if err != nil {
		return nil, postgres.Error("read comments", err)
	}
	defer rows.Close()
	for rows.Next() {
		value, err := postgres.ScanComment(rows)
		if err != nil {
			return nil, postgres.Error("scan comment", err)
		}
		result[value.ID] = value
	}
	if err = rows.Err(); err != nil {
		return nil, postgres.Error("read comments", err)
	}
	return result, nil
}
