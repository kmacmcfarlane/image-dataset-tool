# Database Schema

## Overview

SQLite via `modernc.org/sqlite` (pure Go, no CGO). Configuration: WAL mode, 5s busy
timeout, foreign keys ON, `PRAGMA wal_autocheckpoint = 1000` (default). The database
is an **index** over filesystem manifests — if the DB is lost, F-005 reconciler
rebuilds it from disk.

All tables use UUID primary keys. All rows have `created_at` (ISO 8601). Mutable rows
also have `updated_at`.

All foreign keys use `ON DELETE CASCADE` — deleting a project cascades through
subjects, samples, edits, captions, caption_studies, subject_accounts links,
job_runs, and job_errors.

## Secret encryption

Sensitive values (`cookies_blob` in `subject_accounts`, `value_encrypted` in `secrets`)
are encrypted with AES-256-GCM at the application level before storage. The encryption
key is stored in `$DATA_DIR/secret.key` (mode 0600), mounted into
the container via `.claude-sandbox.yaml`.

## Tables

### projects

| Column      | Type    | Notes              |
| ----------- | ------- | ------------------ |
| id          | TEXT PK | UUID               |
| slug        | TEXT    | UNIQUE, URL-safe   |
| name        | TEXT    | Display name       |
| description | TEXT    | Optional           |
| created_at  | TEXT    | ISO 8601           |
| updated_at  | TEXT    | ISO 8601           |

### subjects

| Column     | Type    | Notes                       |
| ---------- | ------- | --------------------------- |
| id         | TEXT PK | UUID                        |
| project_id | TEXT FK | → projects.id               |
| slug       | TEXT    | UNIQUE within project       |
| name       | TEXT    | Display name (subject name) |
| created_at | TEXT    | ISO 8601                    |
| updated_at | TEXT    | ISO 8601                    |

### accounts

Top-level entity. An account can be linked to multiple subjects.

| Column        | Type    | Notes                            |
| ------------- | ------- | -------------------------------- |
| id            | TEXT PK | UUID                             |
| platform      | TEXT    | e.g. "instagram"                 |
| handle        | TEXT    | Account handle                   |
| cookies_blob  | BLOB    | AES-256-GCM encrypted            |
| last_login_at | TEXT    | ISO 8601, nullable               |
| created_at    | TEXT    | ISO 8601                         |
| updated_at    | TEXT    | ISO 8601                         |

**Unique constraint**: `(platform, handle)`.

### subject_accounts

Join table for the many-to-many relationship between subjects and accounts.

| Column     | Type    | Notes             |
| ---------- | ------- | ----------------- |
| subject_id | TEXT FK | → subjects.id     |
| account_id | TEXT FK | → accounts.id     |
| created_at | TEXT    | ISO 8601          |

**Primary key**: `(subject_id, account_id)`.

### samples

| Column          | Type    | Notes                                    |
| --------------- | ------- | ---------------------------------------- |
| id              | TEXT PK | UUID                                     |
| subject_id      | TEXT FK | → subjects.id                            |
| source_platform | TEXT    | e.g. "instagram"                         |
| source_post_id  | TEXT    | Platform-specific post identifier        |
| slide_index     | INTEGER | 0 for single images, 0-N for carousels   |
| file_path       | TEXT    | Relative to data dir                     |
| sha256          | TEXT    | Hex-encoded file hash                    |
| phash           | TEXT    | Perceptual hash for dedup                |
| width           | INTEGER | Pixels                                   |
| height          | INTEGER | Pixels                                   |
| taken_at        | TEXT    | ISO 8601, nullable                       |
| fetched_at      | TEXT    | ISO 8601                                 |
| status          | TEXT    | 'pending' / 'kept' / 'rejected'          |
| is_duplicate    | INTEGER | 0 or 1                                   |
| duplicate_of    | TEXT    | JSON array of sample UUIDs this is a duplicate of, nullable |
| created_at      | TEXT    | ISO 8601                                 |
| updated_at      | TEXT    | ISO 8601                                 |

**Unique constraint**: `(subject_id, source_post_id, slide_index)` — ensures
idempotent re-pulls.

**Indexes**: `phash`, `sha256`, `status`, `subject_id`, `is_duplicate`.

### edits

| Column            | Type    | Notes                                |
| ----------------- | ------- | ------------------------------------ |
| sample_id         | TEXT PK | → samples.id                         |
| rotation_deg      | REAL    | Free-angle; UI offers 90° increments |
| crop_box_json     | TEXT    | JSON: {x, y, w, h}, nullable        |
| auto_actions_json | TEXT    | JSON array of applied actions        |
| created_at        | TEXT    | ISO 8601                             |
| updated_at        | TEXT    | ISO 8601                             |

### caption_studies

| Column          | Type    | Notes                                 |
| --------------- | ------- | ------------------------------------- |
| id              | TEXT PK | UUID                                  |
| project_id      | TEXT FK | → projects.id                         |
| name            | TEXT    | Display name                          |
| slug            | TEXT    | UNIQUE within project                 |
| provider        | TEXT    | e.g. "anthropic", "local_llama"       |
| model           | TEXT    | Model identifier                      |
| prompt_template | TEXT    | Captioning instruction template       |
| params_json     | TEXT    | JSON: {temperature, max_tokens, ...}  |
| created_at      | TEXT    | ISO 8601                              |
| updated_at      | TEXT    | ISO 8601                              |

### captions

| Column     | Type    | Notes                            |
| ---------- | ------- | -------------------------------- |
| id         | TEXT PK | UUID                             |
| sample_id  | TEXT FK | → samples.id                     |
| study_id   | TEXT FK | → caption_studies.id             |
| text       | TEXT    | Caption text (natural language)  |
| created_at | TEXT    | ISO 8601                         |

Full history retained. The latest caption per study is also written into the
sample's `.json` manifest keyed by study slug.

**Unique constraint**: `(sample_id, study_id)` — one caption per sample per study
(new runs overwrite).

### job_runs

| Column          | Type    | Notes                                           |
| --------------- | ------- | ----------------------------------------------- |
| id              | TEXT PK | UUID                                            |
| type            | TEXT    | 'ig_pull' / 'caption' / 'export'                |
| subject_id      | TEXT FK | → subjects.id, nullable                         |
| study_id        | TEXT FK | → caption_studies.id, nullable (caption jobs)   |
| status          | TEXT    | 'running' / 'succeeded' / 'failed' / 'cancelled' |
| total_items     | INTEGER | Total messages published                        |
| completed_items | INTEGER | Successfully processed                          |
| failed_items    | INTEGER | Moved to DLQ or max retries                     |
| started_at      | TEXT    | ISO 8601                                        |
| finished_at     | TEXT    | ISO 8601, nullable                              |
| created_at      | TEXT    | ISO 8601                                        |

Read-only audit log. Actual queue state lives in NATS JetStream.

### job_messages

Per-job log entries with level filtering. UI defaults to showing warning and above.

| Column         | Type    | Notes                                  |
| -------------- | ------- | -------------------------------------- |
| id             | TEXT PK | UUID                                   |
| job_run_id     | TEXT FK | → job_runs.id                          |
| level          | TEXT    | 'error' / 'warning' / 'info'          |
| sample_ref     | TEXT    | Sample ID or source post ref, nullable |
| account_handle | TEXT    | Which account triggered it, nullable   |
| message        | TEXT    | Log message text                       |
| created_at     | TEXT    | ISO 8601                               |

**Index**: `(job_run_id, level)` for filtered queries.

### secrets

| Column          | Type    | Notes                     |
| --------------- | ------- | ------------------------- |
| key             | TEXT PK | e.g. "anthropic_api_key"  |
| value_encrypted | BLOB    | AES-256-GCM encrypted     |
| created_at      | TEXT    | ISO 8601                  |
| updated_at      | TEXT    | ISO 8601                  |
