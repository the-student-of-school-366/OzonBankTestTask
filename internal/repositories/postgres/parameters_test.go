package postgres_test

import (
	"testing"
	"time"

	"OzonBankTestTask/internal/domain"
	"OzonBankTestTask/internal/domain/page"
	"OzonBankTestTask/internal/repositories/postgres"
	"OzonBankTestTask/internal/usecase"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/require"
)

func TestPageParametersKeepDuplicatesAndNullableCursors(t *testing.T) {
	id := uuid.New()
	after := page.Position{ID: uuid.New(), CreatedAt: domain.NormalizeTime(time.Now())}
	args, err := postgres.PageParameters([]uuid.UUID{id, id}, []page.Request{{First: 1}, {First: 2, After: &after}})
	require.NoError(t, err)
	require.Equal(t, []pgtype.UUID{postgres.UUID(id), postgres.UUID(id)}, args[0])
	require.Equal(t, []int32{1, 2}, args[1])
	require.Equal(t, []pgtype.Timestamptz{{}, {Time: after.CreatedAt, Valid: true}}, args[2])
	require.Equal(t, []pgtype.UUID{{}, postgres.UUID(after.ID)}, args[3])
}

func TestBatchInputRejectedBeforeQuery(t *testing.T) {
	_, err := postgres.IDs(make([]uuid.UUID, usecase.MaxBatchSize+1))
	require.ErrorIs(t, err, domain.ErrInvalidPageSize)
	_, err = postgres.IDs([]uuid.UUID{uuid.Nil})
	require.ErrorIs(t, err, domain.ErrInvalidID)
	_, err = postgres.PageParameters([]uuid.UUID{uuid.New()}, nil)
	require.ErrorIs(t, err, domain.ErrInvalidPageSize)
	_, err = postgres.PageParameters([]uuid.UUID{uuid.New()}, []page.Request{{First: 101}})
	require.ErrorIs(t, err, domain.ErrInvalidPageSize)
	_, err = postgres.PageParameters([]uuid.UUID{uuid.New()}, []page.Request{{First: 1, After: &page.Position{}}})
	require.ErrorIs(t, err, domain.ErrInvalidCursor)
}
