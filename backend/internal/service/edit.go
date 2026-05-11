package service

import (
	"encoding/json"
	"fmt"
	"path/filepath"

	logrus "github.com/sirupsen/logrus"

	"github.com/kmacmcfarlane/image-dataset-tool/backend/internal/atomicfile"
	"github.com/kmacmcfarlane/image-dataset-tool/backend/internal/fileformat"
	"github.com/kmacmcfarlane/image-dataset-tool/backend/internal/model"
	"github.com/kmacmcfarlane/image-dataset-tool/backend/internal/store"
)

// EditService handles edit business logic.
type EditService struct {
	editStore   *store.EditStore
	sampleStore *store.SampleStore
	dataDir     string
	logger      *logrus.Entry
}

// NewEditService creates a new EditService.
func NewEditService(editStore *store.EditStore, sampleStore *store.SampleStore, dataDir string) *EditService {
	return &EditService{
		editStore:   editStore,
		sampleStore: sampleStore,
		dataDir:     dataDir,
		logger:      logrus.WithField("component", "edit_service"),
	}
}

// GetEdit returns the edit for a sample, or nil if no edits exist.
func (s *EditService) GetEdit(sampleID string) (*model.Edit, error) {
	s.logger.WithField("sample_id", sampleID).Trace("entering GetEdit")
	defer s.logger.Trace("returning from GetEdit")

	return s.editStore.GetBySampleID(sampleID)
}

// SaveEdit creates/updates the edit for a sample, writes to manifest, and returns the result.
// This is idempotent: saving the same edits twice produces the same state.
func (s *EditService) SaveEdit(edit *model.Edit) (*model.Edit, error) {
	s.logger.WithField("sample_id", edit.SampleID).Trace("entering SaveEdit")
	defer s.logger.Trace("returning from SaveEdit")

	// Validate sample exists and get its metadata for file path.
	sample, err := s.sampleStore.GetByID(edit.SampleID)
	if err != nil {
		return nil, fmt.Errorf("get sample: %w", err)
	}

	// If edit is empty (all defaults), treat as reset.
	if edit.IsEmpty() {
		return s.ResetEdits(edit.SampleID)
	}

	// Step 1: Upsert DB row.
	result, err := s.editStore.Upsert(edit)
	if err != nil {
		return nil, fmt.Errorf("upsert edit: %w", err)
	}

	// Step 2: Update manifest with file lock (PRD 11.1 rule 1+6).
	if err := s.updateManifestEdits(sample, result); err != nil {
		s.logger.WithFields(logrus.Fields{
			"sample_id": edit.SampleID,
			"error":     err.Error(),
		}).Error("failed to update manifest edits")
		return nil, fmt.Errorf("update manifest: %w", err)
	}

	s.logger.WithField("sample_id", edit.SampleID).Info("edit saved")
	return result, nil
}

// ResetEdits clears all edits for a sample, resetting to original.
// Returns nil (no edit exists after reset).
func (s *EditService) ResetEdits(sampleID string) (*model.Edit, error) {
	s.logger.WithField("sample_id", sampleID).Trace("entering ResetEdits")
	defer s.logger.Trace("returning from ResetEdits")

	sample, err := s.sampleStore.GetByID(sampleID)
	if err != nil {
		return nil, fmt.Errorf("get sample: %w", err)
	}

	// Step 1: Delete DB row.
	if err := s.editStore.DeleteBySampleID(sampleID); err != nil {
		return nil, fmt.Errorf("delete edit: %w", err)
	}

	// Step 2: Reset manifest edits to defaults with file lock.
	defaultEdit := &model.Edit{
		SampleID:        sampleID,
		RotationDeg:     0,
		CropBox:         nil,
		AutoActionsJSON: "[]",
	}
	if err := s.updateManifestEdits(sample, defaultEdit); err != nil {
		s.logger.WithFields(logrus.Fields{
			"sample_id": sampleID,
			"error":     err.Error(),
		}).Error("failed to reset manifest edits")
		return nil, fmt.Errorf("reset manifest: %w", err)
	}

	s.logger.WithField("sample_id", sampleID).Info("edits reset to original")
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
