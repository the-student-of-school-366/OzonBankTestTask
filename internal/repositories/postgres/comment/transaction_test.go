package comment_test

import (
	"context"
	"testing"
	"time"

	"OzonBankTestTask/internal/domain"
	"OzonBankTestTask/internal/repositories/postgres"
	"OzonBankTestTask/internal/repositories/postgres/queries"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

func TestCommentShareLockAllowsInsertsAndBlocksDisable(t *testing.T) {
	pool, posts, comments := repositories(t)
	post := newPost(t, posts)
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	conn, err := pool.Acquire(ctx)
	require.NoError(t, err)
	defer conn.Release()
	tx, err := conn.Begin(ctx)
	require.NoError(t, err)
	defer tx.Rollback(context.Background())
	_, err = postgres.ScanPost(tx.QueryRow(ctx, queries.PostForShare, postgres.UUID(post.ID)))
	require.NoError(t, err)
	// A second FOR SHARE must proceed while the first transaction still holds its lock.
	_, err = comments.CreateComment(ctx, newComment(t, post.ID, nil, "parallel insert"))
	require.NoError(t, err)
	done := make(chan error, 1)
	go func() { _, err := posts.SetCommentsEnabled(ctx, post.ID, "owner", false); done <- err }()
	waitForBlockedTransaction(t, pool, conn.Conn().PgConn().PID())
	require.NoError(t, tx.Commit(ctx))
	require.NoError(t, <-done)
	_, err = comments.CreateComment(ctx, newComment(t, post.ID, nil, "too late"))
	require.ErrorIs(t, err, domain.ErrCommentsDisabled)
}

func TestCommentWaitsForDisableAndSeesCommittedFlag(t *testing.T) {
	pool, posts, comments := repositories(t)
	post := newPost(t, posts)
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	conn, err := pool.Acquire(ctx)
	require.NoError(t, err)
	defer conn.Release()
	tx, err := conn.Begin(ctx)
	require.NoError(t, err)
	defer tx.Rollback(context.Background())
	_, err = postgres.ScanPost(tx.QueryRow(ctx, queries.PostForUpdate, postgres.UUID(post.ID)))
	require.NoError(t, err)
	_, err = tx.Exec(ctx, queries.PostSetComments, postgres.UUID(post.ID), false)
	require.NoError(t, err)
	value := newComment(t, post.ID, nil, "blocked insert")
	done := make(chan error, 1)
	go func() { _, err := comments.CreateComment(ctx, value); done <- err }()
	waitForBlockedTransaction(t, pool, conn.Conn().PgConn().PID())
	require.NoError(t, tx.Commit(ctx))
	require.ErrorIs(t, <-done, domain.ErrCommentsDisabled)
	var count int
	require.NoError(t, conn.QueryRow(ctx, "SELECT count(*) FROM comments").Scan(&count))
	require.Zero(t, count)
}

func TestCanceledLockWaitReleasesTransaction(t *testing.T) {
	pool, posts, comments := repositories(t)
	post := newPost(t, posts)
	conn, err := pool.Acquire(t.Context())
	require.NoError(t, err)
	defer conn.Release()
	tx, err := conn.Begin(t.Context())
	require.NoError(t, err)
	defer tx.Rollback(context.Background())
	_, err = postgres.ScanPost(tx.QueryRow(t.Context(), queries.PostForUpdate, postgres.UUID(post.ID)))
	require.NoError(t, err)
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	value := newComment(t, post.ID, nil, "canceled insert")
	done := make(chan error, 1)
	go func() { _, err := comments.CreateComment(ctx, value); done <- err }()
	waitForBlockedTransaction(t, pool, conn.Conn().PgConn().PID())
	cancel()
	require.ErrorIs(t, <-done, context.Canceled)
	// pgxpool destroys canceled connections asynchronously before updating its counters.
	require.Eventually(t, func() bool { return pool.Stat().AcquiredConns() == 1 }, time.Second, 10*time.Millisecond)
	require.NoError(t, tx.Rollback(t.Context()))
	_, err = comments.CreateComment(t.Context(), value)
	require.NoError(t, err)
}

func waitForBlockedTransaction(t *testing.T, pool *pgxpool.Pool, holder uint32) {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for {
		var blocked bool
		err := pool.QueryRow(ctx, "SELECT EXISTS (SELECT 1 FROM pg_stat_activity WHERE $1::int = ANY(pg_blocking_pids(pid)))", int32(holder)).Scan(&blocked)
		require.NoError(t, err)
		if blocked {
			return
		}
		select {
		case <-ctx.Done():
			t.Fatal("transaction did not wait for the expected PostgreSQL lock")
		case <-ticker.C:
		}
	}
}
