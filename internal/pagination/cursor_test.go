package pagination

import (
	"OzonBankTestTask/internal/domain/page"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"OzonBankTestTask/internal/domain"

	"github.com/google/uuid"
)

func TestCursorRoundTrip(t *testing.T) {
	for _, scope := range []Scope{{Kind: Posts}, {Kind: Roots, ID: uuid.New()}, {Kind: Replies, ID: uuid.New()}} {
		t.Run(string(scope.Kind), func(t *testing.T) {
			for _, moment := range []time.Time{time.UnixMicro(0), time.UnixMicro(-1), time.Date(2026, 11, 12, 10, 0, 0, 123456000, time.FixedZone("MSK", 10800))} {
				position := page.Position{CreatedAt: moment, ID: uuid.New()}
				cursor, err := Encode(position, scope)
				if err != nil {
					t.Fatal(err)
				}
				if len(cursor) > MaxCursorLength {
					t.Fatal("слишком длинный курсор")
				}
				decoded, err := Decode(cursor, scope)
				if err != nil {
					t.Fatal(err)
				}
				if decoded.Compare(position) != 0 || decoded.CreatedAt.Location() != time.UTC {
					t.Fatalf("позиция изменилась: %v -> %v", position, decoded)
				}
			}
		})
	}
}

func TestCursorScope(t *testing.T) {
	scope := Scope{Kind: Roots, ID: uuid.New()}
	position := page.Position{CreatedAt: time.Unix(100, 0), ID: uuid.New()}
	cursor, err := Encode(position, scope)
	if err != nil {
		t.Fatal(err)
	}
	for _, other := range []Scope{{Kind: Posts}, {Kind: Roots, ID: uuid.New()}, {Kind: Replies, ID: scope.ID}} {
		if _, err := Decode(cursor, other); !errors.Is(err, domain.ErrInvalidCursor) {
			t.Fatalf("чужая область %v принята: %v", other, err)
		}
	}
	for _, invalid := range []Scope{{}, {Kind: "unknown"}, {Kind: Roots}, {Kind: Replies}, {Kind: Posts, ID: uuid.New()}} {
		if _, err := Encode(position, invalid); !errors.Is(err, domain.ErrInvalidCursor) {
			t.Fatalf("неверная область принята: %v", invalid)
		}
		if _, err := Decode(cursor, invalid); !errors.Is(err, domain.ErrInvalidCursor) {
			t.Fatalf("неверная область декодирования: %v", invalid)
		}
	}
}

func TestRejectInvalidCursor(t *testing.T) {
	scope := Scope{Kind: Posts}
	valid := map[string]any{"v": 1, "kind": "posts", "createdAtMicros": int64(1000000), "id": uuid.New().String()}
	mutations := []struct {
		name   string
		change func(map[string]any)
	}{
		{"version", func(p map[string]any) { p["v"] = 2 }},
		{"missing version", func(p map[string]any) { delete(p, "v") }},
		{"kind", func(p map[string]any) { p["kind"] = "roots" }},
		{"unexpected scope", func(p map[string]any) { p["scopeID"] = uuid.New().String() }},
		{"missing time", func(p map[string]any) { delete(p, "createdAtMicros") }},
		{"null time", func(p map[string]any) { p["createdAtMicros"] = nil }},
		{"fractional time", func(p map[string]any) { p["createdAtMicros"] = 0.5 }},
		{"year out of range", func(p map[string]any) {
			p["createdAtMicros"] = time.Date(10000, 1, 1, 0, 0, 0, 0, time.UTC).UnixMicro()
		}},
		{"zero time", func(p map[string]any) { p["createdAtMicros"] = time.Time{}.UnixMicro() }},
		{"invalid id", func(p map[string]any) { p["id"] = "bad" }},
		{"nil id", func(p map[string]any) { p["id"] = uuid.Nil.String() }},
		{"missing id", func(p map[string]any) { delete(p, "id") }},
		{"unknown field", func(p map[string]any) { p["offset"] = 20 }},
	}
	for _, tt := range mutations {
		t.Run(tt.name, func(t *testing.T) {
			payload := make(map[string]any, len(valid))
			for key, value := range valid {
				payload[key] = value
			}
			tt.change(payload)
			data, err := json.Marshal(payload)
			if err != nil {
				t.Fatal(err)
			}
			cursor := base64.RawURLEncoding.EncodeToString(data)
			if _, err := Decode(cursor, scope); !errors.Is(err, domain.ErrInvalidCursor) {
				t.Fatalf("принят неверный курсор: %v", err)
			}
		})
	}
	data, err := json.Marshal(valid)
	if err != nil {
		t.Fatal(err)
	}
	for _, cursor := range []string{"", "!!!", strings.Repeat("a", MaxCursorLength+1), base64.RawURLEncoding.EncodeToString([]byte("null")), base64.RawURLEncoding.EncodeToString(append(data, []byte(" {}")...))} {
		if _, err := Decode(cursor, scope); !errors.Is(err, domain.ErrInvalidCursor) {
			t.Fatalf("принят неверный курсор: %v", err)
		}
	}
	for _, position := range []page.Position{
		{},
		{CreatedAt: time.Unix(1, 1), ID: uuid.New()},
		{CreatedAt: time.Date(9999, 12, 31, 23, 0, 0, 0, time.FixedZone("west", -3600)), ID: uuid.New()},
	} {
		if _, err := Encode(position, scope); !errors.Is(err, domain.ErrInvalidCursor) {
			t.Fatalf("принята неверная позиция: %v", err)
		}
	}
}

func FuzzDecodeCursor(f *testing.F) {
	scope := Scope{Kind: Posts}
	cursor, err := Encode(page.Position{CreatedAt: time.Unix(100, 0), ID: uuid.New()}, scope)
	if err != nil {
		f.Fatal(err)
	}
	f.Add(cursor)
	f.Add("")
	f.Add("invalid")
	f.Fuzz(func(t *testing.T, cursor string) {
		position, err := Decode(cursor, scope)
		if err != nil {
			if !errors.Is(err, domain.ErrInvalidCursor) {
				t.Fatal(err)
			}
			return
		}
		encoded, err := Encode(position, scope)
		if err != nil {
			t.Fatal(err)
		}
		decoded, err := Decode(encoded, scope)
		if err != nil || position.Compare(decoded) != 0 {
			t.Fatalf("round-trip: %v", err)
		}
	})
}
