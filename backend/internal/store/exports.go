package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/kmacmcfarlane/image-dataset-tool/backend/internal/model"
)

// ExportStore provides persistence operations for the zip export service.
type ExportStore struct {
	db *sql.DB
}

// NewExportStore creates a new ExportStore.
func NewExportStore(db *sql.DB) *ExportStore {
	return &ExportStore{db: db}
}

// ResolveSubjectInfo returns project slug, subject slug, and project ID for a subject.
func (s *ExportStore) ResolveSubjectInfo(_ context.Context, subjectID string) (string, string, string, error) {
	row := s.db.QueryRow(`
		SELECT p.slug, sub.slug, p.id
		FROM subjects sub
		JOIN projects p ON p.id = sub.project_id
		WHERE sub.id = ?`, subjectID)

	var projectSlug, subjectSlug, projectID string
	if err := row.Scan(&projectSlug, &subjectSlug, &projectID); err != nil {
		return "", "", "", err
	}
	return projectSlug, subjectSlug, projectID, nil
}

// ResolveStudy returns the study ID for a project ID + slug.
func (s *ExportStore) ResolveStudy(_ context.Context, projectID, studySlug string) (string, error) {
	row := s.db.QueryRow(`
		SELECT id FROM caption_studies
		WHERE project_id = ? AND slug = ?`, projectID, studySlug)

	var studyID string
	if err := row.Scan(&studyID); err != nil {
		return "", err
	}
	return studyID, nil
}

// ListKeptSamples returns all samples with status='kept' for a subject.
func (s *ExportStore) ListKeptSamples(_ context.Context, subjectID string) ([]*model.Sample, error) {
	rows, err := s.db.Query(`
		SELECT id, subject_id, source_platform, source_post_id, slide_index,
		       file_path, sha256, phash, width, height, taken_at, fetched_at,
		       status, is_duplicate, duplicate_of, created_at, updated_at
		FROM samples
		WHERE subject_id = ? AND status = 'kept'
		ORDER BY created_at ASC`, subjectID)
	if err != nil {
		return nil, fmt.Errorf("query kept samples: %w", err)
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

// GetEdit returns the edit for a sample, or nil if none.
func (s *ExportStore) GetEdit(_ context.Context, sampleID string) (*model.Edit, error) {
	row := s.db.QueryRow(`
		SELECT sample_id, rotation_deg, crop_box_json, auto_actions_json, created_at, updated_at
		FROM edits
		WHERE sample_id = ?`, sampleID)

	var edit model.Edit
	var cropBox sql.NullString
	var autoActions sql.NullString

	if err := row.Scan(
		&edit.SampleID, &edit.RotationDeg, &cropBox, &autoActions,
		&edit.CreatedAt, &edit.UpdatedAt,
	); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("get edit: %w", err)
	}

	if cropBox.Valid && cropBox.String != "" && cropBox.String != "null" {
		var box model.CropBox
		if err := json.Unmarshal([]byte(cropBox.String), &box); err == nil {
			edit.CropBox = &box
		}
	}
	if autoActions.Valid {
		edit.AutoActionsJSON = autoActions.String
	}
	return &edit, nil
}

// GetCaption returns the caption text for a sample+study pair.
func (s *ExportStore) GetCaption(_ context.Context, sampleID, studyID string) (string, bool, error) {
	row := s.db.QueryRow(`
		SELECT text FROM captions
		WHERE sample_id = ? AND study_id = ?`, sampleID, studyID)

	var text string
	if err := row.Scan(&text); err != nil {
		if err == sql.ErrNoRows {
			return "", false, nil
		}
		return "", false, fmt.Errorf("get caption: %w", err)
	}
	return text, true, nil
}
