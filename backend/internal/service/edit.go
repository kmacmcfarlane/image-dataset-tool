package service

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"path/filepath"

	"github.com/kmacmcfarlane/image-dataset-tool/backend/internal/atomicfile"
	"github.com/kmacmcfarlane/image-dataset-tool/backend/internal/fileformat"
	"github.com/kmacmcfarlane/image-dataset-tool/backend/internal/model"
)

// EditRepository defines the persistence interface for edits.
type EditRepository interface {
	GetBySampleID(sampleID string) (*model.Edit, error)
	Upsert(edit *model.Edit) (*model.Edit, error)
	DeleteBySampleID(sampleID string) error
}

// SampleReader defines a read-only interface for sample lookups used by EditService.
type SampleReader interface {
	GetByID(id string) (*model.Sample, error)
}

// EditService handles edit business logic.
type EditService struct {
	editStore   EditRepository
	sampleStore SampleReader
	dataDir     string
	logger      *slog.Logger
}

// NewEditService creates a new EditService.
func NewEditService(editStore EditRepository, sampleStore SampleReader, dataDir string) *EditService {
	return &EditService{
		editStore:   editStore,
		sampleStore: sampleStore,
		dataDir:     dataDir,
		logger:      slog.With("component", "edit_service"),
	}
}

// GetEdit returns the edit for a sample, or nil if no edits exist.
// Returns ErrNotFound (via SampleReader) if the sample itself does not exist.
func (s *EditService) GetEdit(sampleID string) (*model.Edit, error) {
	s.logger.Debug("entering GetEdit", "sample_id", sampleID)

	// Verify sample exists before returning edits.
	if _, err := s.sampleStore.GetByID(sampleID); err != nil {
		return nil, fmt.Errorf("get sample: %w", err)
	}

	return s.editStore.GetBySampleID(sampleID)
}

// SaveEdit creates/updates the edit for a sample, writes to manifest, and returns the result.
// This is idempotent: saving the same edits twice produces the same state.
func (s *EditService) SaveEdit(edit *model.Edit) (*model.Edit, error) {
	s.logger.Debug("entering SaveEdit", "sample_id", edit.SampleID)

	// Validate sample exists and get its metadata for file path.
	sample, err := s.sampleStore.GetByID(edit.SampleID)
	if err != nil {
		return nil, fmt.Errorf("get sample: %w", err)
	}

	// If edit is empty (all defaults), treat as reset.
	if edit.IsEmpty() {
		return s.ResetEdits(edit.SampleID)
	}

	// Step 1: Update manifest first with file lock (PRD 11.1 rule 1: manifest before DB).
	// If manifest succeeds but DB fails, reconciler recovers on restart.
	if err := s.updateManifestEdits(sample, edit); err != nil {
		s.logger.Error("failed to update manifest edits", "sample_id", edit.SampleID, "error", err)
		return nil, fmt.Errorf("update manifest: %w", err)
	}

	// Step 2: Upsert DB row.
	result, err := s.editStore.Upsert(edit)
	if err != nil {
		return nil, fmt.Errorf("upsert edit: %w", err)
	}

	s.logger.Info("edit saved", "sample_id", edit.SampleID)
	return result, nil
}

// ResetEdits clears all edits for a sample, resetting to original.
// Returns nil (no edit exists after reset).
func (s *EditService) ResetEdits(sampleID string) (*model.Edit, error) {
	s.logger.Debug("entering ResetEdits", "sample_id", sampleID)

	sample, err := s.sampleStore.GetByID(sampleID)
	if err != nil {
		return nil, fmt.Errorf("get sample: %w", err)
	}

	// Step 1: Reset manifest edits to defaults first with file lock (PRD 11.1 rule 1: manifest before DB).
	// If manifest succeeds but DB fails, reconciler recovers on restart.
	defaultEdit := &model.Edit{
		SampleID:        sampleID,
		RotationDeg:     0,
		CropBox:         nil,
		AutoActionsJSON: "[]",
	}
	if err := s.updateManifestEdits(sample, defaultEdit); err != nil {
		s.logger.Error("failed to reset manifest edits", "sample_id", sampleID, "error", err)
		return nil, fmt.Errorf("reset manifest: %w", err)
	}

	// Step 2: Delete DB row.
	if err := s.editStore.DeleteBySampleID(sampleID); err != nil {
		return nil, fmt.Errorf("delete edit: %w", err)
	}

	s.logger.Info("edits reset to original", "sample_id", sampleID)
	return nil, nil
}

// updateManifestEdits acquires a file lock, reads the manifest, updates edits, and writes back.
func (s *EditService) updateManifestEdits(sample *model.Sample, edit *model.Edit) error {
	manifestPath := s.sampleManifestPath(sample)

	// Acquire file lock for concurrent access safety (PRD 11.1 rule 6).
	lock, err := atomicfile.Lock(manifestPath)
	if err != nil {
		return fmt.Errorf("acquire lock: %w", err)
	}
	defer lock.Unlock()

	// Read current manifest.
	meta, err := fileformat.ReadSampleMetadata(manifestPath)
	if err != nil {
		return fmt.Errorf("read manifest: %w", err)
	}

	// Update edits section.
	meta.Edits = modelEditToFileformat(edit)

	// Atomic write (write-to-temp-then-rename per PRD 11.1 rule 2).
	if err := fileformat.WriteSampleMetadata(manifestPath, meta); err != nil {
		return fmt.Errorf("write manifest: %w", err)
	}

	return nil
}

// sampleManifestPath computes the manifest path for a sample.
// The file_path in the sample is relative to the samples dir; we need to resolve
// to $DATA_DIR/projects/<proj-slug>/subjects/<subj-slug>/samples/<sample-id>.json
func (s *EditService) sampleManifestPath(sample *model.Sample) string {
	// sample.FilePath is something like "projects/<proj-slug>/subjects/<subj-slug>/samples/<id>.jpg"
	// The manifest is the same directory with <id>.json
	dir := filepath.Dir(filepath.Join(s.dataDir, sample.FilePath))
	return filepath.Join(dir, sample.ID+".json")
}

// modelEditToFileformat converts a model.Edit to a fileformat.SampleEdits.
func modelEditToFileformat(edit *model.Edit) fileformat.SampleEdits {
	result := fileformat.SampleEdits{
		RotationDeg: edit.RotationDeg,
		AutoActions: []string{},
	}

	if edit.CropBox != nil {
		result.CropBox = &fileformat.CropBox{
			X: edit.CropBox.X,
			Y: edit.CropBox.Y,
			W: edit.CropBox.W,
			H: edit.CropBox.H,
		}
	}

	if edit.AutoActionsJSON != "" && edit.AutoActionsJSON != "[]" && edit.AutoActionsJSON != "null" {
		var actions []string
		if err := json.Unmarshal([]byte(edit.AutoActionsJSON), &actions); err == nil {
			result.AutoActions = actions
		}
	}

	return result
}
