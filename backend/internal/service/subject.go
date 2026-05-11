package service

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/google/uuid"
	logrus "github.com/sirupsen/logrus"

	"github.com/kmacmcfarlane/image-dataset-tool/backend/internal/fileformat"
	"github.com/kmacmcfarlane/image-dataset-tool/backend/internal/model"
	"github.com/kmacmcfarlane/image-dataset-tool/backend/internal/store"
)

// SubjectRepository defines the persistence interface used by SubjectService.
type SubjectRepository interface {
	Create(sub *model.Subject) error
	Get(id string) (*model.Subject, error)
	List(projectID string) ([]*model.Subject, error)
	Update(id string, name string, slug string) (*model.Subject, error)
	Delete(id string) error
	SlugExistsInProject(projectID, slug string) (bool, error)
	LinkAccount(subjectID, accountID string) error
	UnlinkAccount(subjectID, accountID string) error
	ListLinkedAccounts(subjectID string) ([]model.LinkedAccount, error)
}

// SubjectService handles subject business logic.
type SubjectService struct {
	store        SubjectRepository
	projectStore ProjectRepository
	dataDir      string
}

// NewSubjectService creates a new SubjectService.
func NewSubjectService(store SubjectRepository, projectStore ProjectRepository, dataDir string) *SubjectService {
	return &SubjectService{store: store, projectStore: projectStore, dataDir: dataDir}
}

// Create creates a new subject with manifest-first write ordering (PRD §11.1).
func (s *SubjectService) Create(projectID, name string) (*model.Subject, error) {
	// Verify project exists.
	project, err := s.projectStore.Get(projectID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, fmt.Errorf("project %q: %w", projectID, ErrNotFound)
		}
		return nil, fmt.Errorf("get project: %w", err)
	}

	slug := model.GenerateSlug(name)
	if slug == "" {
		return nil, fmt.Errorf("cannot generate slug from name %q: %w", name, ErrBadRequest)
	}

	// Check slug uniqueness within project.
	exists, err := s.store.SlugExistsInProject(projectID, slug)
	if err != nil {
		return nil, fmt.Errorf("check slug: %w", err)
	}
	if exists {
		return nil, fmt.Errorf("slug %q already exists in project: %w", slug, ErrConflict)
	}

	id := uuid.New().String()
	sub := &model.Subject{
		ID:        id,
		ProjectID: projectID,
		Slug:      slug,
		Name:      name,
	}

	// Create subject directory structure.
	subjectDir := filepath.Join(s.dataDir, "projects", project.Slug, "subjects", slug)
	if err := os.MkdirAll(subjectDir, 0755); err != nil {
		return nil, fmt.Errorf("create subject directory: %w", err)
	}

	// Create samples subdirectory.
	samplesDir := filepath.Join(subjectDir, "samples")
	if err := os.MkdirAll(samplesDir, 0755); err != nil {
		return nil, fmt.Errorf("create samples directory: %w", err)
	}

	// Write manifest first (atomic rename), then DB insert (PRD §11.1).
	manifest := &fileformat.SubjectManifest{
		ID:             sub.ID,
		ProjectID:      sub.ProjectID,
		Slug:           sub.Slug,
		Name:           sub.Name,
		LinkedAccounts: []string{},
	}

	manifestPath := filepath.Join(subjectDir, "manifest.json")
	if err := fileformat.WriteSubjectManifest(manifestPath, manifest); err != nil {
		os.RemoveAll(subjectDir)
		return nil, fmt.Errorf("write manifest: %w", err)
	}

	// DB insert after manifest.
	if err := s.store.Create(sub); err != nil {
		logrus.WithError(err).WithField("slug", slug).Warn("DB insert failed after manifest write; reconciler will recover")
		return nil, fmt.Errorf("store subject: %w", err)
	}

	// Update manifest timestamps from DB.
	manifest.CreatedAt = sub.CreatedAt
	manifest.UpdatedAt = sub.UpdatedAt
	if err := fileformat.WriteSubjectManifest(manifestPath, manifest); err != nil {
		logrus.WithError(err).Warn("Failed to update manifest with timestamps; non-fatal")
	}

	logrus.WithFields(logrus.Fields{
		"id":         sub.ID,
		"slug":       sub.Slug,
		"project_id": projectID,
	}).Info("Subject created")

	return sub, nil
}

// Get retrieves a subject by ID with linked accounts and sample stats.
func (s *SubjectService) Get(projectID, id string) (*model.Subject, error) {
	sub, err := s.store.Get(id)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, fmt.Errorf("subject %q: %w", id, ErrNotFound)
		}
		return nil, err
	}

	// Verify subject belongs to the specified project.
	if sub.ProjectID != projectID {
		return nil, fmt.Errorf("subject %q: %w", id, ErrNotFound)
	}

	return sub, nil
}

// List returns all subjects in a project.
func (s *SubjectService) List(projectID string) ([]*model.Subject, error) {
	// Verify project exists.
	_, err := s.projectStore.Get(projectID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, fmt.Errorf("project %q: %w", projectID, ErrNotFound)
		}
		return nil, fmt.Errorf("get project: %w", err)
	}

	return s.store.List(projectID)
}

// Update modifies a subject's name. If the slug changes, the directory is renamed.
func (s *SubjectService) Update(projectID, id string, name *string) (*model.Subject, error) {
	if name == nil {
		sub, err := s.store.Get(id)
		if err != nil {
			if errors.Is(err, store.ErrNotFound) {
				return nil, fmt.Errorf("subject %q: %w", id, ErrNotFound)
			}
			return nil, err
		}
		if sub.ProjectID != projectID {
			return nil, fmt.Errorf("subject %q: %w", id, ErrNotFound)
		}
		return sub, nil
	}

	// Get current subject to know old slug.
	oldSub, err := s.store.Get(id)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, fmt.Errorf("subject %q: %w", id, ErrNotFound)
		}
		return nil, err
	}
	if oldSub.ProjectID != projectID {
		return nil, fmt.Errorf("subject %q: %w", id, ErrNotFound)
	}

	newSlug := model.GenerateSlug(*name)
	if newSlug == "" {
		return nil, fmt.Errorf("cannot generate slug from name %q: %w", *name, ErrBadRequest)
	}

	// Get parent project for directory paths.
	project, err := s.projectStore.Get(projectID)
	if err != nil {
		return nil, fmt.Errorf("get project: %w", err)
	}

	// Rename directory if slug changed.
	if newSlug != oldSub.Slug {
		// Check slug uniqueness within project.
		exists, err := s.store.SlugExistsInProject(projectID, newSlug)
		if err != nil {
			return nil, fmt.Errorf("check slug: %w", err)
		}
		if exists {
			return nil, fmt.Errorf("slug %q already exists in project: %w", newSlug, ErrConflict)
		}

		oldDir := filepath.Join(s.dataDir, "projects", project.Slug, "subjects", oldSub.Slug)
		newDir := filepath.Join(s.dataDir, "projects", project.Slug, "subjects", newSlug)
		if err := os.Rename(oldDir, newDir); err != nil {
			logrus.WithError(err).WithFields(logrus.Fields{
				"old_dir": oldDir,
				"new_dir": newDir,
			}).Warn("Failed to rename subject directory")
			// Continue with DB update even if rename fails.
		}
	}

	// Update DB.
	sub, err := s.store.Update(id, *name, newSlug)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, fmt.Errorf("subject %q: %w", id, ErrNotFound)
		}
		if errors.Is(err, store.ErrDuplicateSlug) {
			return nil, fmt.Errorf("slug %q already exists in project: %w", newSlug, ErrConflict)
		}
		return nil, err
	}

	// Update manifest on disk.
	subjectDir := filepath.Join(s.dataDir, "projects", project.Slug, "subjects", sub.Slug)
	manifestPath := filepath.Join(subjectDir, "manifest.json")
	manifest := &fileformat.SubjectManifest{
		ID:             sub.ID,
		ProjectID:      sub.ProjectID,
		Slug:           sub.Slug,
		Name:           sub.Name,
		LinkedAccounts: []string{},
		CreatedAt:      sub.CreatedAt,
		UpdatedAt:      sub.UpdatedAt,
	}
	if err := fileformat.WriteSubjectManifest(manifestPath, manifest); err != nil {
		logrus.WithError(err).Warn("Failed to update manifest after subject update; non-fatal")
	}

	return sub, nil
}

// LinkAccount links an existing account to a subject.
func (s *SubjectService) LinkAccount(projectID, subjectID, accountID string) error {
	sub, err := s.store.Get(subjectID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return fmt.Errorf("subject %q: %w", subjectID, ErrNotFound)
		}
		return err
	}
	if sub.ProjectID != projectID {
		return fmt.Errorf("subject %q: %w", subjectID, ErrNotFound)
	}

	if err := s.store.LinkAccount(subjectID, accountID); err != nil {
		if errors.Is(err, store.ErrDuplicateLink) {
			return fmt.Errorf("account %q already linked: %w", accountID, ErrConflict)
		}
		if errors.Is(err, store.ErrNotFound) {
			return fmt.Errorf("account %q: %w", accountID, ErrNotFound)
		}
		return fmt.Errorf("link account: %w", err)
	}

	logrus.WithFields(logrus.Fields{
		"subject_id": subjectID,
		"account_id": accountID,
	}).Info("Account linked to subject")

	return nil
}

// UnlinkAccount removes the link between an account and a subject.
func (s *SubjectService) UnlinkAccount(projectID, subjectID, accountID string) error {
	sub, err := s.store.Get(subjectID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return fmt.Errorf("subject %q: %w", subjectID, ErrNotFound)
		}
		return err
	}
	if sub.ProjectID != projectID {
		return fmt.Errorf("subject %q: %w", subjectID, ErrNotFound)
	}

	if err := s.store.UnlinkAccount(subjectID, accountID); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return fmt.Errorf("link (subject=%q, account=%q): %w", subjectID, accountID, ErrNotFound)
		}
		return fmt.Errorf("unlink account: %w", err)
	}

	logrus.WithFields(logrus.Fields{
		"subject_id": subjectID,
		"account_id": accountID,
	}).Info("Account unlinked from subject")

	return nil
}

// ListLinkedAccounts returns accounts linked to a subject.
func (s *SubjectService) ListLinkedAccounts(projectID, subjectID string) ([]model.LinkedAccount, error) {
	sub, err := s.store.Get(subjectID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, fmt.Errorf("subject %q: %w", subjectID, ErrNotFound)
		}
		return nil, err
	}
	if sub.ProjectID != projectID {
		return nil, fmt.Errorf("subject %q: %w", subjectID, ErrNotFound)
	}

	return s.store.ListLinkedAccounts(subjectID)
}

// Delete removes a subject. If hardDelete is true, the directory is also removed.
func (s *SubjectService) Delete(projectID, id string, hardDelete bool) error {
	// Get subject to find slug for directory removal.
	sub, err := s.store.Get(id)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return fmt.Errorf("subject %q: %w", id, ErrNotFound)
		}
		return err
	}
	if sub.ProjectID != projectID {
		return fmt.Errorf("subject %q: %w", id, ErrNotFound)
	}

	// Delete from DB (cascades to samples, edits, captions, subject_accounts).
	if err := s.store.Delete(id); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return fmt.Errorf("subject %q: %w", id, ErrNotFound)
		}
		return err
	}

	if hardDelete {
		// Get project for directory path.
		project, err := s.projectStore.Get(projectID)
		if err != nil {
			logrus.WithError(err).Warn("Failed to get project for hard delete directory removal")
		} else {
			subjectDir := filepath.Join(s.dataDir, "projects", project.Slug, "subjects", sub.Slug)
			if err := os.RemoveAll(subjectDir); err != nil {
				logrus.WithError(err).WithField("dir", subjectDir).Warn("Failed to remove subject directory")
			} else {
				logrus.WithField("dir", subjectDir).Info("Subject directory removed")
			}
		}
	}

	logrus.WithFields(logrus.Fields{
		"id":          sub.ID,
		"project_id":  projectID,
		"hard_delete": hardDelete,
	}).Info("Subject deleted")

	return nil
}
