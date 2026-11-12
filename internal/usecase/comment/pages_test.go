package comment_test

import (
	"OzonBankTestTask/internal/usecase"
	"context"
	"errors"
	"testing"
	"time"

	"OzonBankTestTask/internal/domain"
	"OzonBankTestTask/internal/domain/comment"
	"OzonBankTestTask/internal/domain/page"
	"OzonBankTestTask/internal/dto"
	"OzonBankTestTask/internal/pagination"
	commentusecase "OzonBankTestTask/internal/usecase/comment"

	"github.com/google/uuid"
)

func TestRootPagesKeepDistinctCursorsForSamePost(t *testing.T) {
	postID, emptyPostID := uuid.New(), uuid.New()
	newer := sampleComment(t, postID, nil)
	older := sampleComment(t, postID, nil)
	older.CreatedAt = newer.CreatedAt.Add(-time.Second)
	newPosition, oldPosition := newer.PagePosition(), older.PagePosition()
	scope := pagination.Scope{Kind: pagination.Roots, ID: postID}
	cursor, err := pagination.Encode(newPosition, scope)
	if err != nil {
		t.Fatal(err)
	}
	one, two, calls := 1, 2, 0
	repo := commentRepositoryStub{roots: func(_ context.Context, requests []commentusecase.RootPageRequest) ([]page.Page[comment.Comment], error) {
		calls++
		if len(requests) != 3 || requests[0].PostID != postID || requests[1].PostID != postID || requests[2].PostID != emptyPostID {
			t.Fatalf("запросы: %+v", requests)
		}
		if requests[0].Page.First != 1 || requests[0].Page.After != nil || requests[1].Page.First != 2 || requests[1].Page.After == nil || requests[1].Page.After.Compare(newPosition) != 0 {
			t.Fatal("разные страницы одного поста смешаны")
		}
		return []page.Page[comment.Comment]{
			{Items: []comment.Comment{newer}, End: &newPosition, HasNextPage: true},
			{Items: []comment.Comment{older}, End: &oldPosition},
			{},
		}, nil
	}}
	result, err := commentusecase.New(repo, nil).ListRootsBatch(context.Background(), []dto.RootPageInput{
		{PostID: postID.String(), PageInput: dto.PageInput{First: &one}},
		{PostID: postID.String(), PageInput: dto.PageInput{First: &two, After: &cursor}},
		{PostID: emptyPostID.String()},
	})
	if err != nil {
		t.Fatal(err)
	}
	if calls != 1 || len(result) != 3 || result[0].Nodes[0].ID != newer.ID.String() || result[1].Nodes[0].ID != older.ID.String() || result[2].Nodes == nil || len(result[2].Nodes) != 0 || result[2].PageInfo.EndCursor != nil {
		t.Fatalf("страницы: %+v", result)
	}
	if !result[0].PageInfo.HasNextPage || result[1].PageInfo.HasNextPage {
		t.Fatal("потерян признак следующей страницы")
	}
	decoded, err := pagination.Decode(*result[1].PageInfo.EndCursor, scope)
	if err != nil || decoded.ID != older.ID {
		t.Fatalf("конец второй страницы: %v", err)
	}
}

func TestReplyPagesAndSinglePageWrappers(t *testing.T) {
	parentID := uuid.New()
	reply := sampleComment(t, uuid.New(), &parentID)
	position := reply.PagePosition()
	scope := pagination.Scope{Kind: pagination.Replies, ID: parentID}
	cursor, err := pagination.Encode(position, scope)
	if err != nil {
		t.Fatal(err)
	}
	calls := 0
	repo := commentRepositoryStub{
		replies: func(_ context.Context, requests []commentusecase.ReplyPageRequest) ([]page.Page[comment.Comment], error) {
			calls++
			if len(requests) != 1 || requests[0].ParentID != parentID || requests[0].Page.First != page.DefaultPageSize || requests[0].Page.After == nil || requests[0].Page.After.ID != reply.ID {
				t.Fatalf("запрос ответов: %+v", requests)
			}
			return []page.Page[comment.Comment]{{Items: []comment.Comment{reply}, End: &position}}, nil
		},
		roots: func(_ context.Context, requests []commentusecase.RootPageRequest) ([]page.Page[comment.Comment], error) {
			if len(requests) != 1 || requests[0].PostID != reply.PostID {
				t.Fatalf("запрос корней: %+v", requests)
			}
			return []page.Page[comment.Comment]{{}}, nil
		},
	}
	u := commentusecase.New(repo, nil)
	result, err := u.ListReplies(context.Background(), parentID.String(), dto.PageInput{After: &cursor})
	if err != nil || calls != 1 || len(result.Nodes) != 1 || *result.Nodes[0].ParentID != parentID.String() {
		t.Fatalf("ответы: %+v, %v", result, err)
	}
	if _, err := pagination.Decode(*result.PageInfo.EndCursor, scope); err != nil {
		t.Fatal(err)
	}
	roots, err := u.ListRoots(context.Background(), reply.PostID.String(), dto.PageInput{})
	if err != nil || roots.Nodes == nil || len(roots.Nodes) != 0 {
		t.Fatalf("корни: %+v, %v", roots, err)
	}
}

func TestCommentPageValidationBeforeAnyBatch(t *testing.T) {
	u := commentusecase.New(commentRepositoryStub{}, nil)
	validID := uuid.NewString()
	tooMany := page.MaxPageSize + 1
	if _, err := u.ListRoots(context.Background(), "bad", dto.PageInput{}); !errors.Is(err, domain.ErrInvalidID) {
		t.Fatal(err)
	}
	if _, err := u.ListReplies(context.Background(), "bad", dto.PageInput{}); !errors.Is(err, domain.ErrInvalidID) {
		t.Fatal(err)
	}
	if _, err := u.ListRoots(context.Background(), validID, dto.PageInput{First: &tooMany}); !errors.Is(err, domain.ErrInvalidPageSize) {
		t.Fatal(err)
	}
	if _, err := u.ListReplies(context.Background(), validID, dto.PageInput{First: &tooMany}); !errors.Is(err, domain.ErrInvalidPageSize) {
		t.Fatal(err)
	}
	cursor, err := pagination.Encode(page.Position{CreatedAt: fixedClock(), ID: uuid.New()}, pagination.Scope{Kind: pagination.Roots, ID: uuid.New()})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := u.ListRoots(context.Background(), validID, dto.PageInput{After: &cursor}); !errors.Is(err, domain.ErrInvalidCursor) {
		t.Fatal(err)
	}
	if _, err := u.ListReplies(context.Background(), validID, dto.PageInput{After: &cursor}); !errors.Is(err, domain.ErrInvalidCursor) {
		t.Fatal(err)
	}
	roots := make([]dto.RootPageInput, usecase.MaxBatchSize+1)
	for i := range roots {
		roots[i].PostID = validID
	}
	roots[len(roots)-1].PostID = "bad"
	if _, err := u.ListRootsBatch(context.Background(), roots); !errors.Is(err, domain.ErrInvalidID) {
		t.Fatal(err)
	}
	replies := make([]dto.ReplyPageInput, len(roots))
	for i := range replies {
		replies[i].ParentID = validID
	}
	replies[len(replies)-1].PageInput.After = &cursor
	if _, err := u.ListRepliesBatch(context.Background(), replies); !errors.Is(err, domain.ErrInvalidCursor) {
		t.Fatal(err)
	}
}

func TestCommentPageRepositoryFailures(t *testing.T) {
	broken := errors.New("database unavailable")
	for _, want := range []error{domain.ErrPostNotFound, domain.ErrCommentNotFound, broken} {
		u := commentusecase.New(commentRepositoryStub{
			roots: func(context.Context, []commentusecase.RootPageRequest) ([]page.Page[comment.Comment], error) {
				return nil, want
			},
			replies: func(context.Context, []commentusecase.ReplyPageRequest) ([]page.Page[comment.Comment], error) {
				return nil, want
			},
		}, nil)
		_, rootErr := u.ListRoots(context.Background(), uuid.NewString(), dto.PageInput{})
		_, replyErr := u.ListReplies(context.Background(), uuid.NewString(), dto.PageInput{})
		for _, err := range []error{rootErr, replyErr} {
			if !errors.Is(err, want) || errors.Is(err, domain.ErrStorage) != (want == broken) {
				t.Fatalf("неверная ошибка: %v", err)
			}
		}
	}
	for _, count := range []int{0, 2} {
		u := commentusecase.New(commentRepositoryStub{roots: func(context.Context, []commentusecase.RootPageRequest) ([]page.Page[comment.Comment], error) {
			return make([]page.Page[comment.Comment], count), nil
		}}, nil)
		if _, err := u.ListRoots(context.Background(), uuid.NewString(), dto.PageInput{}); !errors.Is(err, domain.ErrStorage) {
			t.Fatalf("неверная длина результата не проверена: %v", err)
		}
	}
	u := commentusecase.New(commentRepositoryStub{replies: func(context.Context, []commentusecase.ReplyPageRequest) ([]page.Page[comment.Comment], error) {
		return []page.Page[comment.Comment]{{End: &page.Position{}}}, nil
	}}, nil)
	if _, err := u.ListReplies(context.Background(), uuid.NewString(), dto.PageInput{}); !errors.Is(err, domain.ErrStorage) || errors.Is(err, domain.ErrInvalidCursor) {
		t.Fatalf("ошибка данных выдана за ошибку клиента: %v", err)
	}
}
