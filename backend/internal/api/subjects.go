package api

import (
	"context"
	"errors"
	"fmt"

	logrus "github.com/sirupsen/logrus"

	subjects "github.com/kmacmcfarlane/image-dataset-tool/backend/internal/api/gen/subjects"
	"github.com/kmacmcfarlane/image-dataset-tool/backend/internal/service"
)

// SubjectsService implements the Goa subjects service interface.
type SubjectsService struct {
	svc *service.SubjectService
}

// NewSubjectsService creates a new SubjectsService.
func NewSubjectsService(svc *service.SubjectService) *SubjectsService {
	return &SubjectsService{svc: svc}
}

// List returns all subjects in a project with sample counts.
func (s *SubjectsService) List(_ context.Context, p *subjects.ListPayload) (*subjects.SubjectListResult, error) {
	list, err := s.svc.List(p.ProjectID)
	if err != nil {
		if errors.Is(err, service.ErrNotFound) {
			return nil, subjects.MakeNotFound(fmt.Errorf("project %q not found", p.ProjectID))
		}
		logrus.WithError(err).Error("Failed to list subjects")
		return nil, fmt.Errorf("list subjects: %w", err)
	}

	result := &subjects.SubjectListResult{
		Subjects: make([]*subjects.SubjectResult, 0, len(list)),
	}
	for _, sub := range list {
		result.Subjects = append(result.Subjects, &subjects.SubjectResult{
			ID:          sub.ID,
			ProjectID:   sub.ProjectID,
			Slug:        sub.Slug,
			Name:        sub.Name,
			SampleCount: sub.SampleStats.Total,
			CreatedAt:   sub.CreatedAt,
			UpdatedAt:   sub.UpdatedAt,
		})
	}
	return result, nil
}

// Get returns a subject by ID with linked accounts and sample stats.
func (s *SubjectsService) Get(_ context.Context, p *subjects.GetPayload) (*subjects.SubjectDetail, error) {
	sub, err := s.svc.Get(p.ProjectID, p.ID)
	if err != nil {
		if errors.Is(err, service.ErrNotFound) {
			return nil, subjects.MakeNotFound(fmt.Errorf("subject %q not found", p.ID))
		}
		return nil, fmt.Errorf("get subject: %w", err)
	}

	linkedAccounts := make([]*subjects.SubjectLinkedAccount, 0, len(sub.LinkedAccounts))
	for _, a := range sub.LinkedAccounts {
		linkedAccounts = append(linkedAccounts, &subjects.SubjectLinkedAccount{
			ID:       a.ID,
			Platform: a.Platform,
			Handle:   a.Handle,
		})
	}

	return &subjects.SubjectDetail{
		ID:        sub.ID,
		ProjectID: sub.ProjectID,
		Slug:      sub.Slug,
		Name:      sub.Name,
		LinkedAccounts: linkedAccounts,
		SampleStats: &subjects.SubjectSampleStats{
			Pending:  sub.SampleStats.Pending,
			Kept:     sub.SampleStats.Kept,
			Rejected: sub.SampleStats.Rejected,
			Total:    sub.SampleStats.Total,
		},
		CreatedAt: sub.CreatedAt,
		UpdatedAt: sub.UpdatedAt,
	}, nil
}

// Create creates a new subject in a project.
func (s *SubjectsService) Create(_ context.Context, payload *subjects.CreateSubjectPayload) (*subjects.SubjectResult, error) {
	sub, err := s.svc.Create(payload.ProjectID, payload.Name)
	if err != nil {
		if errors.Is(err, service.ErrNotFound) {
			return nil, subjects.MakeNotFound(fmt.Errorf("project %q not found", payload.ProjectID))
		}
		if errors.Is(err, service.ErrConflict) {
			return nil, subjects.MakeConflict(fmt.Errorf("a subject with this slug already exists in the project"))
		}
		if errors.Is(err, service.ErrBadRequest) {
			return nil, subjects.MakeBadRequest(err)
		}
		return nil, fmt.Errorf("create subject: %w", err)
	}

	return &subjects.SubjectResult{
		ID:          sub.ID,
		ProjectID:   sub.ProjectID,
		Slug:        sub.Slug,
		Name:        sub.Name,
		SampleCount: sub.SampleStats.Total,
		CreatedAt:   sub.CreatedAt,
		UpdatedAt:   sub.UpdatedAt,
	}, nil
}

// Update modifies a subject's name.
func (s *SubjectsService) Update(_ context.Context, payload *subjects.UpdateSubjectPayload) (*subjects.SubjectResult, error) {
	sub, err := s.svc.Update(payload.ProjectID, payload.ID, payload.Name)
	if err != nil {
		if errors.Is(err, service.ErrNotFound) {
			return nil, subjects.MakeNotFound(fmt.Errorf("subject %q not found", payload.ID))
		}
		if errors.Is(err, service.ErrConflict) {
			return nil, subjects.MakeConflict(fmt.Errorf("a subject with this slug already exists in the project"))
		}
		if errors.Is(err, service.ErrBadRequest) {
			return nil, subjects.MakeBadRequest(err)
		}
		return nil, fmt.Errorf("update subject: %w", err)
	}

	return &subjects.SubjectResult{
		ID:          sub.ID,
		ProjectID:   sub.ProjectID,
		Slug:        sub.Slug,
		Name:        sub.Name,
		SampleCount: sub.SampleStats.Total,
		CreatedAt:   sub.CreatedAt,
		UpdatedAt:   sub.UpdatedAt,
	}, nil
}

// Delete removes a subject.
func (s *SubjectsService) Delete(_ context.Context, payload *subjects.DeletePayload) error {
	err := s.svc.Delete(payload.ProjectID, payload.ID, payload.HardDelete)
	if err != nil {
		if errors.Is(err, service.ErrNotFound) {
			return subjects.MakeNotFound(fmt.Errorf("subject %q not found", payload.ID))
		}
		return fmt.Errorf("delete subject: %w", err)
	}
	return nil
}

// LinkAccount links an existing account to a subject.
func (s *SubjectsService) LinkAccount(_ context.Context, payload *subjects.LinkAccountPayload) error {
	err := s.svc.LinkAccount(payload.ProjectID, payload.ID, payload.AccountID)
	if err != nil {
		if errors.Is(err, service.ErrNotFound) {
			return subjects.MakeNotFound(fmt.Errorf("subject or account not found"))
		}
		if errors.Is(err, service.ErrConflict) {
			return subjects.MakeConflict(fmt.Errorf("account already linked to this subject"))
		}
		logrus.WithError(err).Error("Failed to link account to subject")
		return fmt.Errorf("link account: %w", err)
	}
	return nil
}

// ListLinkedAccounts returns accounts linked to a subject.
func (s *SubjectsService) ListLinkedAccounts(_ context.Context, payload *subjects.ListLinkedAccountsPayload) (*subjects.SubjectLinkedAccountListResult, error) {
	accounts, err := s.svc.ListLinkedAccounts(payload.ProjectID, payload.ID)
	if err != nil {
		if errors.Is(err, service.ErrNotFound) {
			return nil, subjects.MakeNotFound(fmt.Errorf("subject %q not found", payload.ID))
		}
		logrus.WithError(err).Error("Failed to list linked accounts")
		return nil, fmt.Errorf("list linked accounts: %w", err)
	}

	result := &subjects.SubjectLinkedAccountListResult{
		Accounts: make([]*subjects.SubjectLinkedAccount, 0, len(accounts)),
	}
	for _, a := range accounts {
		result.Accounts = append(result.Accounts, &subjects.SubjectLinkedAccount{
			ID:       a.ID,
			Platform: a.Platform,
			Handle:   a.Handle,
		})
	}
	return result, nil
}

// UnlinkAccount removes the link between a subject and an account.
func (s *SubjectsService) UnlinkAccount(_ context.Context, payload *subjects.UnlinkAccountPayload) error {
	err := s.svc.UnlinkAccount(payload.ProjectID, payload.ID, payload.AccountID)
	if err != nil {
		if errors.Is(err, service.ErrNotFound) {
			return subjects.MakeNotFound(fmt.Errorf("link not found"))
		}
		logrus.WithError(err).Error("Failed to unlink account from subject")
		return fmt.Errorf("unlink account: %w", err)
	}
	return nil
}
