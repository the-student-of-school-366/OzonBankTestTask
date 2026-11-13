package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"time"

	appconfig "OzonBankTestTask/internal/config"
	"OzonBankTestTask/internal/repositories/postgres"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	settings, err := appconfig.Load()
	if err != nil {
		slog.Error("migrations failed", "error", err)
		os.Exit(1)
	}
	if err := postgres.Migrate(ctx, postgres.Config{DSN: settings.DatabaseURL}); err != nil {
		slog.Error("migrations failed", "error", err)
		os.Exit(1)
	}
	slog.Info("database migrations applied")
}
