// Package config загружает настройки запуска приложения и мигратора.
package config

import (
	"errors"
	"fmt"
	"io"
	"os"
	"time"

	"go.yaml.in/yaml/v3"
)

const DefaultPath = "internal/config/config.yaml"

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

func configPath() string {
	if path := os.Getenv("CONFIG_FILE"); path != "" {
		return path
	}
	return DefaultPath
}

// decodeFile читает один YAML-документ. Неизвестные поля запрещены.
func decodeFile(path string) (Config, error) {
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
	return c, nil
}
