package post_test

import (
	"context"
	"testing"
	"time"

	"OzonBankTestTask/internal/domain/page"
	"OzonBankTestTask/internal/domain/post"

	"github.com/google/uuid"
)

type postRepositoryStub struct {
	create func(context.Context, post.Post) (post.Post, error)
	byID   func(context.Context, []uuid.UUID) (map[uuid.UUID]post.Post, error)
	list   func(context.Context, page.Request) (page.Page[post.Post], error)
	set    func(context.Context, uuid.UUID, string, bool) (post.Post, error)
}

func (s postRepositoryStub) CreatePost(ctx context.Context, value post.Post) (post.Post, error) {
	return s.create(ctx, value)
}
func (s postRepositoryStub) PostsByID(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]post.Post, error) {
	return s.byID(ctx, ids)
}
func (s postRepositoryStub) ListPosts(ctx context.Context, request page.Request) (page.Page[post.Post], error) {
	return s.list(ctx, request)
}
func (s postRepositoryStub) SetCommentsEnabled(ctx context.Context, id uuid.UUID, actor string, enabled bool) (post.Post, error) {
	return s.set(ctx, id, actor, enabled)
}

func fixedClock() time.Time {
	return time.Date(2026, 11, 12, 10, 20, 30, 123456000, time.UTC)
}

func samplePost(t *testing.T) post.Post {
	t.Helper()
	value, err := post.NewPost("title", "content", "author", true, fixedClock())
	if err != nil {
		t.Fatal(err)
	}
	return *value
}
