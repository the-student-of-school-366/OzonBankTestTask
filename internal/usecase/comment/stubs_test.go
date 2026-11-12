package comment_test

import (
	"context"
	"testing"
	"time"

	"OzonBankTestTask/internal/domain/comment"
	"OzonBankTestTask/internal/domain/page"
	commentusecase "OzonBankTestTask/internal/usecase/comment"

	"github.com/google/uuid"
)

type commentRepositoryStub struct {
	create  func(context.Context, comment.Comment) (comment.Comment, error)
	byID    func(context.Context, []uuid.UUID) (map[uuid.UUID]comment.Comment, error)
	roots   func(context.Context, []commentusecase.RootPageRequest) ([]page.Page[comment.Comment], error)
	replies func(context.Context, []commentusecase.ReplyPageRequest) ([]page.Page[comment.Comment], error)
}

func (s commentRepositoryStub) CreateComment(ctx context.Context, value comment.Comment) (comment.Comment, error) {
	return s.create(ctx, value)
}
func (s commentRepositoryStub) CommentsByID(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]comment.Comment, error) {
	return s.byID(ctx, ids)
}
func (s commentRepositoryStub) ListRootsBatch(ctx context.Context, requests []commentusecase.RootPageRequest) ([]page.Page[comment.Comment], error) {
	return s.roots(ctx, requests)
}
func (s commentRepositoryStub) ListRepliesBatch(ctx context.Context, requests []commentusecase.ReplyPageRequest) ([]page.Page[comment.Comment], error) {
	return s.replies(ctx, requests)
}

type publisherFunc func(comment.Comment)

func (f publisherFunc) Publish(value comment.Comment) { f(value) }

func fixedClock() time.Time {
	return time.Date(2026, 11, 12, 10, 20, 30, 123456000, time.UTC)
}

func sampleComment(t *testing.T, postID uuid.UUID, parentID *uuid.UUID) comment.Comment {
	t.Helper()
	value, err := comment.NewComment(postID, parentID, "reader", "text", fixedClock())
	if err != nil {
		t.Fatal(err)
	}
	return *value
}
