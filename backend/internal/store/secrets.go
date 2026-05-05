package store

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/kmacmcfarlane/image-dataset-tool/backend/internal/crypto"
	"github.com/kmacmcfarlane/image-dataset-tool/backend/internal/model"
)

// SecretsStore handles encrypted secret storage in SQLite.
type SecretsStore struct {
	db  *sql.DB
	key []byte
}

// NewSecretsStore creates a new SecretsStore with the given DB and encryption key.
func NewSecretsStore(db *sql.DB, key []byte) *SecretsStore {
	return &SecretsStore{db: db, key: key}
}

// List returns all secret entries (keys only, not values).
func (s *SecretsStore) List() ([]*model.SecretEntry, error) {
	rows, err := s.db.Query(
		`SELECT key, created_at, updated_at FROM secrets ORDER BY key`,
	)
	if err != nil {
		return nil, fmt.Errorf("list secrets: %w", err)
	}
	defer rows.Close()

	var entries []*model.SecretEntry
	for rows.Next() {
		e := &model.SecretEntry{}
		if err := rows.Scan(&e.Key, &e.CreatedAt, &e.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan secret row: %w", err)
		}
		entries = append(entries, e)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate secrets: %w", err)
	}
	return entries, nil
}

// Set creates or updates a secret, encrypting the value before storage.
func (s *SecretsStore) Set(key, plaintext string) error {
	encrypted, err := crypto.Encrypt(s.key, []byte(plaintext))
	if err != nil {
		return fmt.Errorf("encrypt secret: %w", err)
	}

	now := time.Now().UTC().Format(time.RFC3339)
	_, err = s.db.Exec(
		`INSERT INTO secrets (key, value_encrypted, created_at, updated_at)
		 VALUES (?, ?, ?, ?)
		 ON CONFLICT(key) DO UPDATE SET value_encrypted = excluded.value_encrypted, updated_at = excluded.updated_at`,
		key, encrypted, now, now,
	)
	if err != nil {
		return fmt.Errorf("upsert secret: %w", err)
	}
	return nil
}

// Delete removes a secret by key. Returns nil even if the key does not exist.
func (s *SecretsStore) Delete(key string) error {
	_, err := s.db.Exec(`DELETE FROM secrets WHERE key = ?`, key)
	if err != nil {
		return fmt.Errorf("delete secret: %w", err)
	}
	return nil
}

// Get retrieves and decrypts a secret value by key.
// Returns sql.ErrNoRows if the key does not exist.
func (s *SecretsStore) Get(key string) (string, error) {
	var encrypted []byte
	err := s.db.QueryRow(
		`SELECT value_encrypted FROM secrets WHERE key = ?`, key,
	).Scan(&encrypted)
	if err != nil {
		return "", fmt.Errorf("get secret: %w", err)
	}

	plaintext, err := crypto.Decrypt(s.key, encrypted)
	if err != nil {
		return "", fmt.Errorf("decrypt secret: %w", err)
	}
	return string(plaintext), nil
}
