// Package service contains business logic.
package service

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/google/uuid"

	"github.com/kmacmcfarlane/image-dataset-tool/backend/internal/fileformat"
	"github.com/kmacmcfarlane/image-dataset-tool/backend/internal/model"
	"github.com/kmacmcfarlane/image-dataset-tool/backend/internal/store"
)

// ProjectRepository defines the persistence interface used by ProjectService.
type ProjectRepository interface {
	Create(p *model.Project) error
	Get(id string) (*model.Project, error)
	List() ([]*model.Project, error)
	Update(id string, name *string, description *string) (*model.Project, error)
	Delete(id string) error
	SlugExists(slug string) (bool, error)
}

// ProjectService handles project business logic.
type ProjectService struct {
	store   ProjectRepository
	dataDir string
}

// NewProjectService creates a new ProjectService.
func NewProjectService(store ProjectRepository, dataDir string) *ProjectService {
	return &ProjectService{store: store, dataDir: dataDir}
}

// Create creates a new project with manifest-first write ordering (PRD §11.1).
func (s *ProjectService) Create(name, description string) (*model.Project, error) {
	slug := model.GenerateSlug(name)
	if slug == "" {
		return nil, fmt.Errorf("cannot generate slug from name %q: %w", name, ErrBadRequest)
	}

	// Check slug uniqueness before filesystem write.
	exists, err := s.store.SlugExists(slug)
	if err != nil {
		return nil, fmt.Errorf("check slug: %w", err)
	}
	if exists {
		return nil, fmt.Errorf("slug %q already exists: %w", slug, ErrConflict)
	}

	id := uuid.New().String()
	p := &model.Project{
		ID:          id,
		Slug:        slug,
		Name:        name,
		Description: description,
	}

	// Create project directory.
	projectDir := filepath.Join(s.dataDir, "projects", slug)
	if err := os.MkdirAll(projectDir, 0755); err != nil {
		return nil, fmt.Errorf("create project directory: %w", err)
	}

	// Create subjects subdirectory.
	subjectsDir := filepath.Join(projectDir, "subjects")
	if err := os.MkdirAll(subjectsDir, 0755); err != nil {
		return nil, fmt.Errorf("create subjects directory: %w", err)
	}

	// Write manifest first (atomic rename), then DB insert (PRD §11.1).
	manifest := &fileformat.ProjectManifest{
		ID:          p.ID,
		Slug:        p.Slug,
		Name:        p.Name,
		Description: p.Description,
	}

	manifestPath := filepath.Join(projectDir, "manifest.json")
	if err := fileformat.WriteProjectManifest(manifestPath, manifest); err != nil {
		// Clean up directory on manifest write failure.
		os.RemoveAll(projectDir)
		return nil, fmt.Errorf("write manifest: %w", err)
	}

	// DB insert after manifest.
	if err := s.store.Create(p); err != nil {
		// Manifest is still valid; reconciler will recover on next startup.
		slog.Warn("DB insert failed after manifest write; reconciler will recover", "error", err, "slug", slug)
		return nil, fmt.Errorf("store project: %w", err)
	}

	// Update manifest timestamps from DB.
	manifest.CreatedAt = p.CreatedAt
	manifest.UpdatedAt = p.UpdatedAt
	if err := fileformat.WriteProjectManifest(manifestPath, manifest); err != nil {
		slog.Warn("Failed to update manifest with timestamps; non-fatal", "error", err)
	}

	slog.Info("Project created", "id", p.ID, "slug", p.Slug)

	return p, nil
}

// Get retrieves a project by ID with subject count.
func (s *ProjectService) Get(id string) (*model.Project, error) {
	p, err := s.store.Get(id)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, fmt.Errorf("project %q: %w", id, ErrNotFound)
		}
		return nil, err
	}
	return p, nil
}

// List returns all projects.
func (s *ProjectService) List() ([]*model.Project, error) {
	return s.store.List()
}

// Update modifies project name and/or description.
func (s *ProjectService) Update(id string, name *string, description *string) (*model.Project, error) {
	if name == nil && description == nil {
		p, err := s.store.Get(id)
		if err != nil {
			if errors.Is(err, store.ErrNotFound) {
				return nil, fmt.Errorf("project %q: %w", id, ErrNotFound)
			}
			return nil, err
		}
		return p, nil
	}

	p, err := s.store.Update(id, name, description)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, fmt.Errorf("project %q: %w", id, ErrNotFound)
		}
		return nil, err
	}

	// Update manifest on disk.
	projectDir := filepath.Join(s.dataDir, "projects", p.Slug)
	manifestPath := filepath.Join(projectDir, "manifest.json")
	manifest := &fileformat.ProjectManifest{
		ID:          p.ID,
		Slug:        p.Slug,
		Name:        p.Name,
		Description: p.Description,
		CreatedAt:   p.CreatedAt,
		UpdatedAt:   p.UpdatedAt,
	}
	if err := fileformat.WriteProjectManifest(manifestPath, manifest); err != nil {
		slog.Warn("Failed to update manifest after project update; non-fatal", "error", err)
	}

	return p, nil
}

// Delete removes a project. If hardDelete is true, the project directory is also removed.
func (s *ProjectService) Delete(id string, hardDelete bool) error {
	// Get project to find slug for directory removal.
	p, err := s.store.Get(id)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return fmt.Errorf("project %q: %w", id, ErrNotFound)
		}
		return err
	}

	// Delete from DB (cascades to subjects etc.).
	if err := s.store.Delete(id); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return fmt.Errorf("project %q: %w", id, ErrNotFound)
		}
		return err
	}

	if hardDelete {
		projectDir := filepath.Join(s.dataDir, "projects", p.Slug)
		if err := os.RemoveAll(projectDir); err != nil {
			slog.Warn("Failed to remove project directory", "error", err, "dir", projectDir)
		} else {
			slog.Info("Project directory removed", "dir", projectDir)
		}
	}

	slog.Info("Project deleted", "id", p.ID, "hard_delete", hardDelete)

	return nil
}
