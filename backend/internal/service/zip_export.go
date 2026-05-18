package service

import (
	"archive/zip"
	"context"
	"database/sql"
	"fmt"
	"io"
	"log/slog"

	"github.com/kmacmcfarlane/image-dataset-tool/backend/internal/export"
	"github.com/kmacmcfarlane/image-dataset-tool/backend/internal/model"
)

// ZipExportStore defines the DB operations needed by ZipExportService.
type ZipExportStore interface {
	// ResolveSubjectInfo returns project slug and subject slug for a subject ID.
	ResolveSubjectInfo(ctx context.Context, subjectID string) (projectSlug, subjectSlug, projectID string, err error)
	// ResolveStudy returns the study ID for a project ID + slug.
	ResolveStudy(ctx context.Context, projectID, studySlug string) (string, error)
	// ListKeptSamples returns all kept samples for a subject.
	ListKeptSamples(ctx context.Context, subjectID string) ([]*model.Sample, error)
	// GetEdit returns the edit for a sample, or nil if none.
	GetEdit(ctx context.Context, sampleID string) (*model.Edit, error)
	// GetCaption returns the caption text for a sample+study pair.
	GetCaption(ctx context.Context, sampleID, studyID string) (string, bool, error)
}

// ZipExportService builds a zip archive on-the-fly from kept samples.
// It writes directly to an io.Writer (via io.Pipe) so no temporary files
// are created on disk, and memory is bounded to one sample at a time.
type ZipExportService struct {
	store        ZipExportStore
	materializer *export.EditMaterializer
	logger       *slog.Logger
}

// NewZipExportService creates a new ZipExportService.
func NewZipExportService(store ZipExportStore, dataDir string) *ZipExportService {
	return &ZipExportService{
		store:        store,
		materializer: export.NewEditMaterializer(dataDir),
		logger:       slog.With("component", "zip_export_service"),
	}
}

// ZipExportParams holds parameters for a zip export.
type ZipExportParams struct {
	SubjectIDs []string
	StudySlug  string
	Format     string // "kohya-ss"
}

// subjectMeta holds resolved metadata for a subject.
type subjectMeta struct {
	subjectID   string
	projectSlug string
	subjectSlug string
	projectID   string
	studyID     string
}

// StreamZip writes a zip archive to the returned io.ReadCloser. The zip
// is built on-the-fly in a background goroutine: one sample at a time is
// materialized, written to the zip, and then its memory is released. This
// keeps memory bounded regardless of sample count.
//
// The caller must read the returned io.ReadCloser to completion (or close
// it to abort). Any error during zip creation is surfaced as an error when
// reading from the pipe.
func (s *ZipExportService) StreamZip(ctx context.Context, params ZipExportParams) (io.ReadCloser, error) {
	if len(params.SubjectIDs) == 0 {
		return nil, fmt.Errorf("at least one subject_id is required: %w", ErrBadRequest)
	}
	if params.Format == "" {
		params.Format = "kohya-ss"
	}
	if params.Format != "kohya-ss" {
		return nil, fmt.Errorf("unsupported format %q; supported: kohya-ss: %w", params.Format, ErrBadRequest)
	}

	// Resolve all subjects upfront (validates they exist).
	var subjects []subjectMeta
	for _, sid := range params.SubjectIDs {
		projSlug, subjSlug, projID, err := s.store.ResolveSubjectInfo(ctx, sid)
		if err != nil {
			if err == sql.ErrNoRows {
				return nil, fmt.Errorf("subject %q not found: %w", sid, ErrNotFound)
			}
			return nil, fmt.Errorf("resolve subject %q: %w", sid, err)
		}

		meta := subjectMeta{
			subjectID:   sid,
			projectSlug: projSlug,
			subjectSlug: subjSlug,
			projectID:   projID,
		}

		// Resolve study ID if provided.
		if params.StudySlug != "" {
			studyID, err := s.store.ResolveStudy(ctx, projID, params.StudySlug)
			if err != nil {
				if err == sql.ErrNoRows {
					return nil, fmt.Errorf("study %q not found for project %q: %w",
						params.StudySlug, projSlug, ErrNotFound)
				}
				return nil, fmt.Errorf("resolve study: %w", err)
			}
			meta.studyID = studyID
		}

		subjects = append(subjects, meta)
	}

	// Verify at least one kept sample exists.
	totalSamples := 0
	for _, sub := range subjects {
		samples, err := s.store.ListKeptSamples(ctx, sub.subjectID)
		if err != nil {
			return nil, fmt.Errorf("list kept samples for subject %q: %w", sub.subjectID, err)
		}
		totalSamples += len(samples)
	}

	if totalSamples == 0 {
		return nil, fmt.Errorf("no kept samples found for the given subjects: %w", ErrBadRequest)
	}

	s.logger.Info("Starting zip stream",
		"subjects", len(subjects),
		"total_samples", totalSamples,
		"format", params.Format,
	)

	// Create a pipe: zip writer writes to pw, Goa reads from pr.
	pr, pw := io.Pipe()

	go s.writeZip(ctx, pw, subjects, params)

	return pr, nil
}

// writeZip runs in a goroutine, writing samples into the zip archive one at a
// time. On completion (success or error), pw is closed.
func (s *ZipExportService) writeZip(ctx context.Context, pw *io.PipeWriter, subjects []subjectMeta, params ZipExportParams) {
	zw := zip.NewWriter(pw)
	var writeErr error

	defer func() {
		// Close the zip writer first to write the central directory.
		if err := zw.Close(); err != nil && writeErr == nil {
			writeErr = fmt.Errorf("close zip writer: %w", err)
		}
		if writeErr != nil {
			pw.CloseWithError(writeErr)
		} else {
			pw.Close()
		}
	}()

	sampleCount := 0
	for _, sub := range subjects {
		if ctx.Err() != nil {
			writeErr = ctx.Err()
			return
		}

		samples, err := s.store.ListKeptSamples(ctx, sub.subjectID)
		if err != nil {
			writeErr = fmt.Errorf("list kept samples: %w", err)
			return
		}

		// Determine zip path prefix: project/subject/
		prefix := sub.projectSlug + "/" + sub.subjectSlug + "/"

		for _, sample := range samples {
			if ctx.Err() != nil {
				writeErr = ctx.Err()
				return
			}

			if err := s.writeSampleToZip(ctx, zw, prefix, sample, sub.studyID); err != nil {
				s.logger.Error("Failed to write sample to zip",
					"sample_id", sample.ID,
					"error", err,
				)
				writeErr = fmt.Errorf("write sample %s: %w", sample.ID, err)
				return
			}
			sampleCount++
		}
	}

	s.logger.Info("Zip stream complete", "samples", sampleCount)
}

// writeSampleToZip materializes one sample and writes it (and optionally its
// caption) to the zip archive.
func (s *ZipExportService) writeSampleToZip(
	ctx context.Context,
	zw *zip.Writer,
	prefix string,
	sample *model.Sample,
	studyID string,
) error {
	_ = ctx // reserved for future cancellation

	// Fetch edit (may be nil).
	edit, err := s.store.GetEdit(ctx, sample.ID)
	if err != nil {
		return fmt.Errorf("get edit: %w", err)
	}

	// Materialize image bytes.
	mat, err := s.materializer.Materialize(sample, edit)
	if err != nil {
		return fmt.Errorf("materialize: %w", err)
	}

	// Write image to zip as .jpg.
	imgName := prefix + sample.ID + ".jpg"
	fw, err := zw.Create(imgName)
	if err != nil {
		return fmt.Errorf("create zip entry %s: %w", imgName, err)
	}
	if _, err := fw.Write(mat.Data); err != nil {
		return fmt.Errorf("write zip entry %s: %w", imgName, err)
	}

	// Release materialized image bytes immediately so GC can reclaim.
	mat.Data = nil

	// Write caption if study is specified and caption exists.
	if studyID != "" {
		captionText, found, err := s.store.GetCaption(ctx, sample.ID, studyID)
		if err != nil {
			return fmt.Errorf("get caption: %w", err)
		}
		if found {
			txtName := prefix + sample.ID + ".txt"
			tw, err := zw.Create(txtName)
			if err != nil {
				return fmt.Errorf("create zip entry %s: %w", txtName, err)
			}
			if _, err := tw.Write([]byte(captionText)); err != nil {
				return fmt.Errorf("write zip entry %s: %w", txtName, err)
			}
		}
	}

	return nil
}
