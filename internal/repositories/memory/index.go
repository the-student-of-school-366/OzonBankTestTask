package memory

import (
	"slices"
	"sort"

	"OzonBankTestTask/internal/domain"
	"OzonBankTestTask/internal/domain/page"

	"github.com/google/uuid"
)

// Индекс отсортирован по убыванию (created_at, id).
func insertPosition(index []page.Position, position page.Position) []page.Position {
	i := sort.Search(len(index), func(i int) bool { return index[i].Compare(position) < 0 })
	return slices.Insert(index, i, position)
}

func positionsForPage(index []page.Position, request page.Request) []page.Position {
	start := 0
	if request.After != nil {
		start = sort.Search(len(index), func(i int) bool { return index[i].Compare(*request.After) < 0 })
	}
	return index[start:min(len(index), start+request.First+1)]
}

func validateIDs(ids []uuid.UUID) error {
	if len(ids) > page.MaxBatchSize {
		return domain.ErrInvalidPageSize
	}
	for _, id := range ids {
		if id == uuid.Nil {
			return domain.ErrInvalidID
		}
	}
	return nil
}
