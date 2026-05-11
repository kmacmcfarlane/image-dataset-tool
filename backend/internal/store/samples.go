package store

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/kmacmcfarlane/image-dataset-tool/backend/internal/model"
)

// validStatuses defines the allowed sample status values.
var validStatuses = map[string]bool{
	"pending":  true,
	"kept":     true,
	"rejected": true,
}

// SampleStore handles sample persistence in SQLite.
type SampleStore struct {
	db *sql.DB
}

// NewSampleStore creates a new SampleStore.
func NewSampleStore(db *sql.DB) *SampleStore {
	return &SampleStore{db: db}
}

// ListBySubject returns all samples for a subject (no pagination per PRD §8.3).
func (s *SampleStore) ListBySubject(subjectID string) ([]*model.Sample, error) {
	rows, err := s.db.Query(`
		SELECT id, subject_id, source_platform, source_post_id, slide_index,
		       file_path, sha256, phash, width, height, taken_at, fetched_at,
		       status, is_duplicate, duplicate_of, created_at, updated_at
		FROM samples
		WHERE subject_id = ?
		ORDER BY created_at DESC`, subjectID)
	if err != nil {
		return nil, fmt.Errorf("list samples by subject: %w", err)
	}
	defer rows.Close()

	var samples []*model.Sample
	for rows.Next() {
		var sample model.Sample
		var takenAt sql.NullString
		var duplicateOf sql.NullString
		var isDup int

		if err := rows.Scan(
			&sample.ID, &sample.SubjectID, &sample.SourcePlatform,
			&sample.SourcePostID, &sample.SlideIndex, &sample.FilePath,
			&sample.SHA256, &sample.PHash, &sample.Width, &sample.Height,
			&takenAt, &sample.FetchedAt, &sample.Status,
			&isDup, &duplicateOf, &sample.CreatedAt, &sample.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan sample: %w", err)
		}

		sample.TakenAt = takenAt.String
		sample.DuplicateOf = duplicateOf.String
		sample.IsDuplicate = isDup != 0

		samples = append(samples, &sample)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate samples: %w", err)
	}
	return samples, nil
}

// GetByID returns a single sample by ID.
func (s *SampleStore) GetByID(id string) (*model.Sample, error) {
	row := s.db.QueryRow(`
		SELECT id, subject_id, source_platform, source_post_id, slide_index,
		       file_path, sha256, phash, width, height, taken_at, fetched_at,
		       status, is_duplicate, duplicate_of, created_at, updated_at
		FROM samples
		WHERE id = ?`, id)

	var sample model.Sample
	var takenAt sql.NullString
	var duplicateOf sql.NullString
	var isDup int

	if err := row.Scan(
		&sample.ID, &sample.SubjectID, &sample.SourcePlatform,
		&sample.SourcePostID, &sample.SlideIndex, &sample.FilePath,
		&sample.SHA256, &sample.PHash, &sample.Width, &sample.Height,
		&takenAt, &sample.FetchedAt, &sample.Status,
		&isDup, &duplicateOf, &sample.CreatedAt, &sample.UpdatedAt,
	); err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get sample by id: %w", err)
	}

	sample.TakenAt = takenAt.String
	sample.DuplicateOf = duplicateOf.String
	sample.IsDuplicate = isDup != 0

	return &sample, nil
}

// UpdateStatus updates the status of a single sample and returns the updated sample.
func (s *SampleStore) UpdateStatus(id string, status string) (*model.Sample, error) {
	if !validStatuses[status] {
		return nil, fmt.Errorf("invalid status %q: must be pending, kept, or rejected", status)
	}

	now := time.Now().UTC().Format(time.RFC3339)
	result, err := s.db.Exec(`
		UPDATE samples SET status = ?, updated_at = ?
		WHERE id = ?`, status, now, id)
	if err != nil {
		return nil, fmt.Errorf("update sample status: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return nil, fmt.Errorf("check rows affected: %w", err)
	}
	if rows == 0 {
		return nil, ErrNotFound
	}

	return s.GetByID(id)
}

// BulkUpdateStatus atomically updates the status of multiple samples within a transaction.
// Returns the number of rows updated. If any sample ID is not found, it is skipped.
func (s *SampleStore) BulkUpdateStatus(ids []string, status string) (int64, error) {
	if !validStatuses[status] {
		return 0, fmt.Errorf("invalid status %q: must be pending, kept, or rejected", status)
	}
	if len(ids) == 0 {
		return 0, nil
	}

	now := time.Now().UTC().Format(time.RFC3339)

	tx, err := s.db.Begin()
	if err != nil {
		return 0, fmt.Errorf("begin bulk status transaction: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck

	// Build placeholders for IN clause.
	placeholders := make([]string, len(ids))
	args := make([]any, 0, len(ids)+2)
	args = append(args, status, now)
	for i, id := range ids {
		placeholders[i] = "?"
		args = append(args, id)
	}

	query := fmt.Sprintf(`
		UPDATE samples SET status = ?, updated_at = ?
		WHERE id IN (%s)`, strings.Join(placeholders, ","))

	result, err := tx.Exec(query, args...)
	if err != nil {
		return 0, fmt.Errorf("bulk update sample status: %w", err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("check bulk rows affected: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("commit bulk status transaction: %w", err)
	}

	return affected, nil
}
