package api

import (
	"fmt"
	"log/slog"
	"time"
)

// Config ограничивает ресурсы одной HTTP-операции. Нулевые значения заменяются значениями по умолчанию.
type Config struct {
	MaxBodyBytes   int64
	RequestTimeout time.Duration
	MaxDepth       int
	MaxComplexity  int
	Logger         *slog.Logger
}

func (c Config) withDefaults() (Config, error) {
	if c.MaxBodyBytes == 0 {
		c.MaxBodyBytes = 1 << 20
	}
	if c.RequestTimeout == 0 {
		c.RequestTimeout = 5 * time.Second
	}
	if c.MaxDepth == 0 {
		c.MaxDepth = 20
	}
	if c.MaxComplexity == 0 {
		c.MaxComplexity = 10_000
	}
	if c.Logger == nil {
		c.Logger = slog.Default()
	}
	if c.MaxBodyBytes < 1 || c.RequestTimeout < 0 || c.MaxDepth < 1 || c.MaxDepth > 100 || c.MaxComplexity < 1 || c.MaxComplexity > 1_000_000 {
		return Config{}, fmt.Errorf("api: invalid request limits")
	}
	return c, nil
}
