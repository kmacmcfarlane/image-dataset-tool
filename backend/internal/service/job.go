package service

import (
	"errors"
	"fmt"
	"log/slog"

	"github.com/kmacmcfarlane/image-dataset-tool/backend/internal/model"
	"github.com/kmacmcfarlane/image-dataset-tool/backend/internal/store"
)

// JobRepository defines the persistence interface used by JobService.
type JobRepository interface {
	ListJobs(f store.JobListFilter) ([]*model.JobRun, int64, error)
	GetJob(id string) (*model.JobRun, error)
	ListJobMessages(f store.JobMessageFilter) ([]*model.JobMessage, int64, error)
}

// JobService handles job-history business logic.
type JobService struct {
	store JobRepository
}

// NewJobService creates a new JobService.
func NewJobService(store JobRepository) *JobService {
	return &JobService{store: store}
}

// ListJobsParams carries query parameters for listing job runs.
type ListJobsParams struct {
	Type   string
	Status string
	Offset int
	Limit  int
}

// ListJobs returns a paginated list of job runs.
func (s *JobService) ListJobs(p ListJobsParams) ([]*model.JobRun, int64, error) {
	limit := p.Limit
	if limit <= 0 {
		limit = 20
	}
	jobs, total, err := s.store.ListJobs(store.JobListFilter{
		Type:   p.Type,
		Status: p.Status,
		Offset: p.Offset,
		Limit:  limit,
	})
	if err != nil {
		slog.Error("Failed to list jobs", "error", err)
		return nil, 0, fmt.Errorf("list jobs: %w", err)
	}
	return jobs, total, nil
}

// GetJob returns a single job run by ID.
func (s *JobService) GetJob(id string) (*model.JobRun, error) {
	jr, err := s.store.GetJob(id)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, fmt.Errorf("job %q: %w", id, ErrNotFound)
		}
		return nil, fmt.Errorf("get job: %w", err)
	}
	return jr, nil
}

// ListJobMessagesParams carries query parameters for listing job messages.
type ListJobMessagesParams struct {
	JobRunID string
	Level    string
	Offset   int
	Limit    int
}

// ListJobMessages returns paginated messages for a job run.
func (s *JobService) ListJobMessages(p ListJobMessagesParams) ([]*model.JobMessage, int64, error) {
	limit := p.Limit
	if limit <= 0 {
		limit = 50
	}
	level := p.Level
	if level == "" {
		level = "warning"
	}

	msgs, total, err := s.store.ListJobMessages(store.JobMessageFilter{
		JobRunID: p.JobRunID,
		Level:    level,
		Offset:   p.Offset,
		Limit:    limit,
	})
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, 0, fmt.Errorf("job %q: %w", p.JobRunID, ErrNotFound)
		}
		slog.Error("Failed to list job messages", "error", err)
		return nil, 0, fmt.Errorf("list job messages: %w", err)
	}
	return msgs, total, nil
}
