// Package store handles database access and schema migrations.
package store

import (
	"database/sql"
	"fmt"

	logrus "github.com/sirupsen/logrus"
	_ "modernc.org/sqlite"
)

// OpenDB opens a SQLite database at the given path with WAL mode, foreign keys,
// and a 5-second busy timeout.
func OpenDB(path string) (*sql.DB, error) {
	dsn := fmt.Sprintf("file:%s?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=foreign_keys(ON)", path)

	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	// Verify connection and pragmas.
	var journalMode string
	if err := db.QueryRow("PRAGMA journal_mode").Scan(&journalMode); err != nil {
		db.Close()
		return nil, fmt.Errorf("check journal_mode: %w", err)
	}
	if journalMode != "wal" {
		db.Close()
		return nil, fmt.Errorf("expected WAL journal mode, got %s", journalMode)
	}

	var fkEnabled int
	if err := db.QueryRow("PRAGMA foreign_keys").Scan(&fkEnabled); err != nil {
		db.Close()
		return nil, fmt.Errorf("check foreign_keys: %w", err)
	}
	if fkEnabled != 1 {
		db.Close()
		return nil, fmt.Errorf("foreign keys not enabled")
	}

	logrus.WithField("path", path).Info("Database opened successfully")
	return db, nil
}
