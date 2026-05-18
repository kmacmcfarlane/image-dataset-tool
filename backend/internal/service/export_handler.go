package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/kmacmcfarlane/image-dataset-tool/backend/internal/export"
	"github.com/kmacmcfarlane/image-dataset-tool/backend/internal/model"
	"github.com/kmacmcfarlane/image-dataset-tool/backend/internal/pipeline"
)

// ExportHandler is a pipeline.Handler for the media.export consumer.
// Each message represents one sample to export.
type ExportHandler struct {
	writer export.DatasetWriter
	store  ExportJobStore
	logger *slog.Logger
}

// NewExportHandler creates a handler for the media.export consumer.
func NewExportHandler(writer export.DatasetWriter, store ExportJobStore) *ExportHandler {
	return &ExportHandler{
		writer: writer,
		store:  store,
		logger: slog.With("component", "export_handler"),
	}
}

// Handle processes a single export message. It deserializes the ExportPayload,
// calls the DatasetWriter for that single sample, and updates the job on completion.
func (h *ExportHandler) Handle(ctx context.Context, env *pipeline.Envelope) error {
	var payload ExportPayload
	if err := json.Unmarshal(env.Payload, &payload); err != nil {
		return fmt.Errorf("unmarshal export payload: %w", err)
	}

	h.logger.Debug("processing export sample",
		"job_id", env.JobID,
		"sample_id", payload.SampleID,
		"format", payload.Format,
		"output_dir", payload.OutputDir,
	)

	// Build ExportOptions scoped to the single sample's subject.
	opts := model.ExportOptions{
		ProjectSlug: payload.ProjectSlug,
		SubjectSlug: payload.SubjectSlug,
		StudySlug:   payload.StudySlug,
		Format:      payload.Format,
		OutputDir:   payload.OutputDir,
	}

	// The kohya writer exports all kept samples for a subject, but we need
	// per-sample export. Use WriteSingle if the writer supports it, otherwise
	// fall back to full write.
	if sw, ok := h.writer.(SingleSampleWriter); ok {
		if err := sw.WriteSingle(ctx, opts, payload.SampleID); err != nil {
			h.logger.Error("export sample failed",
				"job_id", env.JobID,
				"sample_id", payload.SampleID,
				"error", err,
			)
			return fmt.Errorf("export sample %s: %w", payload.SampleID, err)
		}
	} else {
		return fmt.Errorf("writer does not support single-sample export")
	}

	h.logger.Info("export sample completed",
		"job_id", env.JobID,
		"sample_id", payload.SampleID,
	)

	return nil
}

// SingleSampleWriter is an optional interface that DatasetWriters can implement
// to support per-sample export (for use in the NATS pipeline).
type SingleSampleWriter interface {
	// WriteSingle exports a single sample to the output directory.
	WriteSingle(ctx context.Context, opts model.ExportOptions, sampleID string) error
}
