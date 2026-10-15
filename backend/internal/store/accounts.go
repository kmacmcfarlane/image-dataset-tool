package store

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/kmacmcfarlane/image-dataset-tool/backend/internal/crypto"
	"github.com/kmacmcfarlane/image-dataset-tool/backend/internal/model"
)

// AccountStore handles account persistence in SQLite with cookie encryption.
type AccountStore struct {
	db     *sql.DB
	encKey []byte
}

// NewAccountStore creates a new AccountStore.
func NewAccountStore(db *sql.DB, encKey []byte) *AccountStore {
	return &AccountStore{db: db, encKey: encKey}
}

// Create inserts a new account row. Returns ErrDuplicateAccount if (platform, handle) already exists.
func (s *AccountStore) Create(a *model.Account) error {
	now := time.Now().UTC().Format(time.RFC3339)
	a.CreatedAt = now
	a.UpdatedAt = now

	_, err := s.db.Exec(
		`INSERT INTO accounts (id, platform, handle, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?)`,
		a.ID, a.Platform, a.Handle, a.CreatedAt, a.UpdatedAt,
	)
	if err != nil {
		if isDuplicateConstraint(err) {
			return fmt.Errorf("account (%s, %s) already exists: %w", a.Platform, a.Handle, ErrDuplicateAccount)
		}
		return fmt.Errorf("insert account: %w", err)
	}
	return nil
}

// Get retrieves an account by ID. Never returns cookies.
func (s *AccountStore) Get(id string) (*model.Account, error) {
	row := s.db.QueryRow(`
		SELECT id, platform, handle,
		       CASE WHEN cookies_blob IS NOT NULL THEN 1 ELSE 0 END AS has_cookies,
		       COALESCE(last_login_at, '') AS last_login_at,
		       created_at, updated_at
		FROM accounts
		WHERE id = ?`, id)

	var a model.Account
	err := row.Scan(&a.ID, &a.Platform, &a.Handle, &a.HasCookies, &a.LastLoginAt, &a.CreatedAt, &a.UpdatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("account %q: %w", id, ErrNotFound)
		}
		return nil, fmt.Errorf("get account: %w", err)
	}
	return &a, nil
}

// GetDetail retrieves an account by ID with linked subjects. Never returns cookies.
func (s *AccountStore) GetDetail(id string) (*model.AccountDetail, error) {
	a, err := s.Get(id)
	if err != nil {
		return nil, err
	}

	subjects, err := s.getLinkedSubjects(id)
	if err != nil {
		return nil, fmt.Errorf("get linked subjects: %w", err)
	}

	return &model.AccountDetail{
		ID:             a.ID,
		Platform:       a.Platform,
		Handle:         a.Handle,
		HasCookies:     a.HasCookies,
		LastLoginAt:    a.LastLoginAt,
		LinkedSubjects: subjects,
		CreatedAt:      a.CreatedAt,
		UpdatedAt:      a.UpdatedAt,
	}, nil
}

// List returns all accounts ordered by creation time. Never returns cookies.
func (s *AccountStore) List() ([]*model.Account, error) {
	rows, err := s.db.Query(`
		SELECT id, platform, handle,
		       CASE WHEN cookies_blob IS NOT NULL THEN 1 ELSE 0 END AS has_cookies,
		       COALESCE(last_login_at, '') AS last_login_at,
		       created_at, updated_at
		FROM accounts
		ORDER BY created_at DESC`)
	if err != nil {
		return nil, fmt.Errorf("list accounts: %w", err)
	}
	defer rows.Close()

	var accounts []*model.Account
	for rows.Next() {
		var a model.Account
		if err := rows.Scan(&a.ID, &a.Platform, &a.Handle, &a.HasCookies, &a.LastLoginAt, &a.CreatedAt, &a.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan account: %w", err)
		}
		accounts = append(accounts, &a)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate accounts: %w", err)
	}
	return accounts, nil
}

// UpdateCookies encrypts and stores cookies for an account.
func (s *AccountStore) UpdateCookies(id string, cookies []byte) error {
	encrypted, err := crypto.Encrypt(s.encKey, cookies)
	if err != nil {
		return fmt.Errorf("encrypt cookies: %w", err)
	}

	now := time.Now().UTC().Format(time.RFC3339)
	result, err := s.db.Exec(
		`UPDATE accounts SET cookies_blob = ?, updated_at = ? WHERE id = ?`,
		encrypted, now, id,
	)
	if err != nil {
		return fmt.Errorf("update cookies: %w", err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("check rows affected: %w", err)
	}
	if affected == 0 {
		return fmt.Errorf("account %q: %w", id, ErrNotFound)
	}
	return nil
}

// UpdateCookiesWithSessionExpiry encrypts and stores cookies and sets
// session_expires_at in one UPDATE, so a cookie import never leaves the
// cookies saved with a stale expiry.
func (s *AccountStore) UpdateCookiesWithSessionExpiry(id string, cookies []byte, expiresAt time.Time) error {
	encrypted, err := crypto.Encrypt(s.encKey, cookies)
	if err != nil {
		return fmt.Errorf("encrypt cookies: %w", err)
	}

	now := time.Now().UTC().Format(time.RFC3339)
	result, err := s.db.Exec(
		`UPDATE accounts SET cookies_blob = ?, session_expires_at = ?, updated_at = ? WHERE id = ?`,
		encrypted, expiresAt.UTC().Format(time.RFC3339), now, id,
	)
	if err != nil {
		return fmt.Errorf("update cookies and session_expires_at: %w", err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("check rows affected: %w", err)
	}
	if affected == 0 {
		return fmt.Errorf("account %q: %w", id, ErrNotFound)
	}
	return nil
}

// UpdateSessionExpiry sets the session_expires_at timestamp for an account.
func (s *AccountStore) UpdateSessionExpiry(id string, expiresAt time.Time) error {
	now := time.Now().UTC().Format(time.RFC3339)
	result, err := s.db.Exec(
		`UPDATE accounts SET session_expires_at = ?, updated_at = ? WHERE id = ?`,
		expiresAt.UTC().Format(time.RFC3339), now, id,
	)
	if err != nil {
		return fmt.Errorf("update session_expires_at: %w", err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("check rows affected: %w", err)
	}
	if affected == 0 {
		return fmt.Errorf("account %q: %w", id, ErrNotFound)
	}
	return nil
}

// GetSessionExpiry returns the session_expires_at timestamp for an account.
// Returns zero time if not set.
func (s *AccountStore) GetSessionExpiry(id string) (time.Time, error) {
	var expiresAt sql.NullString
	err := s.db.QueryRow(`SELECT session_expires_at FROM accounts WHERE id = ?`, id).Scan(&expiresAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return time.Time{}, fmt.Errorf("account %q: %w", id, ErrNotFound)
		}
		return time.Time{}, fmt.Errorf("get session_expires_at: %w", err)
	}
	if !expiresAt.Valid || expiresAt.String == "" {
		return time.Time{}, nil
	}
	t, err := time.Parse(time.RFC3339, expiresAt.String)
	if err != nil {
		return time.Time{}, fmt.Errorf("parse session_expires_at: %w", err)
	}
	return t, nil
}

// GetCookies retrieves and decrypts cookies for an account (internal use only).
func (s *AccountStore) GetCookies(id string) ([]byte, error) {
	var blob []byte
	err := s.db.QueryRow(`SELECT cookies_blob FROM accounts WHERE id = ?`, id).Scan(&blob)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("account %q: %w", id, ErrNotFound)
		}
		return nil, fmt.Errorf("get cookies: %w", err)
	}

	if blob == nil {
		return nil, nil
	}

	plaintext, err := crypto.Decrypt(s.encKey, blob)
	if err != nil {
		return nil, fmt.Errorf("decrypt cookies: %w", err)
	}
	return plaintext, nil
}

// Delete removes an account by ID. CASCADE deletes subject_accounts links.
func (s *AccountStore) Delete(id string) error {
	result, err := s.db.Exec("DELETE FROM accounts WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("delete account: %w", err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("check rows affected: %w", err)
	}
	if affected == 0 {
		return fmt.Errorf("account %q: %w", id, ErrNotFound)
	}
	return nil
}

// HandleExists checks if a (platform, handle) pair already exists.
func (s *AccountStore) HandleExists(platform, handle string) (bool, error) {
	var count int
	err := s.db.QueryRow(
		"SELECT COUNT(*) FROM accounts WHERE platform = ? AND handle = ?",
		platform, handle,
	).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("check handle existence: %w", err)
	}
	return count > 0, nil
}

// getLinkedSubjects returns subjects linked to an account.
func (s *AccountStore) getLinkedSubjects(accountID string) ([]model.AccountLinkedSubject, error) {
	rows, err := s.db.Query(`
		SELECT sub.id, sub.project_id, sub.name
		FROM subjects sub
		JOIN subject_accounts sa ON sa.subject_id = sub.id
		WHERE sa.account_id = ?
		ORDER BY sub.name`, accountID)
	if err != nil {
		return nil, fmt.Errorf("query linked subjects: %w", err)
	}
	defer rows.Close()

	var subjects []model.AccountLinkedSubject
	for rows.Next() {
		var s model.AccountLinkedSubject
		if err := rows.Scan(&s.ID, &s.ProjectID, &s.Name); err != nil {
			return nil, fmt.Errorf("scan linked subject: %w", err)
		}
		subjects = append(subjects, s)
	}
	return subjects, rows.Err()
}
