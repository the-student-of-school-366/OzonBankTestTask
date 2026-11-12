package post_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"OzonBankTestTask/internal/domain"
	"OzonBankTestTask/internal/domain/page"
	"OzonBankTestTask/internal/domain/post"
	"OzonBankTestTask/internal/dto"
	"OzonBankTestTask/internal/pagination"
	"OzonBankTestTask/internal/usecase"
	postusecase "OzonBankTestTask/internal/usecase/post"

	"github.com/google/uuid"
)

func TestPostCreate(t *testing.T) {
	for _, enabled := range []bool{true, false} {
		t.Run(fmt.Sprint(enabled), func(t *testing.T) {
			calls := 0
			stored := samplePost(t)
			repo := postRepositoryStub{create: func(ctx context.Context, value post.Post) (post.Post, error) {
				calls++
				if err := value.Validate(); err != nil {
					t.Fatal(err)
				}
				if value.Author != "actor" || value.Title != "title" || value.CommentsEnabled != enabled || !value.CreatedAt.Equal(fixedClock()) {
					t.Fatalf("неверные данные для записи: %+v", value)
				}
				return stored, nil
			}}
			u := postusecase.New(repo, usecase.WithClock(fixedClock))
			result, err := u.Create(context.Background(), "actor", dto.CreatePostInput{Title: "title", Content: "content", CommentsEnabled: &enabled})
			if err != nil {
				t.Fatal(err)
			}
			if calls != 1 || result != dto.PostFromDomain(stored) {
				t.Fatalf("нужен фактически сохранённый пост: %+v", result)
			}
		})
	}
}

func TestPostValidationBeforeStorage(t *testing.T) {
	u := postusecase.New(postRepositoryStub{}) // Любой вызов незаданных методов — ошибка теста.
	if _, err := u.Create(context.Background(), "author", dto.CreatePostInput{}); !errors.Is(err, domain.ErrEmptyPostTitle) {
		t.Fatal(err)
	}
	if _, err := u.Create(context.Background(), "", dto.CreatePostInput{Title: "t", Content: "c"}); !errors.Is(err, domain.ErrEmptyAuthor) {
		t.Fatal(err)
	}
	if _, err := u.GetMany(context.Background(), []string{uuid.NewString(), "bad"}); !errors.Is(err, domain.ErrInvalidID) {
		t.Fatal(err)
	}
	zero := 0
	if _, err := u.List(context.Background(), dto.PageInput{First: &zero}); !errors.Is(err, domain.ErrInvalidPageSize) {
		t.Fatal(err)
	}
	if _, err := u.SetCommentsEnabled(context.Background(), "author", dto.SetCommentsEnabledInput{PostID: "bad"}); !errors.Is(err, domain.ErrInvalidID) {
		t.Fatal(err)
	}
	if _, err := u.SetCommentsEnabled(context.Background(), "", dto.SetCommentsEnabledInput{PostID: uuid.NewString()}); !errors.Is(err, domain.ErrEmptyAuthor) {
		t.Fatal(err)
	}
}

func TestPostGetManyPreservesOrderMissingAndDuplicates(t *testing.T) {
	first, second := samplePost(t), samplePost(t)
	missing := uuid.New()
	calls := 0
	repo := postRepositoryStub{byID: func(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]post.Post, error) {
		calls++
		if len(ids) != 3 || ids[0] != second.ID || ids[1] != missing || ids[2] != first.ID {
			t.Fatalf("ключи не дедуплицированы: %v", ids)
		}
		return map[uuid.UUID]post.Post{first.ID: first, second.ID: second}, nil
	}}
	u := postusecase.New(repo)
	result, err := u.GetMany(context.Background(), []string{second.ID.String(), missing.String(), first.ID.String(), second.ID.String()})
	if err != nil {
		t.Fatal(err)
	}
	if calls != 1 || len(result) != 4 || result[0].ID != second.ID.String() || result[1] != nil || result[2].ID != first.ID.String() || result[3].ID != second.ID.String() {
		t.Fatalf("порядок: %+v", result)
	}
	result[0].Title = "changed"
	if result[3].Title == "changed" {
		t.Fatal("повторные ID разделяют DTO")
	}
}

func TestPostGetAndEmptyBatch(t *testing.T) {
	value := samplePost(t)
	u := postusecase.New(postRepositoryStub{byID: func(_ context.Context, ids []uuid.UUID) (map[uuid.UUID]post.Post, error) {
		return map[uuid.UUID]post.Post{value.ID: value}, nil
	}})
	got, err := u.Get(context.Background(), value.ID.String())
	if err != nil || got == nil || got.ID != value.ID.String() {
		t.Fatalf("пост: %+v, %v", got, err)
	}
	got, err = u.Get(context.Background(), uuid.NewString())
	if err != nil || got != nil {
		t.Fatalf("отсутствие: %+v, %v", got, err)
	}
	result, err := postusecase.New(postRepositoryStub{}).GetMany(context.Background(), nil)
	if err != nil || result == nil || len(result) != 0 {
		t.Fatalf("пустой batch: %v, %v", result, err)
	}
}

func TestPostListCursor(t *testing.T) {
	value := samplePost(t)
	position := value.PagePosition()
	cursor, err := pagination.Encode(position, pagination.Scope{Kind: pagination.Posts})
	if err != nil {
		t.Fatal(err)
	}
	first := 5
	repo := postRepositoryStub{list: func(_ context.Context, request page.Request) (page.Page[post.Post], error) {
		if request.First != first || request.After == nil || request.After.Compare(position) != 0 {
			t.Fatalf("параметры страницы: %+v", request)
		}
		return page.Page[post.Post]{Items: []post.Post{value}, End: &position, HasNextPage: true}, nil
	}}
	result, err := postusecase.New(repo).List(context.Background(), dto.PageInput{First: &first, After: &cursor})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Nodes) != 1 || !result.PageInfo.HasNextPage || result.PageInfo.EndCursor == nil || *result.PageInfo.EndCursor != cursor {
		t.Fatalf("страница: %+v", result)
	}
	wrong, err := pagination.Encode(position, pagination.Scope{Kind: pagination.Roots, ID: value.ID})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := postusecase.New(postRepositoryStub{}).List(context.Background(), dto.PageInput{After: &wrong}); !errors.Is(err, domain.ErrInvalidCursor) {
		t.Fatal(err)
	}
}

func TestPostSetCommentsEnabledIsOneAtomicCall(t *testing.T) {
	stored := samplePost(t)
	calls := 0
	repo := postRepositoryStub{set: func(_ context.Context, id uuid.UUID, actor string, enabled bool) (post.Post, error) {
		calls++
		if id != stored.ID || actor != "author" || enabled {
			t.Fatal("неверные параметры обновления")
		}
		stored.CommentsEnabled = enabled
		return stored, nil
	}}
	result, err := postusecase.New(repo).SetCommentsEnabled(context.Background(), "author", dto.SetCommentsEnabledInput{PostID: stored.ID.String(), Enabled: false})
	if err != nil || calls != 1 || result.CommentsEnabled {
		t.Fatalf("обновление: %+v, %v", result, err)
	}
	for _, want := range []error{domain.ErrNotAuthor, domain.ErrPostNotFound} {
		repo.set = func(context.Context, uuid.UUID, string, bool) (post.Post, error) {
			return post.Post{}, fmt.Errorf("update: %w", want)
		}
		if _, err := postusecase.New(repo).SetCommentsEnabled(context.Background(), "actor", dto.SetCommentsEnabledInput{PostID: stored.ID.String()}); !errors.Is(err, want) || errors.Is(err, domain.ErrStorage) {
			t.Fatalf("бизнес-ошибка: %v", err)
		}
	}
}

func TestPostStorageErrors(t *testing.T) {
	broken := errors.New("connection lost")
	repo := postRepositoryStub{
		create: func(context.Context, post.Post) (post.Post, error) { return post.Post{}, broken },
		byID:   func(context.Context, []uuid.UUID) (map[uuid.UUID]post.Post, error) { return nil, broken },
		list: func(context.Context, page.Request) (page.Page[post.Post], error) {
			return page.Page[post.Post]{}, broken
		},
		set: func(context.Context, uuid.UUID, string, bool) (post.Post, error) { return post.Post{}, broken },
	}
	u := postusecase.New(repo)
	_, createErr := u.Create(context.Background(), "actor", dto.CreatePostInput{Title: "t", Content: "c"})
	_, getErr := u.Get(context.Background(), uuid.NewString())
	_, listErr := u.List(context.Background(), dto.PageInput{})
	_, setErr := u.SetCommentsEnabled(context.Background(), "actor", dto.SetCommentsEnabledInput{PostID: uuid.NewString()})
	for _, err := range []error{createErr, getErr, listErr, setErr} {
		if !errors.Is(err, domain.ErrStorage) || !errors.Is(err, broken) {
			t.Fatalf("потеряна причина ошибки: %v", err)
		}
	}
	for _, want := range []error{context.Canceled, context.DeadlineExceeded} {
		repo.byID = func(context.Context, []uuid.UUID) (map[uuid.UUID]post.Post, error) {
			return nil, fmt.Errorf("read: %w", want)
		}
		if _, err := postusecase.New(repo).Get(context.Background(), uuid.NewString()); !errors.Is(err, want) || errors.Is(err, domain.ErrStorage) {
			t.Fatalf("контекст: %v", err)
		}
	}
}

func TestInvalidStoredPostCursorIsStorageError(t *testing.T) {
	u := postusecase.New(postRepositoryStub{list: func(context.Context, page.Request) (page.Page[post.Post], error) {
		return page.Page[post.Post]{End: &page.Position{CreatedAt: time.Now()}}, nil
	}})
	if _, err := u.List(context.Background(), dto.PageInput{}); !errors.Is(err, domain.ErrStorage) || errors.Is(err, domain.ErrInvalidCursor) {
		t.Fatalf("ошибка хранилища представлена как ошибка ввода: %v", err)
	}
}
