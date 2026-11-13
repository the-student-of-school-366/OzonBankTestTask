package config

import (
	"fmt"
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"
)

type Postgres struct {
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	User     string `yaml:"user"`
	Password string `yaml:"password"`
	Database string `yaml:"database"`
	SSLMode  string `yaml:"sslmode"`
}

// applyDatabase оставляет непустой DATABASE_URL без изменений.
// Иначе собирает URL из полей Postgres после непустых PG*.
// Для memory неполный набор полей не ошибка: DatabaseURL остаётся пустым.
func applyDatabase(c *Config) error {
	if value := os.Getenv("DATABASE_URL"); value != "" {
		c.DatabaseURL = value
		return nil
	}
	if err := applyPostgresEnv(&c.Postgres); err != nil {
		return err
	}
	if !postgresConfigured(c.Postgres) {
		if c.Storage == "postgres" {
			return fmt.Errorf("invalid postgres configuration: host, port, user, database and sslmode are required")
		}
		return nil
	}
	c.DatabaseURL = postgresURL(c.Postgres)
	return nil
}

func applyPostgresEnv(p *Postgres) error {
	if v := os.Getenv("PGHOST"); v != "" {
		p.Host = v
	}
	if v := os.Getenv("PGPORT"); v != "" {
		port, err := strconv.Atoi(v)
		if err != nil {
			return fmt.Errorf("PGPORT must be a valid port")
		}
		p.Port = port
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
	return nil
}

func postgresConfigured(p Postgres) bool {
	return strings.TrimSpace(p.Host) != "" && p.Port >= 1 && p.Port <= 65535 &&
		strings.TrimSpace(p.User) != "" && strings.TrimSpace(p.Database) != "" && strings.TrimSpace(p.SSLMode) != ""
}

func postgresURL(p Postgres) string {
	u := url.URL{
		Scheme: "postgres", User: url.UserPassword(p.User, p.Password),
		Host: net.JoinHostPort(p.Host, strconv.Itoa(p.Port)), Path: "/" + p.Database,
	}
	query := u.Query()
	query.Set("sslmode", p.SSLMode)
	u.RawQuery = query.Encode()
	return u.String()
}
