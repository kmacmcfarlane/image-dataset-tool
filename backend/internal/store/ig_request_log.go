package store

import (
	"database/sql"
	"fmt"

	"github.com/kmacmcfarlane/image-dataset-tool/backend/internal/model"
)

// IGRequestLogStore handles ig_request_log persistence in SQLite.
type IGRequestLogStore struct {
	db *sql.DB
}

// NewIGRequestLogStore creates a new IGRequestLogStore.
func NewIGRequestLogStore(db *sql.DB) *IGRequestLogStore {
	return &IGRequestLogStore{db: db}
}

// Insert writes a request log entry.
func (s *IGRequestLogStore) Insert(log *model.IGRequestLog) error {
	_, err := s.db.Exec(`
		INSERT INTO ig_request_log
			(timestamp, account_id, endpoint, http_status, is_throttled, is_denied,
			 response_time_ms, public_ip, error_detail)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		log.Timestamp.UTC().Format("2006-01-02T15:04:05Z"),
		log.AccountID,
		log.Endpoint,
		log.HTTPStatus,
		boolToInt(log.IsThrottled),
		boolToInt(log.IsDenied),
		log.ResponseTimeMs,
		log.PublicIP,
		log.ErrorDetail,
	)
	if err != nil {
		return fmt.Errorf("insert ig_request_log: %w", err)
	}
	return nil
}

// LastIPForAccount returns the public IP from the most recent request for an account.
// Returns empty string if no prior requests exist.
func (s *IGRequestLogStore) LastIPForAccount(accountID string) (string, error) {
	var ip sql.NullString
	err := s.db.QueryRow(`
		SELECT public_ip FROM ig_request_log
		WHERE account_id = ? AND public_ip != ''
		ORDER BY timestamp DESC
		LIMIT 1`, accountID).Scan(&ip)
	if err != nil {
		if err == sql.ErrNoRows {
			return "", nil
		}
		return "", fmt.Errorf("query last IP: %w", err)
	}
	if !ip.Valid {
		return "", nil
	}
	return ip.String, nil
}

// boolToInt converts a bool to SQLite integer (0/1).
func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
