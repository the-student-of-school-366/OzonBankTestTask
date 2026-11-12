package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"time"

	"OzonBankTestTask/internal/repositories/postgres"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	if err := postgres.Migrate(ctx, postgres.Config{DSN: os.Getenv("DATABASE_URL")}); err != nil {
		log.Fatal(err)
	}
	log.Print("database migrations applied")
}
