package usecase

import (
	"context"
	"fmt"

	"OzonBankTestTask/internal/domain"
	"OzonBankTestTask/internal/dto"

	"github.com/google/uuid"
)

// MaxBatchSize ограничивает число ключей одного вызова репозитория.
const MaxBatchSize = 100

func ParseIDs(ids []string) ([]uuid.UUID, error) {
	parsed := make([]uuid.UUID, len(ids))
	for i, id := range ids {
		value, err := dto.ParseID(id)
		if err != nil {
			return nil, fmt.Errorf("id[%d]: %w", i, err)
		}
		parsed[i] = value
	}
	return parsed, nil
}

func LoadByIDs[T any](ctx context.Context, ids []uuid.UUID, fetch func(context.Context, []uuid.UUID) (map[uuid.UUID]T, error)) (map[uuid.UUID]T, error) {
	unique := make([]uuid.UUID, 0, len(ids))
	seen := make(map[uuid.UUID]struct{}, len(ids))
	for _, id := range ids {
		if _, exists := seen[id]; !exists {
			seen[id] = struct{}{}
			unique = append(unique, id)
		}
	}
	result := make(map[uuid.UUID]T, len(unique))
	for start := 0; start < len(unique); start += MaxBatchSize {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		values, err := fetch(ctx, unique[start:min(start+MaxBatchSize, len(unique))])
		if err != nil {
			return nil, err
		}
		for id, value := range values {
			result[id] = value
		}
	}
	return result, nil
}

func LoadPages[Request, Result any](ctx context.Context, requests []Request, fetch func(context.Context, []Request) ([]Result, error)) ([]Result, error) {
	result := make([]Result, 0, len(requests))
	for start := 0; start < len(requests); start += MaxBatchSize {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		batch := requests[start:min(start+MaxBatchSize, len(requests))]
		values, err := fetch(ctx, batch)
		if err != nil {
			return nil, err
		}
		if len(values) != len(batch) {
			return nil, fmt.Errorf("%w: репозиторий вернул %d страниц для %d запросов", domain.ErrStorage, len(values), len(batch))
		}
		result = append(result, values...)
	}
	return result, nil
}
