// Package testdb создаёт отдельную схему для каждого интеграционного теста.
package testdb

import (
	"context"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"OzonBankTestTask/internal/repositories/postgres"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

func Open(t *testing.T, configure ...func(*postgres.Config)) (*pgxpool.Pool, postgres.Config) {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL is not set; PostgreSQL integration test")
	}
	ctx, cancel := context.WithTimeout(t.Context(), time.Minute)
	defer cancel()
	admin, err := pgx.Connect(ctx, dsn)
	require.NoError(t, err)
	schema := "ozon_test_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	name := pgx.Identifier{schema}.Sanitize()
	_, err = admin.Exec(ctx, "CREATE SCHEMA "+name)
	if err != nil {
		_ = admin.Close(context.Background())
		t.Fatal(err)
	}
	var pool *pgxpool.Pool
	t.Cleanup(func() {
		if pool != nil {
			pool.Close()
		}
		cleanup, stop := context.WithTimeout(context.Background(), 10*time.Second)
		defer stop()
		_, err := admin.Exec(cleanup, "DROP SCHEMA "+name+" CASCADE")
		if err != nil {
			t.Errorf("drop test schema: %v", err)
		}
		_ = admin.Close(cleanup)
	})
	if strings.HasPrefix(dsn, "postgres://") || strings.HasPrefix(dsn, "postgresql://") {
		parsed, err := url.Parse(dsn)
		require.NoError(t, err)
		query := parsed.Query()
		query.Set("search_path", schema)
		parsed.RawQuery = query.Encode()
		dsn = parsed.String()
	} else {
		dsn += " search_path=" + schema
	}
	config := postgres.Config{DSN: dsn, MaxConns: 4}
	for _, set := range configure {
		set(&config)
	}
	require.NoError(t, postgres.Migrate(ctx, config))
	pool, err = postgres.Open(ctx, config)
	require.NoError(t, err)
	return pool, config
}
