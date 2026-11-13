package config_test

import (
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"OzonBankTestTask/internal/config"

	"github.com/stretchr/testify/require"
)

const example = `storage: postgres
sandbox:
  enabled: true
http:
  host: 127.0.0.1
  port: 8081
  shutdown_timeout: 12s
postgres:
  host: db.example
  port: 5432
  user: alice
  password: "p@ss:/# word"
  database: posts
  sslmode: require
`

func setFile(t *testing.T, contents string) string {
	t.Helper()
	for _, name := range []string{"STORAGE", "SANDBOX_ENABLED", "HTTP_ADDR", "SHUTDOWN_TIMEOUT", "DATABASE_URL", "PGHOST", "PGPORT", "PGUSER", "PGPASSWORD", "PGDATABASE", "PGSSLMODE"} {
		t.Setenv(name, "")
	}
	path := filepath.Join(t.TempDir(), "config.yaml")
	require.NoError(t, os.WriteFile(path, []byte(contents), 0600))
	t.Setenv("CONFIG_FILE", path)
	return path
}

func TestLoadYAMLAndEscapedCredentials(t *testing.T) {
	setFile(t, example)
	c, err := config.Load()
	require.NoError(t, err)
	require.Equal(t, "postgres", c.Storage)
	require.True(t, c.Sandbox.Enabled)
	require.Equal(t, "127.0.0.1:8081", c.HTTPAddress)
	require.Equal(t, 12*time.Second, c.ShutdownTimeout)
	u, err := url.Parse(c.DatabaseURL)
	require.NoError(t, err)
	require.Equal(t, "alice", u.User.Username())
	password, ok := u.User.Password()
	require.True(t, ok)
	require.Equal(t, "p@ss:/# word", password)
	require.Equal(t, "db.example:5432", u.Host)
	require.Equal(t, "/posts", u.Path)
	require.Equal(t, "require", u.Query().Get("sslmode"))
}

func TestEnvironmentOverridesYAML(t *testing.T) {
	setFile(t, example)
	t.Setenv("STORAGE", "postgres")
	t.Setenv("SANDBOX_ENABLED", "false")
	t.Setenv("HTTP_ADDR", "[::1]:9000")
	t.Setenv("SHUTDOWN_TIMEOUT", "3s")
	t.Setenv("PGHOST", "::1")
	t.Setenv("PGPORT", "6543")
	t.Setenv("PGUSER", "bob")
	t.Setenv("PGPASSWORD", "new@pass")
	t.Setenv("PGDATABASE", "other")
	t.Setenv("PGSSLMODE", "disable")
	c, err := config.Load()
	require.NoError(t, err)
	require.False(t, c.Sandbox.Enabled)
	require.Equal(t, "[::1]:9000", c.HTTPAddress)
	require.Equal(t, 3*time.Second, c.ShutdownTimeout)
	u, err := url.Parse(c.DatabaseURL)
	require.NoError(t, err)
	require.Equal(t, "[::1]:6543", u.Host)
	require.Equal(t, "bob", u.User.Username())
	password, _ := u.User.Password()
	require.Equal(t, "new@pass", password)
	require.Equal(t, "/other", u.Path)
	require.Equal(t, "disable", u.Query().Get("sslmode"))
	t.Setenv("DATABASE_URL", "postgres://override/db?sslmode=disable")
	c, err = config.Load()
	require.NoError(t, err)
	require.Equal(t, "postgres://override/db?sslmode=disable", c.DatabaseURL)
}

func TestInvalidConfiguration(t *testing.T) {
	for _, tc := range []struct{ name, contents, message string }{
		{"unknown field", example + "unknown: value\n", "field unknown"},
		{"multiple documents", example + "---\nstorage: memory\n", "one YAML document"},
		{"invalid storage", strings.Replace(example, "storage: postgres", "storage: other", 1), "STORAGE"},
		{"invalid HTTP port", strings.Replace(example, "port: 8081", "port: 0", 1), "http.port"},
		{"invalid timeout", strings.Replace(example, "12s", "-1s", 1), "shutdown_timeout"},
		{"missing postgres user", strings.Replace(example, "user: alice", "user: ''", 1), "postgres configuration"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			setFile(t, tc.contents)
			_, err := config.Load()
			require.ErrorContains(t, err, tc.message)
		})
	}
	t.Run("missing file", func(t *testing.T) {
		path := setFile(t, example)
		t.Setenv("CONFIG_FILE", path+"-missing")
		_, err := config.Load()
		require.ErrorContains(t, err, "open config")
	})
	t.Run("invalid sandbox setting", func(t *testing.T) {
		setFile(t, example)
		t.Setenv("SANDBOX_ENABLED", "invalid")
		_, err := config.Load()
		require.ErrorContains(t, err, "SANDBOX_ENABLED")
	})
	t.Run("memory without postgres settings", func(t *testing.T) {
		setFile(t, "storage: memory\nhttp:\n  host: localhost\n  port: 8080\n  shutdown_timeout: 1s\n")
		c, err := config.Load()
		require.NoError(t, err)
		require.Equal(t, "memory", c.Storage)
		require.Empty(t, c.DatabaseURL)
	})
}
