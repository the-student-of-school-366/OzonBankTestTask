package config

import (
	"fmt"
	"os"
	"strconv"
)

// Load читает CONFIG_FILE или DefaultPath относительно рабочего каталога.
// Непустые переменные окружения имеют приоритет над полями файла.
func Load() (Config, error) {
	c, err := decodeFile(configPath())
	if err != nil {
		return Config{}, err
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
	if err = applyHTTP(&c); err != nil {
		return Config{}, err
	}
	if err = applyDatabase(&c); err != nil {
		return Config{}, err
	}
	return c, nil
}
