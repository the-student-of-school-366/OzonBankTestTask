// Package config загружает настройки запуска приложения и мигратора.
package config

import (
	"errors"
	"fmt"
	"io"
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"go.yaml.in/yaml/v3"
)

const DefaultPath = "internal/config/config.yaml"

type HTTP struct {
	Host            string `yaml:"host"`
	Port            int    `yaml:"port"`
	ShutdownTimeout string `yaml:"shutdown_timeout"`
}

type Postgres struct {
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	User     string `yaml:"user"`
	Password string `yaml:"password"`
	Database string `yaml:"database"`
	SSLMode  string `yaml:"sslmode"`
}

type Sandbox struct {
	Enabled bool `yaml:"enabled"`
}

type Config struct {
	Storage         string        `yaml:"storage"`
	Sandbox         Sandbox       `yaml:"sandbox"`
	HTTP            HTTP          `yaml:"http"`
	Postgres        Postgres      `yaml:"postgres"`
	HTTPAddress     string        `yaml:"-"`
	ShutdownTimeout time.Duration `yaml:"-"`
	DatabaseURL     string        `yaml:"-"`
}

// Load читает CONFIG_FILE или DefaultPath относительно рабочего каталога.
// Непустые переменные окружения имеют приоритет над полями файла.
func Load() (Config, error) {
	path := os.Getenv("CONFIG_FILE")
	if path == "" {
		path = DefaultPath
	}
	file, err := os.Open(path)
	if err != nil {
		return Config{}, fmt.Errorf("open config %q: %w", path, err)
	}
	defer file.Close()
	decoder := yaml.NewDecoder(file)
	decoder.KnownFields(true)
	var c Config
	if err = decoder.Decode(&c); err != nil {
		return Config{}, fmt.Errorf("decode config %q: %w", path, err)
	}
	var extra any
	if err = decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		if err != nil {
			return Config{}, fmt.Errorf("decode config %q: %w", path, err)
		}
		return Config{}, fmt.Errorf("config %q must contain one YAML document", path)
	}
	if value := os.Getenv("STORAGE"); value != "" {
		c.Storage = value
	}
	if c.Storage != "memory" && c.Storage != "postgres" {
		return Config{}, fmt.Errorf("STORAGE must be memory or postgres")
	}
	if value := os.Getenv("SANDBOX_ENABLED"); value != "" {
		c.Sandbox.Enabled, err = strconv.ParseBool(value)
		if err != nil {
			return Config{}, fmt.Errorf("SANDBOX_ENABLED must be true or false")
		}
	}
	if value := os.Getenv("HTTP_ADDR"); value != "" {
		c.HTTPAddress = value
	} else {
		if c.HTTP.Port < 1 || c.HTTP.Port > 65535 {
			return Config{}, fmt.Errorf("http.port must be between 1 and 65535")
		}
		c.HTTPAddress = net.JoinHostPort(c.HTTP.Host, strconv.Itoa(c.HTTP.Port))
	}
	if _, _, err = net.SplitHostPort(c.HTTPAddress); err != nil {
		return Config{}, fmt.Errorf("invalid HTTP_ADDR: %w", err)
	}
	timeout := c.HTTP.ShutdownTimeout
	if value := os.Getenv("SHUTDOWN_TIMEOUT"); value != "" {
		timeout = value
	}
	c.ShutdownTimeout, err = time.ParseDuration(timeout)
	if err != nil || c.ShutdownTimeout <= 0 {
		return Config{}, fmt.Errorf("shutdown_timeout / SHUTDOWN_TIMEOUT must be a positive duration")
	}
	c.DatabaseURL = os.Getenv("DATABASE_URL")
	if c.DatabaseURL == "" {
		p := &c.Postgres
		if v := os.Getenv("PGHOST"); v != "" {
			p.Host = v
		}
		if v := os.Getenv("PGPORT"); v != "" {
			p.Port, err = strconv.Atoi(v)
			if err != nil {
				return Config{}, fmt.Errorf("PGPORT must be a valid port")
			}
		}
		if v := os.Getenv("PGUSER"); v != "" {
			p.User = v
		}
		if v := os.Getenv("PGPASSWORD"); v != "" {
			p.Password = v
		}
		if v := os.Getenv("PGDATABASE"); v != "" {
			p.Database = v
		}
		if v := os.Getenv("PGSSLMODE"); v != "" {
			p.SSLMode = v
		}
		if strings.TrimSpace(p.Host) == "" || p.Port < 1 || p.Port > 65535 || strings.TrimSpace(p.User) == "" || strings.TrimSpace(p.Database) == "" || strings.TrimSpace(p.SSLMode) == "" {
			if c.Storage == "postgres" {
				return Config{}, fmt.Errorf("invalid postgres configuration: host, port, user, database and sslmode are required")
			}
			return c, nil
		}
		u := url.URL{Scheme: "postgres", User: url.UserPassword(p.User, p.Password), Host: net.JoinHostPort(p.Host, strconv.Itoa(p.Port)), Path: "/" + p.Database}
		query := u.Query()
		query.Set("sslmode", p.SSLMode)
		u.RawQuery = query.Encode()
		c.DatabaseURL = u.String()
	}
	return c, nil
}
