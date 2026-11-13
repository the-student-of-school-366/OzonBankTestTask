package main

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestLoadConfig(t *testing.T) {
	for _, name := range []string{"HTTP_ADDR", "STORAGE", "SANDBOX_ENABLED", "DATABASE_URL", "SHUTDOWN_TIMEOUT", "PGHOST", "PGPORT", "PGUSER", "PGPASSWORD", "PGDATABASE", "PGSSLMODE"} {
		t.Setenv(name, "")
	}
	path, err := filepath.Abs("../../internal/config/config.yaml")
	require.NoError(t, err)
	t.Setenv("CONFIG_FILE", path)
	c, err := loadConfig()
	require.NoError(t, err)
	require.Equal(t, "postgres", c.storage)
	require.True(t, c.sandboxEnabled)
	require.Equal(t, "0.0.0.0:8081", c.address)
	require.Equal(t, 10*time.Second, c.shutdownTimeout)
	require.Contains(t, c.databaseURL, "localhost:55432")
	t.Setenv("DATABASE_URL", "")
	t.Setenv("STORAGE", "memory")
	c, err = loadConfig()
	require.NoError(t, err)
	require.Equal(t, "memory", c.storage)
	t.Setenv("STORAGE", "postgres")
	t.Setenv("DATABASE_URL", "postgres://localhost/test")
	t.Setenv("HTTP_ADDR", "127.0.0.1:8081")
	t.Setenv("SHUTDOWN_TIMEOUT", "3s")
	c, err = loadConfig()
	require.NoError(t, err)
	require.Equal(t, "postgres", c.storage)
	require.Equal(t, "127.0.0.1:8081", c.address)
	require.Equal(t, 3*time.Second, c.shutdownTimeout)
	t.Setenv("SHUTDOWN_TIMEOUT", "-1s")
	_, err = loadConfig()
	require.ErrorContains(t, err, "SHUTDOWN_TIMEOUT")
	t.Setenv("STORAGE", "unknown")
	_, err = loadConfig()
	require.ErrorContains(t, err, "STORAGE")
}
