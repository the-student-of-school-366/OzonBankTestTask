package graphql

import (
	"log/slog"
	"time"

	"OzonBankTestTask/internal/graphql/limits"
)

// Config получает проверенные настройки из api.Config.
type Config struct {
	Limits          limits.Config
	RequestTimeout  time.Duration
	MaxMessageBytes int64
	Origins         []string
	Logger          *slog.Logger
}
