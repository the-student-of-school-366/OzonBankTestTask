package main

import (
	"context"
	"log"
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
		log.Fatal(err)
	}
	if err := postgres.Migrate(ctx, postgres.Config{DSN: settings.DatabaseURL}); err != nil {
		log.Fatal(err)
	}
	log.Print("database migrations applied")
}
