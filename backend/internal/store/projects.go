package store

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/kmacmcfarlane/image-dataset-tool/backend/internal/model"
)

// ProjectStore handles project persistence in SQLite.
type ProjectStore struct {
	db *sql.DB
}

// NewProjectStore creates a new ProjectStore.
func NewProjectStore(db *sql.DB) *ProjectStore {
	return &ProjectStore{db: db}
}

// Create inserts a new project row. Returns an error wrapping ErrDuplicateSlug
// if the slug already exists.
func (s *ProjectStore) Create(p *model.Project) error {
	now := time.Now().UTC().Format(time.RFC3339)
	p.CreatedAt = now
	p.UpdatedAt = now

	_, err := s.db.Exec(
		`INSERT INTO projects (id, slug, name, description, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		p.ID, p.Slug, p.Name, p.Description, p.CreatedAt, p.UpdatedAt,
	)
	if err != nil {
		if isDuplicateConstraint(err) {
			return fmt.Errorf("slug %q already exists: %w", p.Slug, ErrDuplicateSlug)
		}
		return fmt.Errorf("insert project: %w", err)
	}
	return nil
}

// Get retrieves a project by ID, including subject count.
func (s *ProjectStore) Get(id string) (*model.Project, error) {
	row := s.db.QueryRow(`
		SELECT p.id, p.slug, p.name, p.description, p.created_at, p.updated_at,
		       (SELECT COUNT(*) FROM subjects WHERE project_id = p.id) AS subject_count
		FROM projects p
		WHERE p.id = ?`, id)

	var p model.Project
	err := row.Scan(&p.ID, &p.Slug, &p.Name, &p.Description, &p.CreatedAt, &p.UpdatedAt, &p.SubjectCount)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("project %q: %w", id, ErrNotFound)
		}
		return nil, fmt.Errorf("get project: %w", err)
	}
	return &p, nil
}

// List returns all projects ordered by creation time.
func (s *ProjectStore) List() ([]*model.Project, error) {
	rows, err := s.db.Query(`
		SELECT id, slug, name, description, created_at, updated_at
		FROM projects
		ORDER BY created_at DESC`)
	if err != nil {
		return nil, fmt.Errorf("list projects: %w", err)
	}
	defer rows.Close()

	var projects []*model.Project
	for rows.Next() {
		var p model.Project
		if err := rows.Scan(&p.ID, &p.Slug, &p.Name, &p.Description, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan project: %w", err)
		}
		projects = append(projects, &p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate projects: %w", err)
	}
	return projects, nil
}

// Update modifies a project's name and/or description. Returns the updated project.
func (s *ProjectStore) Update(id string, name *string, description *string) (*model.Project, error) {
	now := time.Now().UTC().Format(time.RFC3339)

	// Build dynamic update.
	setClauses := "updated_at = ?"
	args := []any{now}

	if name != nil {
		setClauses += ", name = ?"
		args = append(args, *name)
	}
	if description != nil {
		setClauses += ", description = ?"
		args = append(args, *description)
	}
	args = append(args, id)

	result, err := s.db.Exec(
		fmt.Sprintf("UPDATE projects SET %s WHERE id = ?", setClauses),
		args...,
	)
	if err != nil {
		return nil, fmt.Errorf("update project: %w", err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return nil, fmt.Errorf("check rows affected: %w", err)
	}
	if affected == 0 {
		return nil, fmt.Errorf("project %q: %w", id, ErrNotFound)
	}

	return s.Get(id)
}

// Delete removes a project by ID. Returns ErrNotFound if the project does not exist.
func (s *ProjectStore) Delete(id string) error {
	result, err := s.db.Exec("DELETE FROM projects WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("delete project: %w", err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("check rows affected: %w", err)
	}
	if affected == 0 {
		return fmt.Errorf("project %q: %w", id, ErrNotFound)
	}
	return nil
}

// SlugExists checks if a slug is already taken.
func (s *ProjectStore) SlugExists(slug string) (bool, error) {
	var count int
	err := s.db.QueryRow("SELECT COUNT(*) FROM projects WHERE slug = ?", slug).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("check slug existence: %w", err)
	}
	return count > 0, nil
}
