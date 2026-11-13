package comment_test

import (
	"OzonBankTestTask/internal/repositories/postgres/testdb"
	"testing"
	"time"

	"OzonBankTestTask/internal/domain/comment"
	"OzonBankTestTask/internal/domain/post"
	"OzonBankTestTask/internal/repositories/postgres"
	commentrepo "OzonBankTestTask/internal/repositories/postgres/comment"
	postrepo "OzonBankTestTask/internal/repositories/postgres/post"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

func repositories(t *testing.T, configure ...func(*postgres.Config)) (*pgxpool.Pool, *postrepo.Repository, *commentrepo.Repository) {
	t.Helper()
	pool, _ := testdb.Open(t, configure...)
	return pool, postrepo.New(pool), commentrepo.New(pool)
}

func newPost(t *testing.T, r *postrepo.Repository) post.Post {
	t.Helper()
	value, err := post.NewPost("title", "body", "owner", true, time.Now())
	require.NoError(t, err)
	saved, err := r.CreatePost(t.Context(), *value)
	require.NoError(t, err)
	return saved
}

func newComment(t *testing.T, postID uuid.UUID, parentID *uuid.UUID, text string) comment.Comment {
	t.Helper()
	value, err := comment.NewComment(postID, parentID, "reader", text, time.Now())
	require.NoError(t, err)
	return *value
}
