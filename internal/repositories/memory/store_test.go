package memory_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"OzonBankTestTask/internal/domain"
	"OzonBankTestTask/internal/domain/comment"
	"OzonBankTestTask/internal/domain/page"
	"OzonBankTestTask/internal/domain/post"
	"OzonBankTestTask/internal/repositories/memory"
	commentusecase "OzonBankTestTask/internal/usecase/comment"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func makePost(t *testing.T, s *memory.Store) post.Post {
	t.Helper()
	p, err := post.NewPost("title", "content", "owner", true, time.Now())
	require.NoError(t, err)
	value, err := s.CreatePost(t.Context(), *p)
	require.NoError(t, err)
	return value
}

func makeComment(t *testing.T, postID uuid.UUID, parent *uuid.UUID) comment.Comment {
	t.Helper()
	c, err := comment.NewComment(postID, parent, "reader", "text", time.Now())
	require.NoError(t, err)
	return *c
}

func TestPostPagesAndAuthor(t *testing.T) {
	s := memory.New()
	when := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	ids := []uuid.UUID{uuid.MustParse("00000000-0000-0000-0000-000000000001"), uuid.MustParse("00000000-0000-0000-0000-000000000002"), uuid.MustParse("00000000-0000-0000-0000-000000000003")}
	for _, id := range []uuid.UUID{ids[1], ids[0], ids[2]} {
		_, err := s.CreatePost(t.Context(), post.Post{ID: id, Title: "title", Content: "content", Author: "owner", CommentsEnabled: true, CreatedAt: when})
		require.NoError(t, err)
	}
	first, err := s.ListPosts(t.Context(), page.Request{First: 2})
	require.NoError(t, err)
	require.Equal(t, []uuid.UUID{ids[2], ids[1]}, []uuid.UUID{first.Items[0].ID, first.Items[1].ID})
	require.True(t, first.HasNextPage)
	newer, err := post.NewPost("new", "content", "owner", true, when.Add(time.Second))
	require.NoError(t, err)
	_, err = s.CreatePost(t.Context(), *newer)
	require.NoError(t, err)
	second, err := s.ListPosts(t.Context(), page.Request{First: 2, After: first.End})
	require.NoError(t, err)
	require.Len(t, second.Items, 1)
	require.Equal(t, ids[0], second.Items[0].ID)
	require.False(t, second.HasNextPage)
	empty, err := s.ListPosts(t.Context(), page.Request{First: 2, After: second.End})
	require.NoError(t, err)
	require.NotNil(t, empty.Items)
	require.Empty(t, empty.Items)
	values, err := s.PostsByID(t.Context(), []uuid.UUID{ids[0], ids[0], uuid.New()})
	require.NoError(t, err)
	require.Len(t, values, 1)
	_, err = s.CreatePost(t.Context(), values[ids[0]])
	require.ErrorIs(t, err, domain.ErrStorage)
	_, err = s.SetCommentsEnabled(t.Context(), ids[0], "other", false)
	require.ErrorIs(t, err, domain.ErrNotAuthor)
	changed, err := s.SetCommentsEnabled(t.Context(), ids[0], "owner", false)
	require.NoError(t, err)
	require.False(t, changed.CommentsEnabled)
}

func TestCommentPlacementPagesAndCopies(t *testing.T) {
	s := memory.New()
	a, b := makePost(t, s), makePost(t, s)
	when := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	ids := []uuid.UUID{uuid.MustParse("00000000-0000-0000-0000-000000000001"), uuid.MustParse("00000000-0000-0000-0000-000000000002"), uuid.MustParse("00000000-0000-0000-0000-000000000003")}
	for _, id := range ids {
		value := makeComment(t, a.ID, nil)
		value.ID, value.CreatedAt = id, when
		_, err := s.CreateComment(t.Context(), value)
		require.NoError(t, err)
	}
	duplicate, err := s.CommentsByID(t.Context(), []uuid.UUID{ids[0]})
	require.NoError(t, err)
	_, err = s.CreateComment(t.Context(), duplicate[ids[0]])
	require.ErrorIs(t, err, domain.ErrStorage)
	parent := ids[0]
	input := makeComment(t, a.ID, &parent)
	reply, err := s.CreateComment(t.Context(), input)
	require.NoError(t, err)
	*input.ParentID = uuid.Nil
	*reply.ParentID = uuid.Nil
	loaded, err := s.CommentsByID(t.Context(), []uuid.UUID{reply.ID})
	require.NoError(t, err)
	require.Equal(t, ids[0], *loaded[reply.ID].ParentID)
	*loaded[reply.ID].ParentID = uuid.Nil
	batch, err := s.ListRootsBatch(t.Context(), []commentusecase.RootPageRequest{
		{PostID: a.ID, Page: page.Request{First: 1}}, {PostID: b.ID, Page: page.Request{First: 2}},
		{PostID: a.ID, Page: page.Request{First: 2}},
	})
	require.NoError(t, err)
	require.Len(t, batch, 3)
	require.Equal(t, ids[2], batch[0].Items[0].ID)
	require.True(t, batch[0].HasNextPage)
	require.NotNil(t, batch[1].Items)
	require.Empty(t, batch[1].Items)
	require.Equal(t, ids[1], batch[2].Items[1].ID)
	next, err := s.ListRootsBatch(t.Context(), []commentusecase.RootPageRequest{{PostID: a.ID, Page: page.Request{First: 2, After: batch[0].End}}})
	require.NoError(t, err)
	require.Equal(t, ids[1], next[0].Items[0].ID)
	require.Equal(t, ids[0], next[0].Items[1].ID)
	replies, err := s.ListRepliesBatch(t.Context(), []commentusecase.ReplyPageRequest{{ParentID: ids[0], Page: page.Request{First: 1}}, {ParentID: ids[0], Page: page.Request{First: 1}}})
	require.NoError(t, err)
	require.Equal(t, ids[0], *replies[0].Items[0].ParentID)
	*replies[0].Items[0].ParentID = uuid.Nil
	require.Equal(t, ids[0], *replies[1].Items[0].ParentID)
	_, err = s.CreateComment(t.Context(), makeComment(t, b.ID, &ids[0]))
	require.ErrorIs(t, err, domain.ErrParentPostMismatch)
	missing := uuid.New()
	_, err = s.CreateComment(t.Context(), makeComment(t, a.ID, &missing))
	require.ErrorIs(t, err, domain.ErrCommentNotFound)
	_, err = s.CreateComment(t.Context(), makeComment(t, missing, nil))
	require.ErrorIs(t, err, domain.ErrPostNotFound)
	_, err = s.SetCommentsEnabled(t.Context(), a.ID, "owner", false)
	require.NoError(t, err)
	_, err = s.CreateComment(t.Context(), makeComment(t, a.ID, &missing))
	require.ErrorIs(t, err, domain.ErrCommentsDisabled)
	_, err = s.SetCommentsEnabled(t.Context(), a.ID, "owner", true)
	require.NoError(t, err)
	_, err = s.CreateComment(t.Context(), makeComment(t, a.ID, &ids[0]))
	require.NoError(t, err)
	_, err = s.ListRepliesBatch(t.Context(), []commentusecase.ReplyPageRequest{{ParentID: missing, Page: page.Request{First: 1}}})
	require.ErrorIs(t, err, domain.ErrCommentNotFound)
}

func TestValidationAndCancellation(t *testing.T) {
	s := memory.New()
	_, err := s.PostsByID(t.Context(), make([]uuid.UUID, 101))
	require.ErrorIs(t, err, domain.ErrInvalidPageSize)
	_, err = s.CommentsByID(t.Context(), []uuid.UUID{uuid.Nil})
	require.ErrorIs(t, err, domain.ErrInvalidID)
	_, err = s.ListPosts(t.Context(), page.Request{First: 0})
	require.ErrorIs(t, err, domain.ErrInvalidPageSize)
	_, err = s.ListRootsBatch(t.Context(), []commentusecase.RootPageRequest{{PostID: uuid.New(), Page: page.Request{First: 1, After: &page.Position{}}}})
	require.ErrorIs(t, err, domain.ErrInvalidCursor)
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	_, err = s.ListRootsBatch(ctx, nil)
	require.ErrorIs(t, err, context.Canceled)
	empty, err := s.ListRootsBatch(t.Context(), nil)
	require.NoError(t, err)
	require.NotNil(t, empty)
}

func TestConcurrentCommentCreationAndDisable(t *testing.T) {
	s := memory.New()
	p := makePost(t, s)
	const count = 100
	values := make([]comment.Comment, count)
	for i := range values {
		values[i] = makeComment(t, p.ID, nil)
	}
	start := make(chan struct{})
	errs := make(chan error, count)
	var wg sync.WaitGroup
	for _, value := range values {
		wg.Add(1)
		go func() { defer wg.Done(); <-start; _, err := s.CreateComment(context.Background(), value); errs <- err }()
	}
	wg.Add(1)
	disabled := make(chan error, 1)
	go func() {
		defer wg.Done()
		<-start
		_, err := s.SetCommentsEnabled(context.Background(), p.ID, "owner", false)
		disabled <- err
	}()
	close(start)
	wg.Wait()
	close(errs)
	require.NoError(t, <-disabled)
	var successfulWrites int
	for err := range errs {
		if err != nil {
			require.ErrorIs(t, err, domain.ErrCommentsDisabled)
		} else {
			successfulWrites++
		}
	}
	_, err := s.CreateComment(t.Context(), makeComment(t, p.ID, nil))
	require.ErrorIs(t, err, domain.ErrCommentsDisabled)
	pageResult, err := s.ListRootsBatch(t.Context(), []commentusecase.RootPageRequest{{PostID: p.ID, Page: page.Request{First: 100}}})
	require.NoError(t, err)
	var successes int
	for _, value := range values {
		found, err := s.CommentsByID(t.Context(), []uuid.UUID{value.ID})
		require.NoError(t, err)
		if len(found) != 0 {
			successes++
		}
	}
	require.Len(t, pageResult[0].Items, successes)
	require.Equal(t, successfulWrites, successes)
}
