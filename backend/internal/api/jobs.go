package api

import (
	"context"
	"errors"
	"fmt"

	logrus "github.com/sirupsen/logrus"

	jobs "github.com/kmacmcfarlane/image-dataset-tool/backend/internal/api/gen/jobs"
	"github.com/kmacmcfarlane/image-dataset-tool/backend/internal/model"
	"github.com/kmacmcfarlane/image-dataset-tool/backend/internal/service"
)

// JobsService implements the Goa jobs service interface.
type JobsService struct {
	svc *service.JobService
}

// NewJobsService creates a new JobsService.
func NewJobsService(svc *service.JobService) *JobsService {
	return &JobsService{svc: svc}
}

// List returns a paginated list of job runs.
func (s *JobsService) List(_ context.Context, p *jobs.JobListPayload) (*jobs.JobListResult, error) {
	params := service.ListJobsParams{
		Offset: p.Offset,
		Limit:  p.Limit,
	}
	if p.Type != nil {
		params.Type = *p.Type
	}
	if p.Status != nil {
		params.Status = *p.Status
	}

	list, total, err := s.svc.ListJobs(params)
	if err != nil {
		logrus.WithError(err).Error("Failed to list jobs")
		return nil, fmt.Errorf("list jobs: %w", err)
	}

	result := &jobs.JobListResult{
		Jobs:  make([]*jobs.JobResult, 0, len(list)),
		Total: total,
	}
	for _, jr := range list {
		result.Jobs = append(result.Jobs, jobRunToJobResult(jr))
	}
	return result, nil
}

// Get returns a single job run by ID.
func (s *JobsService) Get(_ context.Context, p *jobs.GetPayload) (*jobs.JobDetail, error) {
	jr, err := s.svc.GetJob(p.ID)
	if err != nil {
		if errors.Is(err, service.ErrNotFound) {
			return nil, jobs.MakeNotFound(fmt.Errorf("job %q not found", p.ID))
		}
		logrus.WithError(err).Error("Failed to get job")
		return nil, fmt.Errorf("get job: %w", err)
	}

	detail := &jobs.JobDetail{
		ID:             jr.ID,
		Type:           jr.Type,
		Status:         jr.Status,
		TotalItems:     jr.TotalItems,
		CompletedItems: jr.CompletedItems,
		FailedItems:    jr.FailedItems,
		StartedAt:      jr.StartedAt,
		CreatedAt:      jr.CreatedAt,
	}
	if jr.SubjectID != "" {
		v := jr.SubjectID
		detail.SubjectID = &v
	}
	if jr.StudyID != "" {
		v := jr.StudyID
		detail.StudyID = &v
	}
	if jr.FinishedAt != "" {
		v := jr.FinishedAt
		detail.FinishedAt = &v
	}
	return detail, nil
}

// ListMessages returns paginated messages for a job run.
func (s *JobsService) ListMessages(_ context.Context, p *jobs.JobMessageListPayload) (*jobs.JobMessageListResult, error) {
	msgs, total, err := s.svc.ListJobMessages(service.ListJobMessagesParams{
		JobRunID: p.ID,
		Level:    p.Level,
		Offset:   p.Offset,
		Limit:    p.Limit,
	})
	if err != nil {
		if errors.Is(err, service.ErrNotFound) {
			return nil, jobs.MakeNotFound(fmt.Errorf("job %q not found", p.ID))
		}
		logrus.WithError(err).Error("Failed to list job messages")
		return nil, fmt.Errorf("list job messages: %w", err)
	}

	result := &jobs.JobMessageListResult{
		Messages: make([]*jobs.JobMessage, 0, len(msgs)),
		Total:    total,
	}
	for _, m := range msgs {
		result.Messages = append(result.Messages, jobMessageToGenType(m))
	}
	return result, nil
}

// jobRunToJobResult maps a model.JobRun to the Goa-generated JobResult type.
func jobRunToJobResult(jr *model.JobRun) *jobs.JobResult {
	r := &jobs.JobResult{
		ID:             jr.ID,
		Type:           jr.Type,
		Status:         jr.Status,
		TotalItems:     jr.TotalItems,
		CompletedItems: jr.CompletedItems,
		FailedItems:    jr.FailedItems,
		StartedAt:      jr.StartedAt,
		CreatedAt:      jr.CreatedAt,
	}
	if jr.SubjectID != "" {
		v := jr.SubjectID
		r.SubjectID = &v
	}
	if jr.StudyID != "" {
		v := jr.StudyID
		r.StudyID = &v
	}
	if jr.FinishedAt != "" {
		v := jr.FinishedAt
		r.FinishedAt = &v
	}
	return r
}

// jobMessageToGenType maps a model.JobMessage to the Goa-generated JobMessage type.
func jobMessageToGenType(m *model.JobMessage) *jobs.JobMessage {
	jm := &jobs.JobMessage{
		ID:        m.ID,
		JobRunID:  m.JobRunID,
		Level:     m.Level,
		Message:   m.Message,
		CreatedAt: m.CreatedAt,
	}
	if m.SampleRef != "" {
		v := m.SampleRef
		jm.SampleRef = &v
	}
	if m.AccountHandle != "" {
		v := m.AccountHandle
		jm.AccountHandle = &v
	}
	return jm
}
