// Package slogutil initialises the default slog logger with a JSON handler
// and configures the global log level from the application config or
// environment variable.
package slogutil

import (
	"log/slog"
	"os"
	"strings"
)

// Configure sets up slog with a JSON handler writing to stdout.
// The log level is resolved in this order:
//  1. "log_level" key in the provided config map (from config.yaml).
//  2. LOG_LEVEL environment variable.
//  3. Default: "info".
//
// This function should be called once at startup before any logging occurs.
func Configure(cfg map[string]any) {
	level := resolveLevel(cfg)

	handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: level,
	})
	slog.SetDefault(slog.New(handler))

	slog.Info("Logging configured", "log_level", level.String())
}

// resolveLevel determines the slog.Level from config and environment.
func resolveLevel(cfg map[string]any) slog.Level {
	// 1. Config map takes precedence.
	if cfg != nil {
		if v, ok := cfg["log_level"]; ok {
			if s, ok := v.(string); ok && s != "" {
				if l, err := parseLevel(s); err == nil {
					return l
				}
				slog.Warn("Invalid log_level in config.yaml — defaulting to info", "value", v)
			}
		}
	}

	// 2. Environment variable fallback.
	if env := os.Getenv("LOG_LEVEL"); env != "" {
		if l, err := parseLevel(env); err == nil {
			return l
		}
		slog.Warn("Invalid LOG_LEVEL env var — defaulting to info", "value", env)
	}

	return slog.LevelInfo
}

// parseLevel maps a string to a slog.Level. Supports: debug, info, warn,
// warning, error. "trace" maps to debug (slog has no trace level).
func parseLevel(s string) (slog.Level, error) {
	switch strings.ToLower(s) {
	case "debug", "trace":
		return slog.LevelDebug, nil
	case "info":
		return slog.LevelInfo, nil
	case "warn", "warning":
		return slog.LevelWarn, nil
	case "error":
		return slog.LevelError, nil
	}
	return slog.LevelInfo, &unknownLevelError{s}
}

type unknownLevelError struct{ val string }

func (e *unknownLevelError) Error() string { return "unknown log level: " + e.val }
