package postgres_test

import (
	"OzonBankTestTask/internal/repositories/postgres/testdb"
	"context"
	"sync"
	"testing"
	"time"

	"OzonBankTestTask/internal/repositories/postgres"
	"OzonBankTestTask/internal/repositories/postgres/queries"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

func TestPreparedStatementsOnEveryConnectionAndPoolLimit(t *testing.T) {
	pool, _ := testdb.Open(t, func(c *postgres.Config) { c.MaxConns = 2 })
	ctx := t.Context()
	a, err := pool.Acquire(ctx)
	require.NoError(t, err)
	defer a.Release()
	b, err := pool.Acquire(ctx)
	require.NoError(t, err)
	defer b.Release()
	check := func(conn *pgxpool.Conn) {
		for _, statement := range queries.All() {
			var sql string
			var fromSQL bool
			require.NoError(t, conn.QueryRow(ctx, "SELECT statement, from_sql FROM pg_prepared_statements WHERE name=$1", statement.Name).Scan(&sql, &fromSQL))
			require.Equal(t, statement.SQL, sql)
			require.False(t, fromSQL)
		}
		rows, err := conn.Query(ctx, queries.PostsFirstPage, 2)
		require.NoError(t, err)
		rows.Close()
		require.NoError(t, rows.Err())
	}
	check(a)
	check(b)
	deadline, cancel := context.WithTimeout(ctx, 30*time.Millisecond)
	defer cancel()
	_, err = pool.Acquire(deadline)
	require.ErrorIs(t, err, context.DeadlineExceeded)
	oldPID := a.Conn().PgConn().PID()
	require.NoError(t, a.Conn().Close(ctx))
	a.Release()
	replacement, err := pool.Acquire(ctx)
	require.NoError(t, err)
	defer replacement.Release()
	require.NotEqual(t, oldPID, replacement.Conn().PgConn().PID())
	check(replacement)
}

func TestMigrationsAreRepeatableAndSerialized(t *testing.T) {
	pool, config := testdb.Open(t)
	var wg sync.WaitGroup
	errors := make(chan error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); errors <- postgres.Migrate(t.Context(), config) }()
	}
	wg.Wait()
	close(errors)
	for err := range errors {
		require.NoError(t, err)
	}
	var versions int
	require.NoError(t, pool.QueryRow(t.Context(), "SELECT count(*) FROM goose_db_version WHERE version_id=1 AND is_applied").Scan(&versions))
	require.Equal(t, 1, versions)
}
