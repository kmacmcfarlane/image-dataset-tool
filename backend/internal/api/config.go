package api

import (
	"fmt"
	"log/slog"
	"os"

	"gopkg.in/yaml.v3"
)

// LoadConfig reads config.yaml (or the path in CONFIG_PATH env var) and returns
// the parsed contents as a nested map. Returns an empty map if the file does not
// exist (the file is optional).
func LoadConfig() (map[string]any, error) {
	path := os.Getenv("CONFIG_PATH")
	if path == "" {
		path = "config.yaml"
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			slog.Info("config.yaml not found — using empty config", "path", path)
			return map[string]any{}, nil
		}
		return nil, fmt.Errorf("read config file: %w", err)
	}

	var cfg map[string]any
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config file: %w", err)
	}

	if cfg == nil {
		cfg = map[string]any{}
	}
	return cfg, nil
}
