package comment_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"OzonBankTestTask/internal/domain"
	"OzonBankTestTask/internal/domain/comment"
	"OzonBankTestTask/internal/domain/page"
	"OzonBankTestTask/internal/repositories/postgres"
	commentrepo "OzonBankTestTask/internal/repositories/postgres/comment"
	"OzonBankTestTask/internal/repositories/postgres/queries"
	commentusecase "OzonBankTestTask/internal/usecase/comment"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestBatchPagesPreserveOrderCursorsAndEmptyOwners(t *testing.T) {
	pool, posts, comments := repositories(t, func(c *postgres.Config) { c.MaxConns = 1 })
	a, empty := newPost(t, posts), newPost(t, posts)
	created := time.Date(2026, 1, 2, 3, 4, 5, 123456000, time.UTC)
	var values []comment.Comment
	for i := 1; i <= 6; i++ {
		var parent *uuid.UUID
		if i > 3 {
			parent = &values[0].ID
		}
		value := newComment(t, a.ID, parent, fmt.Sprintf("comment %d", i))
		value.ID = uuid.MustParse(fmt.Sprintf("00000000-0000-0000-0000-%012d", i))
		value.CreatedAt = created
		saved, err := comments.CreateComment(t.Context(), value)
		require.NoError(t, err)
		values = append(values, saved)
	}
	// One connection lets PostgreSQL's execution counters verify one SQL execution per batch.
	executions := func(name string) int64 {
		var count int64
		require.NoError(t, pool.QueryRow(t.Context(),
			"SELECT generic_plans + custom_plans FROM pg_prepared_statements WHERE name=$1", name).Scan(&count))
		return count
	}
	rootCursor := values[2].PagePosition()
	before := executions(queries.RootsBatch)
	roots, err := comments.ListRootsBatch(t.Context(), []commentusecase.RootPageRequest{
		{PostID: a.ID, Page: page.Request{First: 1}},
		{PostID: empty.ID, Page: page.Request{First: 2}},
		{PostID: a.ID, Page: page.Request{First: 2, After: &rootCursor}},
		{PostID: a.ID, Page: page.Request{First: 2}},
	})
	require.NoError(t, err)
	require.Equal(t, before+1, executions(queries.RootsBatch))
	require.Len(t, roots, 4)
	assertPage(t, roots[0], true, values[2].ID)
	assertPage(t, roots[1], false)
	assertPage(t, roots[2], false, values[1].ID, values[0].ID)
	assertPage(t, roots[3], true, values[2].ID, values[1].ID)
	for _, result := range roots {
		for _, item := range result.Items {
			require.Nil(t, item.ParentID)
		}
	}

	replyCursor := values[5].PagePosition()
	before = executions(queries.RepliesBatch)
	replies, err := comments.ListRepliesBatch(t.Context(), []commentusecase.ReplyPageRequest{
		{ParentID: values[0].ID, Page: page.Request{First: 1}},
		{ParentID: values[1].ID, Page: page.Request{First: 2}},
		{ParentID: values[0].ID, Page: page.Request{First: 2, After: &replyCursor}},
		{ParentID: values[0].ID, Page: page.Request{First: 1}},
	})
	require.NoError(t, err)
	require.Equal(t, before+1, executions(queries.RepliesBatch))
	require.Len(t, replies, 4)
	assertPage(t, replies[0], true, values[5].ID)
	assertPage(t, replies[1], false)
	assertPage(t, replies[2], false, values[4].ID, values[3].ID)
	assertPage(t, replies[3], true, values[5].ID)
	*replies[0].Items[0].ParentID = uuid.Nil
	require.Equal(t, values[0].ID, *replies[3].Items[0].ParentID)

	last, err := comments.ListRepliesBatch(t.Context(), []commentusecase.ReplyPageRequest{
		{ParentID: values[0].ID, Page: page.Request{First: 2, After: replies[2].End}},
	})
	require.NoError(t, err)
	assertPage(t, last[0], false)
	_, err = comments.ListRootsBatch(t.Context(), []commentusecase.RootPageRequest{
		{PostID: a.ID, Page: page.Request{First: 1}},
		{PostID: uuid.New(), Page: page.Request{First: 1}},
	})
	require.ErrorIs(t, err, domain.ErrPostNotFound)
	_, err = comments.ListRepliesBatch(t.Context(), []commentusecase.ReplyPageRequest{
		{ParentID: uuid.New(), Page: page.Request{First: 1}},
	})
	require.ErrorIs(t, err, domain.ErrCommentNotFound)
}

func TestEmptyAndInvalidBatchesDoNotRequireConnection(t *testing.T) {
	r := commentrepo.New(nil)
	roots, err := r.ListRootsBatch(t.Context(), nil)
	require.NoError(t, err)
	require.NotNil(t, roots)
	require.Empty(t, roots)
	replies, err := r.ListRepliesBatch(t.Context(), nil)
	require.NoError(t, err)
	require.NotNil(t, replies)
	_, err = r.ListRootsBatch(t.Context(), []commentusecase.RootPageRequest{{PostID: uuid.New(), Page: page.Request{First: 101}}})
	require.ErrorIs(t, err, domain.ErrInvalidPageSize)
	_, err = r.ListRepliesBatch(t.Context(), []commentusecase.ReplyPageRequest{{Page: page.Request{First: 1}}})
	require.ErrorIs(t, err, domain.ErrInvalidID)
	values, err := r.CommentsByID(t.Context(), nil)
	require.NoError(t, err)
	require.Empty(t, values)
	_, err = r.CreateComment(t.Context(), comment.Comment{})
	require.ErrorIs(t, err, domain.ErrInvalidID)
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	_, err = r.ListRootsBatch(ctx, nil)
	require.ErrorIs(t, err, context.Canceled)
}

func assertPage(t *testing.T, actual page.Page[comment.Comment], hasNext bool, ids ...uuid.UUID) {
	t.Helper()
	require.NotNil(t, actual.Items)
	require.Len(t, actual.Items, len(ids))
	for i, id := range ids {
		require.Equal(t, id, actual.Items[i].ID)
	}
	require.Equal(t, hasNext, actual.HasNextPage)
	if len(ids) == 0 {
		require.Nil(t, actual.End)
	} else {
		require.NotNil(t, actual.End)
		require.Equal(t, ids[len(ids)-1], actual.End.ID)
	}
}
