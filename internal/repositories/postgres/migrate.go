package postgres

import (
	"context"
	"embed"
	"fmt"
	"io/fs"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	"github.com/pressly/goose/v3/lock"
)

//go:embed migrations/*.sql
var migrationFiles embed.FS

// Migrate применяет встроенные версии в транзакциях под advisory lock.
// Отдельное соединение мигратора не подготавливает запросы к ещё не созданным таблицам.
func Migrate(ctx context.Context, settings Config) error {
	config, err := settings.poolConfig()
	if err != nil {
		return err
	}
	config.ConnConfig.DefaultQueryExecMode = pgx.QueryExecModeExec
	db := stdlib.OpenDB(*config.ConnConfig)
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	defer db.Close()
	files, err := fs.Sub(migrationFiles, "migrations")
	if err != nil {
		return err
	}
	locker, err := lock.NewPostgresSessionLocker()
	if err != nil {
		return err
	}
	provider, err := goose.NewProvider(goose.DialectPostgres, db, files, goose.WithSessionLocker(locker))
	if err != nil {
		return fmt.Errorf("postgres: migration provider: %w", err)
	}
	if _, err = provider.Up(ctx); err != nil {
		return fmt.Errorf("postgres: migrate: %w", err)
	}
	return nil
}
