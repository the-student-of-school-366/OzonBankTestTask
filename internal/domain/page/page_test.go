package page_test

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"OzonBankTestTask/internal/domain"
	"OzonBankTestTask/internal/domain/page"
	"OzonBankTestTask/internal/domain/post"

	"github.com/google/uuid"
)

func TestNewPageRequest(t *testing.T) {
	position := page.Position{CreatedAt: time.Unix(100, 0).UTC(), ID: uuid.New()}
	req, err := page.NewPageRequest(page.DefaultPageSize, &position)
	if err != nil {
		t.Fatalf("запрос страницы: %v", err)
	}
	if req.First != page.DefaultPageSize || req.After == nil || req.After.Compare(position) != 0 {
		t.Fatalf("запрос %+v", req)
	}

	position.ID = uuid.Nil
	if req.After.ID == uuid.Nil {
		t.Fatal("позиция не скопирована")
	}
	first, err := page.NewPageRequest(1, nil)
	if err != nil {
		t.Fatalf("первая страница: %v", err)
	}
	if first.After != nil {
		t.Fatalf("первая страница имеет курсор: %v", first.After)
	}

	for _, limit := range []int{0, -1, page.MaxPageSize + 1} {
		if _, err := page.NewPageRequest(limit, nil); !errors.Is(err, domain.ErrInvalidPageSize) {
			t.Fatalf("limit=%d: ошибка %v", limit, err)
		}
	}
	if _, err := page.NewPageRequest(10, &position); !errors.Is(err, domain.ErrInvalidCursor) {
		t.Fatalf("нулевой ID курсора: %v", err)
	}
}

func TestCutPage(t *testing.T) {
	moment := time.Unix(100, 0).UTC()
	var posts []post.Post
	for i := 0; i < 3; i++ {
		posts = append(posts, post.Post{
			ID:        uuid.New(),
			Title:     fmt.Sprintf("пост %d", i),
			CreatedAt: moment.Add(time.Duration(3-i) * time.Second),
		})
	}

	result, err := page.CutPage(posts, 2)
	if err != nil {
		t.Fatal(err)
	}
	if !result.HasNextPage || len(result.Items) != 2 {
		t.Fatalf("страница: %+v", result)
	}
	if result.End == nil || result.End.Compare(posts[1].PagePosition()) != 0 {
		t.Fatalf("неверная конечная позиция: %v", result.End)
	}

	last, err := page.CutPage(posts[:2], 2)
	if err != nil {
		t.Fatal(err)
	}
	if last.HasNextPage || last.End == nil || last.End.ID != posts[1].ID {
		t.Fatalf("последняя страница: %+v", last)
	}

	empty, err := page.CutPage([]post.Post(nil), 2)
	if err != nil {
		t.Fatal(err)
	}
	if empty.HasNextPage || empty.End != nil || len(empty.Items) != 0 || empty.Items == nil {
		t.Fatalf("пустая страница: %+v", empty)
	}
	result.Items[0].ID = uuid.Nil
	if posts[0].ID == uuid.Nil {
		t.Fatal("страница разделяет срез с выборкой")
	}
	for _, size := range []int{0, -1, page.MaxPageSize + 1} {
		if _, err := page.CutPage(posts, size); !errors.Is(err, domain.ErrInvalidPageSize) {
			t.Fatalf("размер %d: %v", size, err)
		}
	}
}

func TestPositionOrder(t *testing.T) {
	moment := time.Unix(100, 0).UTC()
	smaller := page.Position{CreatedAt: moment, ID: uuid.MustParse("00000000-0000-4000-8000-000000000001")}
	larger := page.Position{CreatedAt: moment, ID: uuid.MustParse("00000000-0000-4000-8000-000000000002")}
	if smaller.Compare(larger) >= 0 || larger.Compare(smaller) <= 0 || smaller.Compare(smaller) != 0 {
		t.Fatal("неверный порядок UUID")
	}
	larger.CreatedAt = moment.Add(-time.Microsecond)
	if smaller.Compare(larger) <= 0 {
		t.Fatal("время должно предшествовать ID при сравнении")
	}
}

func TestSentinelIdentity(t *testing.T) {
	wrapped := fmt.Errorf("чтение поста: %w", domain.ErrPostNotFound)
	if !errors.Is(wrapped, domain.ErrPostNotFound) {
		t.Fatal("обёрнутая ошибка должна находиться через errors.Is")
	}
	if errors.Is(wrapped, domain.ErrCommentNotFound) {
		t.Fatal("другая доменная ошибка не должна совпадать")
	}
	if errors.Is(errors.New("пост не найден"), domain.ErrPostNotFound) {
		t.Fatal("совпадение текста не должно заменять sentinel")
	}
}
