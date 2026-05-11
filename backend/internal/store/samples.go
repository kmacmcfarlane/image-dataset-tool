package store

import (
	"database/sql"
	"fmt"

	"github.com/kmacmcfarlane/image-dataset-tool/backend/internal/model"
)

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
