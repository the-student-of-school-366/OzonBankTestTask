package postgres

import (
	"context"
	"fmt"

	"OzonBankTestTask/internal/repositories/postgres/queries"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Open создаёт общий пул. Миграции должны быть применены до открытия пула.
// Владелец пула вызывает Close после завершения запросов приложения.
func Open(ctx context.Context, settings Config) (*pgxpool.Pool, error) {
	config, err := settings.poolConfig()
	if err != nil {
		return nil, err
	}
	config.AfterConnect = func(ctx context.Context, conn *pgx.Conn) error {
		for _, statement := range queries.All() {
			if _, err := conn.Prepare(ctx, statement.Name, statement.SQL); err != nil {
				return fmt.Errorf("prepare %s: %w", statement.Name, err)
			}
		}
		return nil
	}
	startup, cancel := context.WithTimeout(ctx, config.ConnConfig.ConnectTimeout)
	defer cancel()
	pool, err := pgxpool.NewWithConfig(startup, config)
	if err != nil {
		return nil, fmt.Errorf("postgres: create pool: %w", err)
	}
	if err = pool.Ping(startup); err != nil {
		pool.Close()
		return nil, fmt.Errorf("postgres: initialize pool: %w", err)
	}
	return pool, nil
}
