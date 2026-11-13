package comment_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"OzonBankTestTask/internal/domain/comment"
	"OzonBankTestTask/internal/domain/page"
	"OzonBankTestTask/internal/dto"
	"OzonBankTestTask/internal/usecase"
	commentusecase "OzonBankTestTask/internal/usecase/comment"

	"github.com/google/uuid"
)

func TestIDBatchLimitAndDeduplication(t *testing.T) {
	count := 2*page.MaxBatchSize + 1
	ids := make([]string, count)
	for i := range ids {
		ids[i] = uuid.NewString()
	}
	ids = append(ids, ids[0])
	calls := 0
	check := func(keys []uuid.UUID) {
		if len(keys) == 0 || len(keys) > page.MaxBatchSize {
			t.Fatalf("размер batch: %d", len(keys))
		}
		for i, key := range keys {
			if key.String() != ids[calls*page.MaxBatchSize+i] {
				t.Fatal("нарушен порядок ключей")
			}
		}
		calls++
	}
	comments := commentusecase.New(commentRepositoryStub{byID: func(_ context.Context, keys []uuid.UUID) (map[uuid.UUID]comment.Comment, error) {
		check(keys)
		values := make(map[uuid.UUID]comment.Comment, len(keys))
		for _, key := range keys {
			values[key] = comment.Comment{ID: key}
		}
		return values, nil
	}}, nil)
	commentValues, err := comments.GetMany(context.Background(), ids)
	if err != nil || calls != 3 || len(commentValues) != len(ids) {
		t.Fatalf("batch комментариев: calls=%d err=%v", calls, err)
	}
	for i, value := range commentValues {
		if value.ID != ids[i] {
			t.Fatal("комментарии переставлены")
		}
	}
}

func TestPageBatchLimitAndEmptyInputs(t *testing.T) {
	count := 2*page.MaxBatchSize + 1
	roots := make([]dto.RootPageInput, count)
	replies := make([]dto.ReplyPageInput, count)
	for i := range roots {
		roots[i].PostID = uuid.NewString()
		replies[i].ParentID = uuid.NewString()
	}
	rootCalls, replyCalls := 0, 0
	u := commentusecase.New(commentRepositoryStub{
		roots: func(_ context.Context, requests []commentusecase.RootPageRequest) ([]page.Page[comment.Comment], error) {
			if len(requests) == 0 || len(requests) > page.MaxBatchSize {
				t.Fatal("неверный размер batch")
			}
			result := make([]page.Page[comment.Comment], len(requests))
			for i, request := range requests {
				if request.PostID.String() != roots[rootCalls*page.MaxBatchSize+i].PostID {
					t.Fatal("порядок постов нарушен")
				}
				value := sampleComment(t, request.PostID, nil)
				position := value.PagePosition()
				result[i] = page.Page[comment.Comment]{Items: []comment.Comment{value}, End: &position}
			}
			rootCalls++
			return result, nil
		},
		replies: func(_ context.Context, requests []commentusecase.ReplyPageRequest) ([]page.Page[comment.Comment], error) {
			if len(requests) == 0 || len(requests) > page.MaxBatchSize {
				t.Fatal("неверный размер batch")
			}
			result := make([]page.Page[comment.Comment], len(requests))
			for i, request := range requests {
				if request.ParentID.String() != replies[replyCalls*page.MaxBatchSize+i].ParentID {
					t.Fatal("порядок родителей нарушен")
				}
				value := sampleComment(t, uuid.New(), &request.ParentID)
				position := value.PagePosition()
				result[i] = page.Page[comment.Comment]{Items: []comment.Comment{value}, End: &position}
			}
			replyCalls++
			return result, nil
		},
	}, nil)
	rootValues, err := u.ListRootsBatch(context.Background(), roots)
	if err != nil || rootCalls != 3 || len(rootValues) != count {
		t.Fatalf("страницы корней: %d, %v", rootCalls, err)
	}
	for i, value := range rootValues {
		if value.Nodes[0].PostID != roots[i].PostID {
			t.Fatal("результаты корней переставлены")
		}
	}
	replyValues, err := u.ListRepliesBatch(context.Background(), replies)
	if err != nil || replyCalls != 3 || len(replyValues) != count {
		t.Fatalf("страницы ответов: %d, %v", replyCalls, err)
	}
	for i, value := range replyValues {
		if *value.Nodes[0].ParentID != replies[i].ParentID {
			t.Fatal("результаты ответов переставлены")
		}
	}
	u = commentusecase.New(commentRepositoryStub{}, nil)
	rootValues, err = u.ListRootsBatch(context.Background(), nil)
	if err != nil || rootValues == nil || len(rootValues) != 0 {
		t.Fatalf("пустые корни: %v", err)
	}
	replyValues, err = u.ListRepliesBatch(context.Background(), nil)
	if err != nil || replyValues == nil || len(replyValues) != 0 {
		t.Fatalf("пустые ответы: %v", err)
	}
}

func TestCancellationBetweenBatches(t *testing.T) {
	ids := make([]string, page.MaxBatchSize+1)
	roots := make([]dto.RootPageInput, len(ids))
	for i := range ids {
		ids[i] = uuid.NewString()
		roots[i].PostID = ids[i]
	}
	t.Run("pages", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		calls := 0
		u := commentusecase.New(commentRepositoryStub{roots: func(_ context.Context, requests []commentusecase.RootPageRequest) ([]page.Page[comment.Comment], error) {
			calls++
			cancel()
			return make([]page.Page[comment.Comment], len(requests)), nil
		}}, nil)
		result, err := u.ListRootsBatch(ctx, roots)
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
	comments := commentusecase.New(commentRepositoryStub{}, nil, clock)
	checks := []func() error{
		func() error { _, err := comments.Create(ctx, "", dto.CreateCommentInput{}); return err },
		func() error { _, err := comments.Get(ctx, ""); return err },
		func() error { _, err := comments.ListRoots(ctx, "", dto.PageInput{}); return err },
		func() error { _, err := comments.ListReplies(ctx, "", dto.PageInput{}); return err },
	}
	for i, check := range checks {
		if err := check(); !errors.Is(err, context.Canceled) {
			t.Fatalf("проверка %d: %v", i, err)
		}
	}
}
