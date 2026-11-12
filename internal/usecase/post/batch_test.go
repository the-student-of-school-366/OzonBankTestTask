package post_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"OzonBankTestTask/internal/domain/post"
	"OzonBankTestTask/internal/dto"
	"OzonBankTestTask/internal/usecase"
	postusecase "OzonBankTestTask/internal/usecase/post"

	"github.com/google/uuid"
)

func TestIDBatchLimitAndDeduplication(t *testing.T) {
	count := 2*usecase.MaxBatchSize + 1
	ids := make([]string, count)
	for i := range ids {
		ids[i] = uuid.NewString()
	}
	ids = append(ids, ids[0])
	calls := 0
	check := func(keys []uuid.UUID) {
		if len(keys) == 0 || len(keys) > usecase.MaxBatchSize {
			t.Fatalf("размер batch: %d", len(keys))
		}
		for i, key := range keys {
			if key.String() != ids[calls*usecase.MaxBatchSize+i] {
				t.Fatal("нарушен порядок ключей")
			}
		}
		calls++
	}
	posts := postusecase.New(postRepositoryStub{byID: func(_ context.Context, keys []uuid.UUID) (map[uuid.UUID]post.Post, error) {
		check(keys)
		values := make(map[uuid.UUID]post.Post, len(keys))
		for _, key := range keys {
			values[key] = post.Post{ID: key}
		}
		return values, nil
	}})
	postValues, err := posts.GetMany(context.Background(), ids)
	if err != nil || calls != 3 || len(postValues) != len(ids) {
		t.Fatalf("batch постов: calls=%d err=%v", calls, err)
	}
	for i, value := range postValues {
		if value.ID != ids[i] {
			t.Fatal("посты переставлены")
		}
	}
}

func TestCancellationBetweenBatches(t *testing.T) {
	ids := make([]string, usecase.MaxBatchSize+1)
	for i := range ids {
		ids[i] = uuid.NewString()
	}
	t.Run("IDs", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		calls := 0
		u := postusecase.New(postRepositoryStub{byID: func(context.Context, []uuid.UUID) (map[uuid.UUID]post.Post, error) {
			calls++
			cancel()
			return nil, nil
		}})
		result, err := u.GetMany(ctx, ids)
		if !errors.Is(err, context.Canceled) || result != nil || calls != 1 {
			t.Fatalf("отмена между batch: calls=%d err=%v", calls, err)
		}
	})
}

func TestCanceledContextDoesNotCallDependencies(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	clock := usecase.WithClock(func() time.Time {
		t.Fatal("часы вызваны после отмены контекста")
		return time.Time{}
	})
	posts := postusecase.New(postRepositoryStub{}, clock)
	checks := []func() error{
		func() error { _, err := posts.Create(ctx, "", dto.CreatePostInput{}); return err },
		func() error { _, err := posts.Get(ctx, ""); return err },
		func() error { _, err := posts.List(ctx, dto.PageInput{}); return err },
		func() error { _, err := posts.SetCommentsEnabled(ctx, "", dto.SetCommentsEnabledInput{}); return err },
	}
	for i, check := range checks {
		if err := check(); !errors.Is(err, context.Canceled) {
			t.Fatalf("проверка %d: %v", i, err)
		}
	}
}
