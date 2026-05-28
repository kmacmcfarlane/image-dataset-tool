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

// GetProjectIDForSubject returns the project_id for a given subject_id.
func (s *SampleStore) GetProjectIDForSubject(subjectID string) (string, error) {
	row := s.db.QueryRow("SELECT project_id FROM subjects WHERE id = ?", subjectID)
	var projectID string
	if err := row.Scan(&projectID); err != nil {
		if err == sql.ErrNoRows {
			return "", ErrNotFound
		}
		return "", fmt.Errorf("get project id for subject: %w", err)
	}
	return projectID, nil
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

// InsertOrIgnore inserts a sample row, ignoring conflicts on the unique
// constraint (subject_id, source_post_id, slide_index). This provides
// redelivery idempotency for the media.process pipeline consumer.
// Returns true if the row was inserted, false if it already existed.
func (s *SampleStore) InsertOrIgnore(sample *model.Sample) (bool, error) {
	now := time.Now().UTC().Format(time.RFC3339)
	if sample.CreatedAt == "" {
		sample.CreatedAt = now
	}
	if sample.UpdatedAt == "" {
		sample.UpdatedAt = now
	}

	isDup := 0
	if sample.IsDuplicate {
		isDup = 1
	}

	var duplicateOf sql.NullString
	if sample.DuplicateOf != "" {
		duplicateOf = sql.NullString{String: sample.DuplicateOf, Valid: true}
	}

	var takenAt sql.NullString
	if sample.TakenAt != "" {
		takenAt = sql.NullString{String: sample.TakenAt, Valid: true}
	}

	result, err := s.db.Exec(`
		INSERT OR IGNORE INTO samples
			(id, subject_id, source_platform, source_post_id, slide_index,
			 file_path, sha256, phash, width, height, taken_at, fetched_at,
			 status, is_duplicate, duplicate_of, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		sample.ID, sample.SubjectID, sample.SourcePlatform,
		sample.SourcePostID, sample.SlideIndex, sample.FilePath,
		sample.SHA256, sample.PHash, sample.Width, sample.Height,
		takenAt, sample.FetchedAt, sample.Status,
		isDup, duplicateOf, sample.CreatedAt, sample.UpdatedAt,
	)
	if err != nil {
		return false, fmt.Errorf("insert or ignore sample: %w", err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("check rows affected on insert: %w", err)
	}
	return affected > 0, nil
}

// FindDuplicatesBySHA256 returns sample IDs within a subject that have the exact sha256.
func (s *SampleStore) FindDuplicatesBySHA256(subjectID, sha256 string) ([]string, error) {
	rows, err := s.db.Query(`
		SELECT id FROM samples
		WHERE subject_id = ? AND sha256 = ?`, subjectID, sha256)
	if err != nil {
		return nil, fmt.Errorf("find duplicates by sha256: %w", err)
	}
	defer rows.Close()

	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan duplicate id: %w", err)
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// FindDuplicatesByPHash returns sample IDs within a subject whose pHash is within
// the given Hamming distance threshold of the target pHash. Comparison is done
// in Go after loading all pHash values for the subject.
func (s *SampleStore) FindDuplicatesByPHash(subjectID, targetPHash string, threshold int) ([]string, error) {
	rows, err := s.db.Query(`
		SELECT id, phash FROM samples
		WHERE subject_id = ? AND phash != ''`, subjectID)
	if err != nil {
		return nil, fmt.Errorf("find duplicates by phash: %w", err)
	}
	defer rows.Close()

	var ids []string
	for rows.Next() {
		var id, phash string
		if err := rows.Scan(&id, &phash); err != nil {
			return nil, fmt.Errorf("scan phash row: %w", err)
		}
		dist := hammingDistance(targetPHash, phash)
		if dist >= 0 && dist <= threshold {
			ids = append(ids, id)
		}
	}
	return ids, rows.Err()
}

// hammingDistance computes the Hamming distance between two hex-encoded
// pHash strings. Returns -1 if the strings have different lengths.
func hammingDistance(a, b string) int {
	if len(a) != len(b) {
		return -1
	}
	dist := 0
	for i := 0; i < len(a); i++ {
		x := hexVal(a[i])
		y := hexVal(b[i])
		if x < 0 || y < 0 {
			return -1
		}
		v := x ^ y
		for v != 0 {
			dist++
			v &= v - 1
		}
	}
	return dist
}

// hexVal returns the numeric value of a hex digit, or -1 if invalid.
func hexVal(c byte) int {
	switch {
	case c >= '0' && c <= '9':
		return int(c - '0')
	case c >= 'a' && c <= 'f':
		return int(c-'a') + 10
	case c >= 'A' && c <= 'F':
		return int(c-'A') + 10
	default:
		return -1
	}
}
