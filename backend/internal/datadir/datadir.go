// Package datadir handles data directory bootstrapping and validation.
package datadir

import (
	"fmt"
	"os"
	"path/filepath"

	logrus "github.com/sirupsen/logrus"
)

// RequiredSubdirs are the directories created on startup if missing.
var RequiredSubdirs = []string{"projects", "exports", "tmp", "nats"}

// DefaultPath returns the default data directory path (~/ image-dataset-tool).
func DefaultPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		// Fallback if home dir detection fails.
		return filepath.Join(".", "image-dataset-tool")
	}
	return filepath.Join(home, "image-dataset-tool")
}

// Resolve returns the data directory path from $DATA_DIR or the default.
func Resolve() string {
	if v := os.Getenv("DATA_DIR"); v != "" {
		return v
	}
	return DefaultPath()
}

// Bootstrap creates the data directory and required subdirectories if they
// do not exist. Returns the resolved data directory path.
func Bootstrap() (string, error) {
	dir := Resolve()

	logrus.WithField("path", dir).Info("Bootstrapping data directory")

	// Create root data dir.
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("create data dir %s: %w", dir, err)
	}

	// Create required subdirectories.
	for _, sub := range RequiredSubdirs {
		subPath := filepath.Join(dir, sub)
		if err := os.MkdirAll(subPath, 0755); err != nil {
			return "", fmt.Errorf("create subdir %s: %w", subPath, err)
		}
	}

	logrus.Info("Data directory bootstrapped successfully")
	return dir, nil
}

// SecretKeyPath returns the path to the encryption key file.
func SecretKeyPath(dataDir string) string {
	return filepath.Join(dataDir, "secret.key")
}

// DBPath returns the path to the SQLite database file.
func DBPath(dataDir string) string {
	return filepath.Join(dataDir, "db.sqlite")
}
