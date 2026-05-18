package store

import (
	"database/sql"
	"fmt"
	"strings"

	"github.com/kmacmcfarlane/image-dataset-tool/backend/internal/model"
)

// JobStore handles job_runs and job_messages persistence.
type JobStore struct {
	db *sql.DB
}

// NewJobStore creates a new JobStore.
func NewJobStore(db *sql.DB) *JobStore {
	return &JobStore{db: db}
}

// JobListFilter holds optional filters for ListJobs.
type JobListFilter struct {
	Type   string
	Status string
	Offset int
	Limit  int
}

// ListJobs returns a paginated list of job runs, newest first.
func (s *JobStore) ListJobs(f JobListFilter) ([]*model.JobRun, int64, error) {
	whereClauses := []string{}
	args := []any{}

	if f.Type != "" {
		whereClauses = append(whereClauses, "type = ?")
		args = append(args, f.Type)
	}
	if f.Status != "" {
		whereClauses = append(whereClauses, "status = ?")
		args = append(args, f.Status)
	}

	where := ""
	if len(whereClauses) > 0 {
		where = "WHERE " + strings.Join(whereClauses, " AND ")
	}

	// Count.
	var total int64
	countArgs := append([]any{}, args...)
	if err := s.db.QueryRow(
		fmt.Sprintf("SELECT COUNT(*) FROM job_runs %s", where),
		countArgs...,
	).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count job_runs: %w", err)
	}

	// Page.
	limit := f.Limit
	if limit <= 0 {
		limit = 20
	}
	offset := max(f.Offset, 0)

	pageArgs := append(args, limit, offset)
	rows, err := s.db.Query(
		fmt.Sprintf(`SELECT id, type, COALESCE(subject_id,''), COALESCE(study_id,''),
			status, total_items, completed_items, failed_items,
			started_at, COALESCE(finished_at,''), created_at,
			COALESCE(output_path,''), sample_count
		FROM job_runs %s
		ORDER BY created_at DESC
		LIMIT ? OFFSET ?`, where),
		pageArgs...,
	)
	if err != nil {
		return nil, 0, fmt.Errorf("list job_runs: %w", err)
	}
	defer rows.Close()

	var jobs []*model.JobRun
	for rows.Next() {
		jr := &model.JobRun{}
		var totalItems sql.NullInt64
		var sampleCount sql.NullInt64
		if err := rows.Scan(
			&jr.ID, &jr.Type, &jr.SubjectID, &jr.StudyID,
			&jr.Status, &totalItems, &jr.CompletedItems, &jr.FailedItems,
			&jr.StartedAt, &jr.FinishedAt, &jr.CreatedAt,
			&jr.OutputPath, &sampleCount,
		); err != nil {
			return nil, 0, fmt.Errorf("scan job_run: %w", err)
		}
		if totalItems.Valid {
			v := int(totalItems.Int64)
			jr.TotalItems = &v
		}
		if sampleCount.Valid {
			v := int(sampleCount.Int64)
			jr.SampleCount = &v
		}
		jobs = append(jobs, jr)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate job_runs: %w", err)
	}
	return jobs, total, nil
}

// GetJob returns a single job run by ID.
func (s *JobStore) GetJob(id string) (*model.JobRun, error) {
	row := s.db.QueryRow(`
		SELECT id, type, COALESCE(subject_id,''), COALESCE(study_id,''),
			status, total_items, completed_items, failed_items,
			started_at, COALESCE(finished_at,''), created_at,
			COALESCE(output_path,''), sample_count
		FROM job_runs WHERE id = ?`, id)

	jr := &model.JobRun{}
	var totalItems sql.NullInt64
	var sampleCount sql.NullInt64
	if err := row.Scan(
		&jr.ID, &jr.Type, &jr.SubjectID, &jr.StudyID,
		&jr.Status, &totalItems, &jr.CompletedItems, &jr.FailedItems,
		&jr.StartedAt, &jr.FinishedAt, &jr.CreatedAt,
		&jr.OutputPath, &sampleCount,
	); err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("job %q: %w", id, ErrNotFound)
		}
		return nil, fmt.Errorf("get job_run: %w", err)
	}
	if totalItems.Valid {
		v := int(totalItems.Int64)
		jr.TotalItems = &v
	}
	if sampleCount.Valid {
		v := int(sampleCount.Int64)
		jr.SampleCount = &v
	}
	return jr, nil
}

// CreateJobRun inserts a new job_run row.
func (s *JobStore) CreateJobRun(jr *model.JobRun) error {
	_, err := s.db.Exec(`
		INSERT INTO job_runs (id, type, subject_id, study_id, status,
			total_items, completed_items, failed_items, trace_id,
			pagination_exhausted, output_path, started_at, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, 0, 0, ?, 0, ?, ?, ?, ?)`,
		jr.ID, jr.Type,
		nullIfEmpty(jr.SubjectID), nullIfEmpty(jr.StudyID),
		jr.Status, jr.TotalItems, jr.TraceID,
		nullIfEmpty(jr.OutputPath),
		jr.StartedAt, jr.CreatedAt, jr.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("insert job_run: %w", err)
	}
	return nil
}

// UpdateJobRunExportResult sets the output_path and sample_count on a completed export job.
func (s *JobStore) UpdateJobRunExportResult(jobID, outputPath string, sampleCount int) error {
	_, err := s.db.Exec(`
		UPDATE job_runs SET output_path = ?, sample_count = ?, updated_at = datetime('now')
		WHERE id = ?`,
		outputPath, sampleCount, jobID,
	)
	if err != nil {
		return fmt.Errorf("update export result for job %s: %w", jobID, err)
	}
	return nil
}

// nullIfEmpty returns nil if s is empty, otherwise s.
func nullIfEmpty(s string) interface{} {
	if s == "" {
		return nil
	}
	return s
}

// JobMessageFilter holds optional filters for ListJobMessages.
type JobMessageFilter struct {
	JobRunID string
	Level    string // if set, return only messages at this level or above
	Offset   int
	Limit    int
}

// levelOrder maps level name to integer for >=  comparisons.
// error=3 > warning=2 > info=1
var levelOrder = map[string]int{
	"info":    1,
	"warning": 2,
	"error":   3,
}

// ListJobMessages returns paginated messages for a job run.
// The level filter returns messages at the given level and above (e.g. "warning" → warning + error).
func (s *JobStore) ListJobMessages(f JobMessageFilter) ([]*model.JobMessage, int64, error) {
	// Verify job exists.
	var exists int
	if err := s.db.QueryRow("SELECT COUNT(*) FROM job_runs WHERE id = ?", f.JobRunID).Scan(&exists); err != nil {
		return nil, 0, fmt.Errorf("check job_run: %w", err)
	}
	if exists == 0 {
		return nil, 0, fmt.Errorf("job %q: %w", f.JobRunID, ErrNotFound)
	}

	whereClauses := []string{"job_run_id = ?"}
	args := []any{f.JobRunID}

	if f.Level != "" {
		minOrd, ok := levelOrder[f.Level]
		if !ok {
			minOrd = levelOrder["warning"]
		}
		levels := []string{}
		for lvl, ord := range levelOrder {
			if ord >= minOrd {
				levels = append(levels, "'"+lvl+"'")
			}
		}
		whereClauses = append(whereClauses, fmt.Sprintf("level IN (%s)", strings.Join(levels, ",")))
	}

	where := "WHERE " + strings.Join(whereClauses, " AND ")

	var total int64
	countArgs := append([]any{}, args...)
	if err := s.db.QueryRow(
		fmt.Sprintf("SELECT COUNT(*) FROM job_messages %s", where),
		countArgs...,
	).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count job_messages: %w", err)
	}

	limit := f.Limit
	if limit <= 0 {
		limit = 50
	}
	offset := max(f.Offset, 0)

	pageArgs := append(args, limit, offset)
	rows, err := s.db.Query(
		fmt.Sprintf(`SELECT id, job_run_id, level,
			COALESCE(sample_ref,''), COALESCE(account_handle,''),
			message, created_at
		FROM job_messages %s
		ORDER BY created_at ASC
		LIMIT ? OFFSET ?`, where),
		pageArgs...,
	)
	if err != nil {
		return nil, 0, fmt.Errorf("list job_messages: %w", err)
	}
	defer rows.Close()

	var msgs []*model.JobMessage
	for rows.Next() {
		m := &model.JobMessage{}
		if err := rows.Scan(
			&m.ID, &m.JobRunID, &m.Level,
			&m.SampleRef, &m.AccountHandle,
			&m.Message, &m.CreatedAt,
		); err != nil {
			return nil, 0, fmt.Errorf("scan job_message: %w", err)
		}
		msgs = append(msgs, m)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate job_messages: %w", err)
	}
	return msgs, total, nil
}
