package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/kmacmcfarlane/image-dataset-tool/backend/internal/model"
)

// EditStore handles edit persistence in SQLite.
type EditStore struct {
	db *sql.DB
}

// NewEditStore creates a new EditStore.
func NewEditStore(db *sql.DB) *EditStore {
	return &EditStore{db: db}
}

// GetBySampleID returns the edit for a sample, or nil if no edits exist.
func (s *EditStore) GetBySampleID(sampleID string) (*model.Edit, error) {
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
		return nil, fmt.Errorf("get edit by sample id: %w", err)
	}

	edit.CropBox = parseCropBoxJSON(cropBox)
	if autoActions.Valid {
		edit.AutoActionsJSON = autoActions.String
	}

	return &edit, nil
}

// Upsert creates or updates the edits row for a sample.
// Returns the resulting edit.
func (s *EditStore) Upsert(edit *model.Edit) (*model.Edit, error) {
	now := time.Now().UTC().Format(time.RFC3339)

	cropBoxJSON := formatCropBoxJSON(edit.CropBox)
	var autoActionsJSON *string
	if edit.AutoActionsJSON != "" {
		autoActionsJSON = &edit.AutoActionsJSON
	}

	// Use INSERT ... ON CONFLICT for idempotent upsert.
	_, err := s.db.Exec(`
		INSERT INTO edits (sample_id, rotation_deg, crop_box_json, auto_actions_json, created_at, updated_at)
		VALUES (?, ?, ?, ?, COALESCE((SELECT created_at FROM edits WHERE sample_id = ?), ?), ?)
		ON CONFLICT(sample_id) DO UPDATE SET
			rotation_deg = excluded.rotation_deg,
			crop_box_json = excluded.crop_box_json,
			auto_actions_json = excluded.auto_actions_json,
			updated_at = excluded.updated_at`,
		edit.SampleID, edit.RotationDeg, cropBoxJSON, autoActionsJSON,
		edit.SampleID, now, now)
	if err != nil {
		if isForeignKeyConstraint(err) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("upsert edit: %w", err)
	}

	return s.GetBySampleID(edit.SampleID)
}

// DeleteBySampleID removes the edits row for a sample (reset to original).
func (s *EditStore) DeleteBySampleID(sampleID string) error {
	_, err := s.db.Exec("DELETE FROM edits WHERE sample_id = ?", sampleID)
	if err != nil {
		return fmt.Errorf("delete edit: %w", err)
	}
	return nil
}

// parseCropBoxJSON converts a NullString JSON to a model.CropBox, or nil.
func parseCropBoxJSON(ns sql.NullString) *model.CropBox {
	if !ns.Valid || ns.String == "" || ns.String == "null" {
		return nil
	}
	var box model.CropBox
	if err := json.Unmarshal([]byte(ns.String), &box); err != nil {
		return nil
	}
	return &box
}

// formatCropBoxJSON converts a model.CropBox to a JSON string pointer, or nil.
func formatCropBoxJSON(box *model.CropBox) *string {
	if box == nil {
		return nil
	}
	data, err := json.Marshal(box)
	if err != nil {
		return nil
	}
	s := string(data)
	return &s
}
