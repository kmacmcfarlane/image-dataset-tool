package api

import (
	"context"
	"errors"
	"fmt"
	"io"

	"log/slog"

	genexports "github.com/kmacmcfarlane/image-dataset-tool/backend/internal/api/gen/exports"
	"github.com/kmacmcfarlane/image-dataset-tool/backend/internal/service"
)

// ExportsService implements the Goa exports service interface.
type ExportsService struct {
	exportSvc    *service.ExportService
	zipExportSvc *service.ZipExportService
}

// NewExportsService creates a new ExportsService.
func NewExportsService(exportSvc *service.ExportService, zipExportSvc *service.ZipExportService) *ExportsService {
	return &ExportsService{
		exportSvc:    exportSvc,
		zipExportSvc: zipExportSvc,
	}
}

// Create triggers a server-side export via the NATS pipeline.
func (s *ExportsService) Create(ctx context.Context, p *genexports.CreateExportPayload) (*genexports.CreateExportResult, error) {
	params := service.ExportParams{
		SubjectIDs: p.SubjectIds,
		Format:     p.Format,
	}
	if p.StudySlug != nil {
		params.StudySlug = *p.StudySlug
	}
	if p.Destination != nil {
		params.Destination = *p.Destination
	}

	jr, err := s.exportSvc.TriggerExport(ctx, params)
	if err != nil {
		if errors.Is(err, service.ErrBadRequest) {
			return nil, genexports.MakeBadRequest(err)
		}
		if errors.Is(err, service.ErrNotFound) {
			return nil, genexports.MakeNotFound(err)
		}
		slog.Error("Failed to trigger export", "error", err)
		return nil, fmt.Errorf("trigger export: %w", err)
	}

	totalItems := 0
	if jr.TotalItems != nil {
		totalItems = *jr.TotalItems
	}

	return &genexports.CreateExportResult{
		JobID:      jr.ID,
		TraceID:    jr.TraceID,
		Status:     jr.Status,
		TotalItems: totalItems,
		OutputPath: jr.OutputPath,
	}, nil
}

// DownloadZip streams a zip archive of exported samples.
func (s *ExportsService) DownloadZip(ctx context.Context, p *genexports.DownloadZipPayload) (*genexports.DownloadZipResult, io.ReadCloser, error) {
	params := service.ZipExportParams{
		SubjectIDs: p.SubjectIds,
		Format:     p.Format,
	}
	if p.StudySlug != nil {
		params.StudySlug = *p.StudySlug
	}

	body, err := s.zipExportSvc.StreamZip(ctx, params)
	if err != nil {
		if errors.Is(err, service.ErrBadRequest) {
			return nil, nil, genexports.MakeBadRequest(err)
		}
		if errors.Is(err, service.ErrNotFound) {
			return nil, nil, genexports.MakeNotFound(err)
		}
		slog.Error("Failed to stream zip export", "error", err)
		return nil, nil, fmt.Errorf("stream zip: %w", err)
	}

	result := &genexports.DownloadZipResult{
		ContentType:        "application/zip",
		ContentDisposition: "attachment; filename=\"export.zip\"",
	}

	return result, body, nil
}
