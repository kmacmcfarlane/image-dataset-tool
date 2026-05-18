// Package kohya implements the DatasetWriter for kohya-ss training format.
// Output is paired <sample-id>.jpg + <sample-id>.txt files in the output directory.
// The .txt file is omitted when no caption is available for the selected study.
package kohya

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/kmacmcfarlane/image-dataset-tool/backend/internal/export"
	"github.com/kmacmcfarlane/image-dataset-tool/backend/internal/model"
)

// Writer implements export.DatasetWriter for the kohya-ss dataset format.
// Each kept sample becomes a <sample-id>.jpg (edit-materialized) file plus an
// optional <sample-id>.txt caption file (omitted when the study has no caption).
type Writer struct {
	db           *sql.DB
	materializer *export.EditMaterializer
	logger       *slog.Logger
}

// NewWriter creates a new kohya-ss Writer.
// dataDir is the root data directory (used by the EditMaterializer to resolve
// sample file paths).
func NewWriter(db *sql.DB, dataDir string) *Writer {
	return &Writer{
		db:           db,
		materializer: export.NewEditMaterializer(dataDir),
		logger:       slog.With("component", "kohya_writer"),
	}
}

// Write exports kept samples for the subject identified by opts.SubjectSlug
// (within opts.ProjectSlug). Edits are materialized and written as .jpg files.
// If a caption study slug is provided in opts.StudySlug and a caption exists for
// a sample in that study, a paired .txt file is written; otherwise the .txt is
// omitted. Returns a summary of the export run.
func (w *Writer) Write(ctx context.Context, opts model.ExportOptions) (*model.ExportResult, error) {
	w.logger.Info("starting kohya-ss export",
		"project", opts.ProjectSlug,
		"subject", opts.SubjectSlug,
		"study", opts.StudySlug,
		"dir", opts.OutputDir,
	)

	// Resolve subject ID from slugs.
	subjectID, projectID, err := w.resolveSubject(opts.ProjectSlug, opts.SubjectSlug)
	if err != nil {
		return nil, fmt.Errorf("resolve subject: %w", err)
	}

	// Resolve study ID from slug (if provided).
	studyID := ""
	if opts.StudySlug != "" {
		studyID, err = w.resolveStudy(projectID, opts.StudySlug)
		if err != nil {
			return nil, fmt.Errorf("resolve study: %w", err)
		}
	}

	// Fetch kept samples for the subject.
	samples, err := w.listKeptSamples(subjectID)
	if err != nil {
		return nil, fmt.Errorf("list kept samples: %w", err)
	}

	// Ensure output directory exists.
	if err := os.MkdirAll(opts.OutputDir, 0755); err != nil {
		return nil, fmt.Errorf("create output dir %s: %w", opts.OutputDir, err)
	}

	result := &model.ExportResult{OutputDir: opts.OutputDir}

	for _, sample := range samples {
		if err := ctx.Err(); err != nil {
			return result, err
		}

		if err := w.writeSample(ctx, sample, studyID, opts.OutputDir, result); err != nil {
			errMsg := fmt.Sprintf("sample %s: %v", sample.ID, err)
			w.logger.Error("export sample failed", "sample_id", sample.ID, "error", err)
			result.Errors = append(result.Errors, errMsg)
			result.SkippedCount++
			continue
		}
		result.SampleCount++
	}

	w.logger.Info("kohya-ss export complete",
		"samples", result.SampleCount,
		"captions", result.CaptionCount,
		"skipped", result.SkippedCount,
	)

	return result, nil
}

// writeSample materializes the image, writes the .jpg file, and (if a caption
// exists) writes the .txt file.
func (w *Writer) writeSample(
	ctx context.Context,
	sample *model.Sample,
	studyID string,
	outputDir string,
	result *model.ExportResult,
) error {
	_ = ctx // reserved for future cancellation hooks

	// Fetch edit (may be nil).
	edit, err := w.getEdit(sample.ID)
	if err != nil {
		return fmt.Errorf("get edit: %w", err)
	}

	// Materialize image bytes (apply rotation/crop).
	mat, err := w.materializer.Materialize(sample, edit)
	if err != nil {
		return fmt.Errorf("materialize: %w", err)
	}

	// Write image file as .jpg regardless of source format (kohya-ss expects JPEGs).
	imgPath := filepath.Join(outputDir, sample.ID+".jpg")
	if err := os.WriteFile(imgPath, mat.Data, 0644); err != nil {
		return fmt.Errorf("write image file: %w", err)
	}

	w.logger.Debug("wrote image file", "sample_id", sample.ID, "width", mat.Width, "height", mat.Height)

	// Optionally write caption .txt.
	if studyID != "" {
		captionText, found, err := w.getCaption(sample.ID, studyID)
		if err != nil {
			return fmt.Errorf("get caption: %w", err)
		}
		if found {
			txtPath := filepath.Join(outputDir, sample.ID+".txt")
			if err := os.WriteFile(txtPath, []byte(captionText), 0644); err != nil {
				return fmt.Errorf("write caption file: %w", err)
			}
			result.CaptionCount++
			w.logger.Debug("wrote caption file", "sample_id", sample.ID)
		}
	}

	return nil
}

// resolveSubject returns (subjectID, projectID) for the given slugs.
func (w *Writer) resolveSubject(projectSlug, subjectSlug string) (string, string, error) {
	row := w.db.QueryRow(`
		SELECT s.id, s.project_id
		FROM subjects s
		JOIN projects p ON p.id = s.project_id
		WHERE p.slug = ? AND s.slug = ?`, projectSlug, subjectSlug)

	var subjectID, projectID string
	if err := row.Scan(&subjectID, &projectID); err != nil {
		if err == sql.ErrNoRows {
			return "", "", fmt.Errorf("subject %q not found in project %q", subjectSlug, projectSlug)
		}
		return "", "", fmt.Errorf("query subject: %w", err)
	}
	return subjectID, projectID, nil
}

// resolveStudy returns the study ID for the given project ID + slug.
func (w *Writer) resolveStudy(projectID, studySlug string) (string, error) {
	row := w.db.QueryRow(`
		SELECT id FROM caption_studies
		WHERE project_id = ? AND slug = ?`, projectID, studySlug)

	var studyID string
	if err := row.Scan(&studyID); err != nil {
		if err == sql.ErrNoRows {
			return "", fmt.Errorf("caption study %q not found in project", studySlug)
		}
		return "", fmt.Errorf("query study: %w", err)
	}
	return studyID, nil
}

// listKeptSamples returns all samples with status='kept' for a subject.
func (w *Writer) listKeptSamples(subjectID string) ([]*model.Sample, error) {
	rows, err := w.db.Query(`
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

// getEdit returns the edit for a sample, or nil if none.
func (w *Writer) getEdit(sampleID string) (*model.Edit, error) {
	row := w.db.QueryRow(`
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

	edit.CropBox = parseCropBoxJSON(cropBox)
	if autoActions.Valid {
		edit.AutoActionsJSON = autoActions.String
	}
	return &edit, nil
}

// getCaption returns the caption text for a sample+study pair.
// Returns ("", false, nil) when no caption exists.
func (w *Writer) getCaption(sampleID, studyID string) (string, bool, error) {
	row := w.db.QueryRow(`
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

// WriteSingle exports a single sample by ID. This is used by the NATS pipeline
// handler to process one sample per message.
func (w *Writer) WriteSingle(ctx context.Context, opts model.ExportOptions, sampleID string) error {
	// Ensure output directory exists.
	if err := os.MkdirAll(opts.OutputDir, 0755); err != nil {
		return fmt.Errorf("create output dir %s: %w", opts.OutputDir, err)
	}

	// Fetch the sample.
	sample, err := w.getSample(sampleID)
	if err != nil {
		return fmt.Errorf("get sample %s: %w", sampleID, err)
	}

	// Resolve study ID from slug (if provided).
	studyID := ""
	if opts.StudySlug != "" && opts.ProjectSlug != "" {
		// Need project ID from subject.
		var projectID string
		err := w.db.QueryRow(`
			SELECT p.id FROM projects p
			JOIN subjects s ON s.project_id = p.id
			WHERE p.slug = ? AND s.id = ?`, opts.ProjectSlug, sample.SubjectID).Scan(&projectID)
		if err != nil {
			return fmt.Errorf("resolve project for sample %s: %w", sampleID, err)
		}
		studyID, err = w.resolveStudy(projectID, opts.StudySlug)
		if err != nil {
			return fmt.Errorf("resolve study: %w", err)
		}
	}

	result := &model.ExportResult{OutputDir: opts.OutputDir}
	if err := w.writeSample(ctx, sample, studyID, opts.OutputDir, result); err != nil {
		return fmt.Errorf("write sample %s: %w", sampleID, err)
	}

	return nil
}

// getSample fetches a single sample by ID.
func (w *Writer) getSample(sampleID string) (*model.Sample, error) {
	row := w.db.QueryRow(`
		SELECT id, subject_id, source_platform, source_post_id, slide_index,
		       file_path, sha256, phash, width, height, taken_at, fetched_at,
		       status, is_duplicate, duplicate_of, created_at, updated_at
		FROM samples
		WHERE id = ?`, sampleID)

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
		return nil, fmt.Errorf("scan sample: %w", err)
	}
	sample.TakenAt = takenAt.String
	sample.DuplicateOf = duplicateOf.String
	sample.IsDuplicate = isDup != 0
	return &sample, nil
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
