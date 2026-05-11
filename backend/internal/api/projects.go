package api

import (
	"context"
	"errors"
	"fmt"

	logrus "github.com/sirupsen/logrus"

	projects "github.com/kmacmcfarlane/image-dataset-tool/backend/internal/api/gen/projects"
	"github.com/kmacmcfarlane/image-dataset-tool/backend/internal/service"
)

// ProjectsService implements the Goa projects service interface.
type ProjectsService struct {
	svc *service.ProjectService
}

// NewProjectsService creates a new ProjectsService.
func NewProjectsService(svc *service.ProjectService) *ProjectsService {
	return &ProjectsService{svc: svc}
}

// List returns all projects.
func (s *ProjectsService) List(_ context.Context) (*projects.ProjectListResult, error) {
	list, err := s.svc.List()
	if err != nil {
		logrus.WithError(err).Error("Failed to list projects")
		return nil, fmt.Errorf("list projects: %w", err)
	}

	result := &projects.ProjectListResult{
		Projects: make([]*projects.ProjectResult, 0, len(list)),
	}
	for _, p := range list {
		result.Projects = append(result.Projects, &projects.ProjectResult{
			ID:          p.ID,
			Slug:        p.Slug,
			Name:        p.Name,
			Description: p.Description,
			CreatedAt:   p.CreatedAt,
			UpdatedAt:   p.UpdatedAt,
		})
	}
	return result, nil
}

// Get returns a project by ID with subject count.
func (s *ProjectsService) Get(_ context.Context, p *projects.GetPayload) (*projects.ProjectDetail, error) {
	proj, err := s.svc.Get(p.ID)
	if err != nil {
		if errors.Is(err, service.ErrNotFound) {
			return nil, projects.MakeNotFound(fmt.Errorf("project %q not found", p.ID))
		}
		return nil, fmt.Errorf("get project: %w", err)
	}

	return &projects.ProjectDetail{
		ID:           proj.ID,
		Slug:         proj.Slug,
		Name:         proj.Name,
		Description:  proj.Description,
		SubjectCount: proj.SubjectCount,
		CreatedAt:    proj.CreatedAt,
		UpdatedAt:    proj.UpdatedAt,
	}, nil
}

// Create creates a new project.
func (s *ProjectsService) Create(_ context.Context, payload *projects.CreateProjectPayload) (*projects.ProjectResult, error) {
	proj, err := s.svc.Create(payload.Name, payload.Description)
	if err != nil {
		if errors.Is(err, service.ErrConflict) {
			return nil, projects.MakeConflict(fmt.Errorf("a project with this slug already exists"))
		}
		if errors.Is(err, service.ErrBadRequest) {
			return nil, projects.MakeBadRequest(err)
		}
		return nil, fmt.Errorf("create project: %w", err)
	}

	return &projects.ProjectResult{
		ID:          proj.ID,
		Slug:        proj.Slug,
		Name:        proj.Name,
		Description: proj.Description,
		CreatedAt:   proj.CreatedAt,
		UpdatedAt:   proj.UpdatedAt,
	}, nil
}

// Update modifies a project's name and/or description.
func (s *ProjectsService) Update(_ context.Context, payload *projects.UpdateProjectPayload) (*projects.ProjectResult, error) {
	proj, err := s.svc.Update(payload.ID, payload.Name, payload.Description)
	if err != nil {
		if errors.Is(err, service.ErrNotFound) {
			return nil, projects.MakeNotFound(fmt.Errorf("project %q not found", payload.ID))
		}
		if errors.Is(err, service.ErrBadRequest) {
			return nil, projects.MakeBadRequest(err)
		}
		return nil, fmt.Errorf("update project: %w", err)
	}

	return &projects.ProjectResult{
		ID:          proj.ID,
		Slug:        proj.Slug,
		Name:        proj.Name,
		Description: proj.Description,
		CreatedAt:   proj.CreatedAt,
		UpdatedAt:   proj.UpdatedAt,
	}, nil
}

// Delete removes a project.
func (s *ProjectsService) Delete(_ context.Context, payload *projects.DeletePayload) error {
	err := s.svc.Delete(payload.ID, payload.HardDelete)
	if err != nil {
		if errors.Is(err, service.ErrNotFound) {
			return projects.MakeNotFound(fmt.Errorf("project %q not found", payload.ID))
		}
		return fmt.Errorf("delete project: %w", err)
	}
	return nil
}
