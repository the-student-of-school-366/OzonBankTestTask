package postgres

import (
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Config struct {
	DSN               string
	MaxConns          int32
	MinConns          int32
	ConnectTimeout    time.Duration
	MaxConnLifetime   time.Duration
	MaxConnIdleTime   time.Duration
	HealthCheckPeriod time.Duration
}

func (c Config) poolConfig() (*pgxpool.Config, error) {
	if strings.TrimSpace(c.DSN) == "" {
		return nil, fmt.Errorf("postgres: DSN is required")
	}
	if c.MaxConns == 0 {
		c.MaxConns = 16
	}
	if c.ConnectTimeout == 0 {
		c.ConnectTimeout = 5 * time.Second
	}
	if c.MaxConnLifetime == 0 {
		c.MaxConnLifetime = 30 * time.Minute
	}
	if c.MaxConnIdleTime == 0 {
		c.MaxConnIdleTime = 5 * time.Minute
	}
	if c.HealthCheckPeriod == 0 {
		c.HealthCheckPeriod = 30 * time.Second
	}
	if c.MaxConns < 1 || c.MinConns < 0 || c.MinConns > c.MaxConns || c.ConnectTimeout < 0 || c.MaxConnLifetime < 0 || c.MaxConnIdleTime < 0 || c.HealthCheckPeriod < 0 {
		return nil, fmt.Errorf("postgres: invalid pool configuration")
	}
	config, err := pgxpool.ParseConfig(c.DSN)
	if err != nil {
		return nil, fmt.Errorf("postgres: parse configuration: %w", err)
	}
	config.MaxConns, config.MinConns = c.MaxConns, c.MinConns
	config.ConnConfig.ConnectTimeout = c.ConnectTimeout
	config.MaxConnLifetime, config.MaxConnIdleTime = c.MaxConnLifetime, c.MaxConnIdleTime
	config.MaxConnLifetimeJitter = c.MaxConnLifetime / 10
	config.HealthCheckPeriod = c.HealthCheckPeriod
	config.ConnConfig.RuntimeParams["timezone"] = "UTC"
	if config.ConnConfig.RuntimeParams["application_name"] == "" {
		config.ConnConfig.RuntimeParams["application_name"] = "ozon-bank"
	}
	return config, nil
}
