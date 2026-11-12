package page

import (
	"OzonBankTestTask/internal/domain"
	"bytes"
	"time"

	"github.com/google/uuid"
)

const DefaultPageSize = 20
const MaxPageSize = 100

// Position — неизменяемая пара сортировки (created_at, id).
// Следующая страница при сортировке DESC содержит строго меньшие пары.
type Position struct {
	CreatedAt time.Time
	ID        uuid.UUID
}

func (p Position) Validate() error {
	if p.ID == uuid.Nil {
		return domain.ErrInvalidID
	}
	return domain.ValidateTime(p.CreatedAt)
}

// Compare задаёт возрастающий порядок пар; отрицательное значение означает p < other.
func (p Position) Compare(other Position) int {
	if order := p.CreatedAt.Compare(other.CreatedAt); order != 0 {
		return order
	}
	return bytes.Compare(p.ID[:], other.ID[:])
}

type Request struct {
	First int
	After *Position
}

func NewPageRequest(first int, after *Position) (Request, error) {
	if first <= 0 || first > MaxPageSize {
		return Request{}, domain.ErrInvalidPageSize
	}
	if after != nil {
		if err := after.Validate(); err != nil {
			return Request{}, domain.ErrInvalidCursor
		}
		positionCopy := *after
		positionCopy.CreatedAt = positionCopy.CreatedAt.UTC()
		after = &positionCopy
	}
	return Request{First: first, After: after}, nil
}

// Page содержит страницу без транспортного представления курсора.
type Page[T any] struct {
	Items       []T
	End         *Position
	HasNextPage bool
}

type Item interface {
	PagePosition() Position
}

// CutPage обрезает отсортированную выборку по правилу LIMIT first+1.
// Копируется срез; за независимость вложенных полей элементов отвечает хранилище.
func CutPage[T Item](fetched []T, first int) (Page[T], error) {
	if first <= 0 || first > MaxPageSize {
		return Page[T]{}, domain.ErrInvalidPageSize
	}
	count := min(len(fetched), first)
	items := make([]T, count)
	copy(items, fetched[:count])
	page := Page[T]{Items: items, HasNextPage: len(fetched) > first}
	if count > 0 {
		position := items[count-1].PagePosition()
		if err := position.Validate(); err != nil {
			return Page[T]{}, err
		}
		position.CreatedAt = position.CreatedAt.UTC()
		page.End = &position
	}
	return page, nil
}
