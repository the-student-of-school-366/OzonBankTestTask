package main

import (
	"time"

	appconfig "OzonBankTestTask/internal/config"
)

type config struct {
	address         string
	storage         string
	databaseURL     string
	shutdownTimeout time.Duration
}

func loadConfig() (config, error) {
	settings, err := appconfig.Load()
	if err != nil {
		return config{}, err
	}
	return config{address: settings.HTTPAddress, storage: settings.Storage, databaseURL: settings.DatabaseURL, shutdownTimeout: settings.ShutdownTimeout}, nil
}
