package config

import (
	"fmt"
	"net"
	"os"
	"strconv"
	"time"
)

type HTTP struct {
	Host            string `yaml:"host"`
	Port            int    `yaml:"port"`
	ShutdownTimeout string `yaml:"shutdown_timeout"`
}

// applyHTTP собирает адрес прослушивания и положительный таймаут остановки.
// Непустые HTTP_ADDR и SHUTDOWN_TIMEOUT заменяют значения из файла.
func applyHTTP(c *Config) error {
	if value := os.Getenv("HTTP_ADDR"); value != "" {
		c.HTTPAddress = value
	} else {
		if c.HTTP.Port < 1 || c.HTTP.Port > 65535 {
			return fmt.Errorf("http.port must be between 1 and 65535")
		}
		c.HTTPAddress = net.JoinHostPort(c.HTTP.Host, strconv.Itoa(c.HTTP.Port))
	}
	if _, _, err := net.SplitHostPort(c.HTTPAddress); err != nil {
		return fmt.Errorf("invalid HTTP_ADDR: %w", err)
	}
	timeout := c.HTTP.ShutdownTimeout
	if value := os.Getenv("SHUTDOWN_TIMEOUT"); value != "" {
		timeout = value
	}
	parsed, err := time.ParseDuration(timeout)
	if err != nil || parsed <= 0 {
		return fmt.Errorf("shutdown_timeout / SHUTDOWN_TIMEOUT must be a positive duration")
	}
	c.ShutdownTimeout = parsed
	return nil
}
