package post_test

import (
	"OzonBankTestTask/internal/repositories/postgres/testdb"
	"context"
	"testing"
	"time"

	"OzonBankTestTask/internal/domain"
	"OzonBankTestTask/internal/domain/page"
	"OzonBankTestTask/internal/domain/post"
	postrepo "OzonBankTestTask/internal/repositories/postgres/post"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/require"
)

func TestCreateReadAndKeysetPagination(t *testing.T) {
	pool, _ := testdb.Open(t)
	r := postrepo.New(pool)
	ctx := t.Context()
	when := time.Date(2026, 11, 12, 12, 0, 0, 123456000, time.FixedZone("MSK", 3*3600))
	ids := []uuid.UUID{
		uuid.MustParse("00000000-0000-0000-0000-000000000001"), uuid.MustParse("00000000-0000-0000-0000-000000000002"),
		uuid.MustParse("00000000-0000-0000-0000-000000000003"), uuid.MustParse("00000000-0000-0000-0000-000000000004"),
	}
	for _, id := range ids {
		value := post.Post{ID: id, Title: "title ' ; DROP TABLE posts; --", Content: "body", Author: "author", CommentsEnabled: true, CreatedAt: when}
		saved, err := r.CreatePost(ctx, value)
		require.NoError(t, err)
		require.Equal(t, value.Title, saved.Title)
		require.Equal(t, when.UTC(), saved.CreatedAt)
	}
	values, err := r.PostsByID(ctx, []uuid.UUID{ids[1], uuid.New(), ids[1]})
	require.NoError(t, err)
	require.Len(t, values, 1)
	first, err := r.ListPosts(ctx, page.Request{First: 2})
	require.NoError(t, err)
	require.Equal(t, ids[3], first.Items[0].ID)
	require.Equal(t, ids[2], first.Items[1].ID)
	require.Equal(t, ids[2], first.End.ID)
	require.True(t, first.HasNextPage)
	newer, err := post.NewPost("new", "body", "author", true, when.Add(time.Second))
	require.NoError(t, err)
	_, err = r.CreatePost(ctx, *newer)
	require.NoError(t, err)
	second, err := r.ListPosts(ctx, page.Request{First: 2, After: first.End})
	require.NoError(t, err)
	require.Equal(t, ids[1], second.Items[0].ID)
	require.Equal(t, ids[0], second.Items[1].ID)
	require.False(t, second.HasNextPage)
	empty, err := r.ListPosts(ctx, page.Request{First: 2, After: second.End})
	require.NoError(t, err)
	require.NotNil(t, empty.Items)
	require.Empty(t, empty.Items)
	require.Nil(t, empty.End)
	_, err = r.CreatePost(ctx, *newer)
	require.ErrorIs(t, err, domain.ErrStorage)
	var pgErr *pgconn.PgError
	require.ErrorAs(t, err, &pgErr)
	require.Equal(t, "23505", pgErr.Code)
}

func TestOnlyAuthorCanChangeComments(t *testing.T) {
	pool, _ := testdb.Open(t)
	r := postrepo.New(pool)
	value, err := post.NewPost("title", "body", "owner", true, time.Now())
	require.NoError(t, err)
	_, err = r.CreatePost(t.Context(), *value)
	require.NoError(t, err)
	_, err = r.SetCommentsEnabled(t.Context(), value.ID, "intruder", false)
	require.ErrorIs(t, err, domain.ErrNotAuthor)
	current, err := r.PostsByID(t.Context(), []uuid.UUID{value.ID})
	require.NoError(t, err)
	require.True(t, current[value.ID].CommentsEnabled)
	changed, err := r.SetCommentsEnabled(t.Context(), value.ID, "owner", false)
	require.NoError(t, err)
	require.False(t, changed.CommentsEnabled)
	_, err = r.SetCommentsEnabled(t.Context(), uuid.New(), "owner", true)
	require.ErrorIs(t, err, domain.ErrPostNotFound)
	require.Zero(t, pool.Stat().AcquiredConns())
}

func TestInvalidInputDoesNotRequireConnection(t *testing.T) {
	r := postrepo.New(nil)
	_, err := r.CreatePost(t.Context(), post.Post{})
	require.ErrorIs(t, err, domain.ErrInvalidID)
	_, err = r.ListPosts(t.Context(), page.Request{First: 0})
	require.ErrorIs(t, err, domain.ErrInvalidPageSize)
	_, err = r.SetCommentsEnabled(t.Context(), uuid.Nil, "owner", false)
	require.ErrorIs(t, err, domain.ErrInvalidID)
	values, err := r.PostsByID(t.Context(), nil)
	require.NoError(t, err)
	require.Empty(t, values)
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	_, err = r.PostsByID(ctx, []uuid.UUID{uuid.New()})
	require.ErrorIs(t, err, context.Canceled)
}
