package postgres

import (
	"OzonBankTestTask/internal/domain"
	"OzonBankTestTask/internal/domain/page"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

func IDs(ids []uuid.UUID) ([]pgtype.UUID, error) {
	if len(ids) > page.MaxBatchSize {
		return nil, domain.ErrInvalidPageSize
	}
	result := make([]pgtype.UUID, len(ids))
	for i, id := range ids {
		if id == uuid.Nil {
			return nil, domain.ErrInvalidID
		}
		result[i] = pgtype.UUID{Bytes: id, Valid: true}
	}
	return result, nil
}

// PageParameters сохраняет порядок и дубликаты: SQL использует WITH ORDINALITY.
func PageParameters(ids []uuid.UUID, pages []page.Request) ([]any, error) {
	values, err := IDs(ids)
	if err != nil {
		return nil, err
	}
	if len(ids) != len(pages) {
		return nil, domain.ErrInvalidPageSize
	}
	first := make([]int32, len(pages))
	times := make([]pgtype.Timestamptz, len(pages))
	cursors := make([]pgtype.UUID, len(pages))
	for i, request := range pages {
		if _, err := page.NewPageRequest(request.First, request.After); err != nil {
			return nil, err
		}
		first[i] = int32(request.First)
		if request.After != nil {
			times[i] = pgtype.Timestamptz{Time: request.After.CreatedAt.UTC(), Valid: true}
			cursors[i] = pgtype.UUID{Bytes: request.After.ID, Valid: true}
		}
	}
	return []any{values, first, times, cursors}, nil
}
