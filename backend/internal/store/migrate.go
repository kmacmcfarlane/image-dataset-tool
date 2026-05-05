package store

import (
	"database/sql"
	"fmt"

	logrus "github.com/sirupsen/logrus"
)

// migration represents a single schema migration.
type migration struct {
	version int
	name    string
	sql     string
}

// migrations is the ordered list of schema migrations.
var migrations = []migration{
	{
		version: 1,
		name:    "initial_schema",
		sql: `
CREATE TABLE IF NOT EXISTS projects (
	id TEXT PRIMARY KEY,
	slug TEXT NOT NULL UNIQUE,
	name TEXT NOT NULL,
	description TEXT NOT NULL DEFAULT '',
	created_at TEXT NOT NULL,
	updated_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS subjects (
	id TEXT PRIMARY KEY,
	project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
	slug TEXT NOT NULL,
	name TEXT NOT NULL,
	created_at TEXT NOT NULL,
	updated_at TEXT NOT NULL,
	UNIQUE(project_id, slug)
);

CREATE TABLE IF NOT EXISTS accounts (
	id TEXT PRIMARY KEY,
	platform TEXT NOT NULL,
	handle TEXT NOT NULL,
	cookies_blob BLOB,
	last_login_at TEXT,
	created_at TEXT NOT NULL,
	updated_at TEXT NOT NULL,
	UNIQUE(platform, handle)
);

CREATE TABLE IF NOT EXISTS subject_accounts (
	subject_id TEXT NOT NULL REFERENCES subjects(id) ON DELETE CASCADE,
	account_id TEXT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
	created_at TEXT NOT NULL,
	PRIMARY KEY (subject_id, account_id)
);

CREATE TABLE IF NOT EXISTS samples (
	id TEXT PRIMARY KEY,
	subject_id TEXT NOT NULL REFERENCES subjects(id) ON DELETE CASCADE,
	source_platform TEXT NOT NULL,
	source_post_id TEXT NOT NULL,
	slide_index INTEGER NOT NULL DEFAULT 0,
	file_path TEXT NOT NULL,
	sha256 TEXT NOT NULL,
	phash TEXT NOT NULL,
	width INTEGER NOT NULL,
	height INTEGER NOT NULL,
	taken_at TEXT,
	fetched_at TEXT NOT NULL,
	status TEXT NOT NULL DEFAULT 'pending',
	is_duplicate INTEGER NOT NULL DEFAULT 0,
	duplicate_of TEXT,
	created_at TEXT NOT NULL,
	updated_at TEXT NOT NULL,
	UNIQUE(subject_id, source_post_id, slide_index)
);

CREATE INDEX IF NOT EXISTS idx_samples_phash ON samples(phash);
CREATE INDEX IF NOT EXISTS idx_samples_sha256 ON samples(sha256);
CREATE INDEX IF NOT EXISTS idx_samples_status ON samples(status);
CREATE INDEX IF NOT EXISTS idx_samples_subject_id ON samples(subject_id);
CREATE INDEX IF NOT EXISTS idx_samples_is_duplicate ON samples(is_duplicate);

CREATE TABLE IF NOT EXISTS edits (
	sample_id TEXT PRIMARY KEY REFERENCES samples(id) ON DELETE CASCADE,
	rotation_deg REAL NOT NULL DEFAULT 0.0,
	crop_box_json TEXT,
	auto_actions_json TEXT,
	created_at TEXT NOT NULL,
	updated_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS caption_studies (
	id TEXT PRIMARY KEY,
	project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
	name TEXT NOT NULL,
	slug TEXT NOT NULL,
	provider TEXT NOT NULL,
	model TEXT NOT NULL,
	prompt_template TEXT NOT NULL,
	params_json TEXT NOT NULL DEFAULT '{}',
	created_at TEXT NOT NULL,
	updated_at TEXT NOT NULL,
	UNIQUE(project_id, slug)
);

CREATE TABLE IF NOT EXISTS captions (
	id TEXT PRIMARY KEY,
	sample_id TEXT NOT NULL REFERENCES samples(id) ON DELETE CASCADE,
	study_id TEXT NOT NULL REFERENCES caption_studies(id) ON DELETE CASCADE,
	text TEXT NOT NULL,
	created_at TEXT NOT NULL,
	UNIQUE(sample_id, study_id)
);

CREATE TABLE IF NOT EXISTS job_runs (
	id TEXT PRIMARY KEY,
	type TEXT NOT NULL,
	subject_id TEXT REFERENCES subjects(id) ON DELETE CASCADE,
	study_id TEXT REFERENCES caption_studies(id) ON DELETE CASCADE,
	status TEXT NOT NULL,
	total_items INTEGER NOT NULL DEFAULT 0,
	completed_items INTEGER NOT NULL DEFAULT 0,
	failed_items INTEGER NOT NULL DEFAULT 0,
	started_at TEXT NOT NULL,
	finished_at TEXT,
	created_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS job_messages (
	id TEXT PRIMARY KEY,
	job_run_id TEXT NOT NULL REFERENCES job_runs(id) ON DELETE CASCADE,
	level TEXT NOT NULL,
	sample_ref TEXT,
	account_handle TEXT,
	message TEXT NOT NULL,
	created_at TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_job_messages_run_level ON job_messages(job_run_id, level);

CREATE TABLE IF NOT EXISTS secrets (
	key TEXT PRIMARY KEY,
	value_encrypted BLOB NOT NULL,
	created_at TEXT NOT NULL,
	updated_at TEXT NOT NULL
);
`,
	},
}

// Migrate applies all pending migrations to the database.
func Migrate(db *sql.DB) error {
	// Create migrations tracking table.
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version INTEGER PRIMARY KEY,
			name TEXT NOT NULL,
			applied_at TEXT NOT NULL DEFAULT (datetime('now'))
		)
	`)
	if err != nil {
		return fmt.Errorf("create schema_migrations table: %w", err)
	}

	for _, m := range migrations {
		var count int
		err := db.QueryRow("SELECT COUNT(*) FROM schema_migrations WHERE version = ?", m.version).Scan(&count)
		if err != nil {
			return fmt.Errorf("check migration %d: %w", m.version, err)
		}

		if count > 0 {
			continue
		}

		logrus.WithFields(logrus.Fields{
			"version": m.version,
			"name":    m.name,
		}).Info("Applying migration")

		tx, err := db.Begin()
		if err != nil {
			return fmt.Errorf("begin migration %d: %w", m.version, err)
		}

		if _, err := tx.Exec(m.sql); err != nil {
			tx.Rollback()
			return fmt.Errorf("execute migration %d (%s): %w", m.version, m.name, err)
		}

		if _, err := tx.Exec("INSERT INTO schema_migrations (version, name) VALUES (?, ?)", m.version, m.name); err != nil {
			tx.Rollback()
			return fmt.Errorf("record migration %d: %w", m.version, err)
		}

		if err := tx.Commit(); err != nil {
			return fmt.Errorf("commit migration %d: %w", m.version, err)
		}
	}

	logrus.Info("All migrations applied")
	return nil
}
