package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/nats-io/nats.go/jetstream"

	"github.com/kmacmcfarlane/image-dataset-tool/backend/internal/model"
	"github.com/kmacmcfarlane/image-dataset-tool/backend/internal/natsutil"
	"github.com/kmacmcfarlane/image-dataset-tool/backend/internal/pipeline"
)

// ExportJobStore defines persistence methods used by ExportService.
type ExportJobStore interface {
	CreateJobRun(jr *model.JobRun) error
	UpdateJobRunExportResult(jobID, outputPath string, sampleCount int) error
}

// ExportParams holds parameters for triggering an export.
type ExportParams struct {
	SubjectIDs []string
	StudySlug  string
	Format     string // "kohya-ss"
	Destination string // optional override; defaults to $DATA_DIR/exports/...
}

// ExportPayload is the per-sample message payload published to media.export.
type ExportPayload struct {
	SampleID    string `json:"sample_id"`
	SubjectID   string `json:"subject_id"`
	ProjectSlug string `json:"project_slug"`
	SubjectSlug string `json:"subject_slug"`
	StudySlug   string `json:"study_slug"`
	Format      string `json:"format"`
	OutputDir   string `json:"output_dir"`
}

// ExportService handles export triggering business logic.
type ExportService struct {
	db      *sql.DB
	js      jetstream.JetStream
	store   ExportJobStore
	dataDir string
	logger  *slog.Logger
}

// NewExportService creates a new ExportService.
func NewExportService(db *sql.DB, js jetstream.JetStream, store ExportJobStore, dataDir string) *ExportService {
	return &ExportService{
		db:      db,
		js:      js,
		store:   store,
		dataDir: dataDir,
		logger:  slog.With("component", "export_service"),
	}
}

// TriggerExport validates inputs, creates a job_run, enumerates kept samples,
// and publishes one message per sample to media.export.
func (s *ExportService) TriggerExport(ctx context.Context, params ExportParams) (*model.JobRun, error) {
	if len(params.SubjectIDs) == 0 {
		return nil, fmt.Errorf("at least one subject_id is required: %w", ErrBadRequest)
	}
	if params.Format == "" {
		params.Format = "kohya-ss"
	}
	if params.Format != "kohya-ss" {
		return nil, fmt.Errorf("unsupported format %q; supported: kohya-ss: %w", params.Format, ErrBadRequest)
	}

	// Resolve subjects and collect samples.
	type subjectInfo struct {
		subjectID   string
		projectSlug string
		subjectSlug string
	}

	var subjects []subjectInfo
	for _, sid := range params.SubjectIDs {
		var projectSlug, subjectSlug string
		err := s.db.QueryRowContext(ctx, `
			SELECT p.slug, s.slug
			FROM subjects s
			JOIN projects p ON p.id = s.project_id
			WHERE s.id = ?`, sid).Scan(&projectSlug, &subjectSlug)
		if err != nil {
			if err == sql.ErrNoRows {
				return nil, fmt.Errorf("subject %q not found: %w", sid, ErrNotFound)
			}
			return nil, fmt.Errorf("resolve subject %q: %w", sid, err)
		}
		subjects = append(subjects, subjectInfo{
			subjectID:   sid,
			projectSlug: projectSlug,
			subjectSlug: subjectSlug,
		})
	}

	// Resolve study slug if provided (validate it exists for at least one subject's project).
	if params.StudySlug != "" {
		var exists int
		err := s.db.QueryRowContext(ctx, `
			SELECT COUNT(*) FROM caption_studies cs
			JOIN projects p ON p.id = cs.project_id
			JOIN subjects s ON s.project_id = p.id
			WHERE cs.slug = ? AND s.id IN (`+placeholders(len(params.SubjectIDs))+`)`,
			append([]any{params.StudySlug}, toInterfaceSlice(params.SubjectIDs)...)...,
		).Scan(&exists)
		if err != nil {
			return nil, fmt.Errorf("validate study slug: %w", err)
		}
		if exists == 0 {
			return nil, fmt.Errorf("study %q not found for any of the given subjects: %w", params.StudySlug, ErrNotFound)
		}
	}

	// Enumerate kept samples across all subjects.
	type sampleEntry struct {
		sampleID  string
		subjectInfo
	}

	var samples []sampleEntry
	for _, sub := range subjects {
		rows, err := s.db.QueryContext(ctx, `
			SELECT id FROM samples
			WHERE subject_id = ? AND status = 'kept'
			ORDER BY created_at ASC`, sub.subjectID)
		if err != nil {
			return nil, fmt.Errorf("list kept samples for subject %q: %w", sub.subjectID, err)
		}
		for rows.Next() {
			var sid string
			if err := rows.Scan(&sid); err != nil {
				rows.Close()
				return nil, fmt.Errorf("scan sample: %w", err)
			}
			samples = append(samples, sampleEntry{sampleID: sid, subjectInfo: sub})
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return nil, fmt.Errorf("iterate samples: %w", err)
		}
	}

	if len(samples) == 0 {
		return nil, fmt.Errorf("no kept samples found for the given subjects: %w", ErrBadRequest)
	}

	// Generate timestamp for output directory.
	now := time.Now().UTC()
	timestamp := now.Format("20060102-150405")

	// Create job_run.
	jobID := uuid.New().String()
	traceID := pipeline.NewTraceID()
	totalItems := len(samples)

	jr := &model.JobRun{
		ID:         jobID,
		Type:       "export",
		SubjectID:  params.SubjectIDs[0], // primary subject
		Status:     "running",
		TotalItems: &totalItems,
		TraceID:    traceID,
		StartedAt:  now.Format(time.RFC3339),
		CreatedAt:  now.Format(time.RFC3339),
	}

	if err := s.store.CreateJobRun(jr); err != nil {
		return nil, fmt.Errorf("create job run: %w", err)
	}

	// Mark pagination exhausted immediately (all samples known upfront).
	_, err := s.db.ExecContext(ctx, `
		UPDATE job_runs SET pagination_exhausted = 1, updated_at = ? WHERE id = ?`,
		now.Format(time.RFC3339), jobID)
	if err != nil {
		return nil, fmt.Errorf("set pagination exhausted: %w", err)
	}

	// Publish one message per sample.
	for _, sample := range samples {
		outputDir := params.Destination
		if outputDir == "" {
			outputDir = filepath.Join(s.dataDir, "exports",
				sample.projectSlug, sample.subjectSlug,
				params.Format+"-"+timestamp)
		}

		payload := ExportPayload{
			SampleID:    sample.sampleID,
			SubjectID:   sample.subjectID,
			ProjectSlug: sample.projectSlug,
			SubjectSlug: sample.subjectSlug,
			StudySlug:   params.StudySlug,
			Format:      params.Format,
			OutputDir:   outputDir,
		}

		payloadBytes, err := json.Marshal(payload)
		if err != nil {
			return nil, fmt.Errorf("marshal export payload: %w", err)
		}

		env := &pipeline.Envelope{
			JobID:    jobID,
			TraceID:  traceID,
			Subject:  natsutil.SubjectExport,
			SampleID: sample.sampleID,
			Payload:  payloadBytes,
		}

		envBytes, err := pipeline.MarshalEnvelope(env)
		if err != nil {
			return nil, fmt.Errorf("marshal envelope: %w", err)
		}

		if _, err := s.js.Publish(ctx, natsutil.SubjectExport, envBytes); err != nil {
			s.logger.Error("Failed to publish export message",
				"sample_id", sample.sampleID,
				"job_id", jobID,
				"error", err,
			)
			return nil, fmt.Errorf("publish export message for sample %s: %w", sample.sampleID, err)
		}
	}

	// Store the output path on the job for later retrieval.
	// Use the first sample's output dir as the canonical output path.
	outputDir := params.Destination
	if outputDir == "" && len(samples) > 0 {
		first := samples[0]
		outputDir = filepath.Join(s.dataDir, "exports",
			first.projectSlug, first.subjectSlug,
			params.Format+"-"+timestamp)
	}
	jr.OutputPath = outputDir

	if err := s.store.UpdateJobRunExportResult(jobID, outputDir, 0); err != nil {
		s.logger.Error("Failed to set initial output path", "error", err)
		// Non-fatal: the export will still run.
	}

	s.logger.Info("Export triggered",
		"job_id", jobID,
		"trace_id", traceID,
		"samples", len(samples),
		"format", params.Format,
		"output_dir", outputDir,
	)

	return jr, nil
}

// placeholders returns "?,?,?" for n items.
func placeholders(n int) string {
	if n <= 0 {
		return ""
	}
	var sb strings.Builder
	sb.WriteString("?")
	for i := 1; i < n; i++ {
		sb.WriteString(",?")
	}
	return sb.String()
}

// toInterfaceSlice converts []string to []any.
func toInterfaceSlice(ss []string) []any {
	out := make([]any, len(ss))
	for i, s := range ss {
		out[i] = s
	}
	return out
}
