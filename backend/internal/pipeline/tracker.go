package pipeline

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"time"
)

// JobStatus represents the status of a job run.
type JobStatus string

const (
	JobStatusRunning     JobStatus = "running"
	JobStatusSucceeded   JobStatus = "succeeded"
	JobStatusFailed      JobStatus = "failed"
	JobStatusCancelled   JobStatus = "cancelled"
	JobStatusInterrupted JobStatus = "interrupted"
	JobStatusPaused      JobStatus = "paused"
)

// JobTracker manages job_runs counters in the database.
// All counter updates are atomic SQL operations.
type JobTracker interface {
	// IncrCompleted atomically increments completed_items for a job.
	// Returns the new completed_items and failed_items counts.
	IncrCompleted(ctx context.Context, jobID string) (completed, failed, total int, err error)

	// IncrFailed atomically increments failed_items for a job.
	// Returns the new completed_items and failed_items counts.
	IncrFailed(ctx context.Context, jobID string) (completed, failed, total int, err error)

	// IncrTotal atomically increments total_items for a job (used as pages are discovered).
	IncrTotal(ctx context.Context, jobID string, delta int) error

	// SetPaginationExhausted marks that no more pages will be discovered.
	// The total_items value becomes final.
	SetPaginationExhausted(ctx context.Context, jobID string) error

	// CheckCompletion checks if a job is complete (completed + failed = total AND pagination exhausted).
	// If complete, sets the job status to succeeded (or failed if all items failed).
	// Returns the new status if changed, empty string if not yet complete.
	CheckCompletion(ctx context.Context, jobID string) (JobStatus, error)

	// SetStatus updates the job status and optionally sets finished_at.
	SetStatus(ctx context.Context, jobID string, status JobStatus) error

	// GetRunningJobs returns all job_runs with status='running'.
	GetRunningJobs(ctx context.Context) ([]RunningJob, error)

	// MarkInterrupted sets a job's status to 'interrupted'.
	MarkInterrupted(ctx context.Context, jobID string) error
}

// RunningJob is a summary of a job with status='running'.
type RunningJob struct {
	ID        string
	Type      string
	SubjectID sql.NullString
	TraceID   sql.NullString
}

// SQLJobTracker implements JobTracker using direct SQL on the job_runs table.
type SQLJobTracker struct {
	db *sql.DB
}

// NewJobTracker creates a new SQLJobTracker.
func NewJobTracker(db *sql.DB) *SQLJobTracker {
	return &SQLJobTracker{db: db}
}

func (t *SQLJobTracker) IncrCompleted(ctx context.Context, jobID string) (int, int, int, error) {
	var completed, failed, total int
	err := t.db.QueryRowContext(ctx, `
		UPDATE job_runs SET completed_items = completed_items + 1, updated_at = ?
		WHERE id = ?
		RETURNING completed_items, failed_items, COALESCE(total_items, 0)
	`, time.Now().UTC().Format(time.RFC3339), jobID).Scan(&completed, &failed, &total)
	if err != nil {
		return 0, 0, 0, fmt.Errorf("incr completed for job %s: %w", jobID, err)
	}
	return completed, failed, total, nil
}

func (t *SQLJobTracker) IncrFailed(ctx context.Context, jobID string) (int, int, int, error) {
	var completed, failed, total int
	err := t.db.QueryRowContext(ctx, `
		UPDATE job_runs SET failed_items = failed_items + 1, updated_at = ?
		WHERE id = ?
		RETURNING completed_items, failed_items, COALESCE(total_items, 0)
	`, time.Now().UTC().Format(time.RFC3339), jobID).Scan(&completed, &failed, &total)
	if err != nil {
		return 0, 0, 0, fmt.Errorf("incr failed for job %s: %w", jobID, err)
	}
	return completed, failed, total, nil
}

func (t *SQLJobTracker) IncrTotal(ctx context.Context, jobID string, delta int) error {
	res, err := t.db.ExecContext(ctx, `
		UPDATE job_runs SET total_items = COALESCE(total_items, 0) + ?, updated_at = ?
		WHERE id = ?
	`, delta, time.Now().UTC().Format(time.RFC3339), jobID)
	if err != nil {
		return fmt.Errorf("incr total for job %s: %w", jobID, err)
	}
	rowsAffected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("rows affected for incr total job %s: %w", jobID, err)
	}
	if rowsAffected == 0 {
		return fmt.Errorf("incr total for job %s: job not found", jobID)
	}
	return nil
}

func (t *SQLJobTracker) SetPaginationExhausted(ctx context.Context, jobID string) error {
	_, err := t.db.ExecContext(ctx, `
		UPDATE job_runs SET pagination_exhausted = 1, updated_at = ?
		WHERE id = ?
	`, time.Now().UTC().Format(time.RFC3339), jobID)
	if err != nil {
		return fmt.Errorf("set pagination exhausted for job %s: %w", jobID, err)
	}
	return nil
}

func (t *SQLJobTracker) CheckCompletion(ctx context.Context, jobID string) (JobStatus, error) {
	var completed, failed int
	var totalNull sql.NullInt64
	var paginationExhausted int
	var currentStatus string

	err := t.db.QueryRowContext(ctx, `
		SELECT status, completed_items, failed_items, total_items, pagination_exhausted
		FROM job_runs WHERE id = ?
	`, jobID).Scan(&currentStatus, &completed, &failed, &totalNull, &paginationExhausted)
	if err != nil {
		return "", fmt.Errorf("check completion for job %s: %w", jobID, err)
	}

	if currentStatus != string(JobStatusRunning) {
		return "", nil // already in terminal state
	}

	// Job with NULL total_items: need pagination exhausted AND counters match.
	// Job with set total_items: completed + failed = total.
	if !totalNull.Valid {
		return "", nil // total not yet known
	}
	total := int(totalNull.Int64)

	if completed+failed < total {
		return "", nil // not yet complete
	}

	if paginationExhausted == 0 {
		return "", nil // still paginating
	}

	// Determine final status.
	newStatus := JobStatusSucceeded
	if failed > 0 && completed == 0 {
		newStatus = JobStatusFailed
	}

	now := time.Now().UTC().Format(time.RFC3339)
	res, err := t.db.ExecContext(ctx, `
		UPDATE job_runs SET status = ?, finished_at = ?, updated_at = ?
		WHERE id = ? AND status = 'running'
	`, string(newStatus), now, now, jobID)
	if err != nil {
		return "", fmt.Errorf("set completion status for job %s: %w", jobID, err)
	}

	// If another worker already transitioned this job, skip the duplicate event.
	rowsAffected, err := res.RowsAffected()
	if err != nil {
		return "", fmt.Errorf("rows affected for job %s: %w", jobID, err)
	}
	if rowsAffected == 0 {
		return "", nil
	}

	slog.Info("Job completed",
		slog.String("job_id", jobID),
		slog.String("status", string(newStatus)),
		slog.Int("completed", completed),
		slog.Int("failed", failed),
		slog.Int("total", total),
	)

	return newStatus, nil
}

func (t *SQLJobTracker) SetStatus(ctx context.Context, jobID string, status JobStatus) error {
	now := time.Now().UTC().Format(time.RFC3339)
	query := `UPDATE job_runs SET status = ?, updated_at = ? WHERE id = ?`
	args := []interface{}{string(status), now, jobID}

	// Set finished_at for terminal statuses.
	if status == JobStatusSucceeded || status == JobStatusFailed ||
		status == JobStatusCancelled || status == JobStatusInterrupted {
		query = `UPDATE job_runs SET status = ?, finished_at = ?, updated_at = ? WHERE id = ?`
		args = []interface{}{string(status), now, now, jobID}
	}

	_, err := t.db.ExecContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("set status for job %s: %w", jobID, err)
	}
	return nil
}

func (t *SQLJobTracker) GetRunningJobs(ctx context.Context) ([]RunningJob, error) {
	rows, err := t.db.QueryContext(ctx, `
		SELECT id, type, subject_id, trace_id FROM job_runs WHERE status = 'running'
	`)
	if err != nil {
		return nil, fmt.Errorf("get running jobs: %w", err)
	}
	defer rows.Close()

	var jobs []RunningJob
	for rows.Next() {
		var j RunningJob
		if err := rows.Scan(&j.ID, &j.Type, &j.SubjectID, &j.TraceID); err != nil {
			return nil, fmt.Errorf("scan running job: %w", err)
		}
		jobs = append(jobs, j)
	}
	return jobs, rows.Err()
}

func (t *SQLJobTracker) MarkInterrupted(ctx context.Context, jobID string) error {
	return t.SetStatus(ctx, jobID, JobStatusInterrupted)
}
