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
	offset := f.Offset
	if offset < 0 {
		offset = 0
	}

	pageArgs := append(args, limit, offset)
	rows, err := s.db.Query(
		fmt.Sprintf(`SELECT id, type, COALESCE(subject_id,''), COALESCE(study_id,''),
			status, total_items, completed_items, failed_items,
			started_at, COALESCE(finished_at,''), created_at
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
		if err := rows.Scan(
			&jr.ID, &jr.Type, &jr.SubjectID, &jr.StudyID,
			&jr.Status, &totalItems, &jr.CompletedItems, &jr.FailedItems,
			&jr.StartedAt, &jr.FinishedAt, &jr.CreatedAt,
		); err != nil {
			return nil, 0, fmt.Errorf("scan job_run: %w", err)
		}
		if totalItems.Valid {
			v := int(totalItems.Int64)
			jr.TotalItems = &v
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
			started_at, COALESCE(finished_at,''), created_at
		FROM job_runs WHERE id = ?`, id)

	jr := &model.JobRun{}
	var totalItems sql.NullInt64
	if err := row.Scan(
		&jr.ID, &jr.Type, &jr.SubjectID, &jr.StudyID,
		&jr.Status, &totalItems, &jr.CompletedItems, &jr.FailedItems,
		&jr.StartedAt, &jr.FinishedAt, &jr.CreatedAt,
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
	return jr, nil
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
	offset := f.Offset
	if offset < 0 {
		offset = 0
	}

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
