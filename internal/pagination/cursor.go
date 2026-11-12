package pagination

import (
	"OzonBankTestTask/internal/domain/page"
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"time"

	"OzonBankTestTask/internal/domain"

	"github.com/google/uuid"
)

const MaxCursorLength = 512
const cursorVersion = 1

type Kind string

const (
	Posts   Kind = "posts"
	Roots   Kind = "roots"
	Replies Kind = "replies"
)

// Scope привязывает курсор к списку постов, корням поста или ответам родителя.
// У Posts ID должен быть нулевым; у Roots/Replies — ID поста/родителя.
type Scope struct {
	Kind Kind
	ID   uuid.UUID
}

func (s Scope) Validate() error {
	switch s.Kind {
	case Posts:
		if s.ID == uuid.Nil {
			return nil
		}
	case Roots, Replies:
		if s.ID != uuid.Nil {
			return nil
		}
	}
	return fmt.Errorf("%w: область выдачи", domain.ErrInvalidCursor)
}

type cursorPayload struct {
	Version         int        `json:"v"`
	Kind            Kind       `json:"kind"`
	ScopeID         *uuid.UUID `json:"scopeID,omitempty"`
	CreatedAtMicros *int64     `json:"createdAtMicros"`
	ID              uuid.UUID  `json:"id"`
}

// Encode не подписывает курсор: позиция сортировки не предоставляет права доступа.
func Encode(position page.Position, scope Scope) (string, error) {
	if err := scope.Validate(); err != nil {
		return "", err
	}
	if err := position.Validate(); err != nil {
		return "", fmt.Errorf("%w: %v", domain.ErrInvalidCursor, err)
	}
	micros := position.CreatedAt.UnixMicro()
	payload := cursorPayload{Version: cursorVersion, Kind: scope.Kind, CreatedAtMicros: &micros, ID: position.ID}
	if scope.Kind != Posts {
		id := scope.ID
		payload.ScopeID = &id
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("%w: %v", domain.ErrInvalidCursor, err)
	}
	return base64.RawURLEncoding.EncodeToString(data), nil
}

func Decode(cursor string, expected Scope) (page.Position, error) {
	if err := expected.Validate(); err != nil {
		return page.Position{}, err
	}
	if len(cursor) == 0 || len(cursor) > MaxCursorLength {
		return page.Position{}, domain.ErrInvalidCursor
	}
	data, err := base64.RawURLEncoding.Strict().DecodeString(cursor)
	if err != nil {
		return page.Position{}, domain.ErrInvalidCursor
	}
	var payload cursorPayload
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&payload); err != nil {
		return page.Position{}, domain.ErrInvalidCursor
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return page.Position{}, domain.ErrInvalidCursor
	}
	if payload.Version != cursorVersion || payload.Kind != expected.Kind || payload.CreatedAtMicros == nil {
		return page.Position{}, domain.ErrInvalidCursor
	}
	if expected.Kind == Posts {
		if payload.ScopeID != nil {
			return page.Position{}, domain.ErrInvalidCursor
		}
	} else if payload.ScopeID == nil || *payload.ScopeID != expected.ID {
		return page.Position{}, domain.ErrInvalidCursor
	}
	position := page.Position{CreatedAt: time.UnixMicro(*payload.CreatedAtMicros).UTC(), ID: payload.ID}
	if err := position.Validate(); err != nil {
		return page.Position{}, domain.ErrInvalidCursor
	}
	return position, nil
}
