package postgres

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestPoolDefaultsAndOverrides(t *testing.T) {
	config, err := (Config{DSN: "postgres://localhost/example?sslmode=disable"}).poolConfig()
	require.NoError(t, err)
	require.EqualValues(t, 16, config.MaxConns)
	require.Zero(t, config.MinConns)
	require.Equal(t, 5*time.Second, config.ConnConfig.ConnectTimeout)
	require.Equal(t, "UTC", config.ConnConfig.RuntimeParams["timezone"])
	config, err = (Config{DSN: "postgres://localhost/example", MaxConns: 5, MinConns: 2, ConnectTimeout: time.Second, MaxConnLifetime: time.Minute, MaxConnIdleTime: time.Second, HealthCheckPeriod: time.Second}).poolConfig()
	require.NoError(t, err)
	require.EqualValues(t, 5, config.MaxConns)
	require.EqualValues(t, 2, config.MinConns)
	require.Equal(t, time.Second, config.ConnConfig.ConnectTimeout)
	require.Equal(t, time.Minute, config.MaxConnLifetime)
	require.Equal(t, 6*time.Second, config.MaxConnLifetimeJitter)
}

func TestInvalidPoolConfiguration(t *testing.T) {
	for _, config := range []Config{
		{}, {DSN: "postgres://%"}, {DSN: "postgres://localhost/db", MaxConns: -1},
		{DSN: "postgres://localhost/db", MinConns: -1}, {DSN: "postgres://localhost/db", MinConns: 3, MaxConns: 2},
		{DSN: "postgres://localhost/db", ConnectTimeout: -1}, {DSN: "postgres://localhost/db", MaxConnLifetime: -1},
		{DSN: "postgres://localhost/db", MaxConnIdleTime: -1}, {DSN: "postgres://localhost/db", HealthCheckPeriod: -1},
	} {
		_, err := config.poolConfig()
		require.Error(t, err)
	}
}
