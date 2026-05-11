package store

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/kmacmcfarlane/image-dataset-tool/backend/internal/model"
)

// SubjectStore handles subject persistence in SQLite.
type SubjectStore struct {
	db *sql.DB
}

// NewSubjectStore creates a new SubjectStore.
func NewSubjectStore(db *sql.DB) *SubjectStore {
	return &SubjectStore{db: db}
}

// Create inserts a new subject row.
func (s *SubjectStore) Create(sub *model.Subject) error {
	now := time.Now().UTC().Format(time.RFC3339)
	sub.CreatedAt = now
	sub.UpdatedAt = now

	_, err := s.db.Exec(
		`INSERT INTO subjects (id, project_id, slug, name, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		sub.ID, sub.ProjectID, sub.Slug, sub.Name, sub.CreatedAt, sub.UpdatedAt,
	)
	if err != nil {
		if isDuplicateConstraint(err) {
			return fmt.Errorf("slug %q already exists in project: %w", sub.Slug, ErrDuplicateSlug)
		}
		return fmt.Errorf("insert subject: %w", err)
	}
	return nil
}

// Get retrieves a subject by ID with linked accounts and sample stats.
func (s *SubjectStore) Get(id string) (*model.Subject, error) {
	row := s.db.QueryRow(`
		SELECT s.id, s.project_id, s.slug, s.name, s.created_at, s.updated_at
		FROM subjects s
		WHERE s.id = ?`, id)

	var sub model.Subject
	err := row.Scan(&sub.ID, &sub.ProjectID, &sub.Slug, &sub.Name, &sub.CreatedAt, &sub.UpdatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("subject %q: %w", id, ErrNotFound)
		}
		return nil, fmt.Errorf("get subject: %w", err)
	}

	// Load sample stats.
	stats, err := s.getSampleStats(id)
	if err != nil {
		return nil, fmt.Errorf("get sample stats: %w", err)
	}
	sub.SampleStats = stats

	// Load linked accounts.
	accounts, err := s.getLinkedAccounts(id)
	if err != nil {
		return nil, fmt.Errorf("get linked accounts: %w", err)
	}
	sub.LinkedAccounts = accounts

	return &sub, nil
}

// List returns all subjects for a project with sample counts.
func (s *SubjectStore) List(projectID string) ([]*model.Subject, error) {
	rows, err := s.db.Query(`
		SELECT s.id, s.project_id, s.slug, s.name, s.created_at, s.updated_at,
		       (SELECT COUNT(*) FROM samples WHERE subject_id = s.id) AS sample_count
		FROM subjects s
		WHERE s.project_id = ?
		ORDER BY s.created_at DESC`, projectID)
	if err != nil {
		return nil, fmt.Errorf("list subjects: %w", err)
	}
	defer rows.Close()

	var subjects []*model.Subject
	for rows.Next() {
		var sub model.Subject
		var sampleCount int
		if err := rows.Scan(&sub.ID, &sub.ProjectID, &sub.Slug, &sub.Name,
			&sub.CreatedAt, &sub.UpdatedAt, &sampleCount); err != nil {
			return nil, fmt.Errorf("scan subject: %w", err)
		}
		sub.SampleStats = model.SampleStats{Total: sampleCount}
		subjects = append(subjects, &sub)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate subjects: %w", err)
	}
	return subjects, nil
}

// Update modifies a subject's name and slug. Returns the updated subject.
func (s *SubjectStore) Update(id string, name string, slug string) (*model.Subject, error) {
	now := time.Now().UTC().Format(time.RFC3339)

	result, err := s.db.Exec(
		`UPDATE subjects SET name = ?, slug = ?, updated_at = ? WHERE id = ?`,
		name, slug, now, id,
	)
	if err != nil {
		if isDuplicateConstraint(err) {
			return nil, fmt.Errorf("slug %q already exists in project: %w", slug, ErrDuplicateSlug)
		}
		return nil, fmt.Errorf("update subject: %w", err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return nil, fmt.Errorf("check rows affected: %w", err)
	}
	if affected == 0 {
		return nil, fmt.Errorf("subject %q: %w", id, ErrNotFound)
	}

	return s.Get(id)
}

// Delete removes a subject by ID.
func (s *SubjectStore) Delete(id string) error {
	result, err := s.db.Exec("DELETE FROM subjects WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("delete subject: %w", err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("check rows affected: %w", err)
	}
	if affected == 0 {
		return fmt.Errorf("subject %q: %w", id, ErrNotFound)
	}
	return nil
}

// SlugExistsInProject checks if a slug is already taken within a project.
func (s *SubjectStore) SlugExistsInProject(projectID, slug string) (bool, error) {
	var count int
	err := s.db.QueryRow(
		"SELECT COUNT(*) FROM subjects WHERE project_id = ? AND slug = ?",
		projectID, slug,
	).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("check slug existence: %w", err)
	}
	return count > 0, nil
}

// getSampleStats returns sample counts by status for a subject.
func (s *SubjectStore) getSampleStats(subjectID string) (model.SampleStats, error) {
	rows, err := s.db.Query(`
		SELECT status, COUNT(*) FROM samples
		WHERE subject_id = ?
		GROUP BY status`, subjectID)
	if err != nil {
		return model.SampleStats{}, fmt.Errorf("query sample stats: %w", err)
	}
	defer rows.Close()

	var stats model.SampleStats
	for rows.Next() {
		var status string
		var count int
		if err := rows.Scan(&status, &count); err != nil {
			return model.SampleStats{}, fmt.Errorf("scan sample stats: %w", err)
		}
		switch status {
		case "pending":
			stats.Pending = count
		case "kept":
			stats.Kept = count
		case "rejected":
			stats.Rejected = count
		}
		stats.Total += count
	}
	return stats, rows.Err()
}

// getLinkedAccounts returns accounts linked to a subject.
func (s *SubjectStore) getLinkedAccounts(subjectID string) ([]model.LinkedAccount, error) {
	rows, err := s.db.Query(`
		SELECT a.id, a.platform, a.handle
		FROM accounts a
		JOIN subject_accounts sa ON sa.account_id = a.id
		WHERE sa.subject_id = ?
		ORDER BY a.handle`, subjectID)
	if err != nil {
		return nil, fmt.Errorf("query linked accounts: %w", err)
	}
	defer rows.Close()

	var accounts []model.LinkedAccount
	for rows.Next() {
		var a model.LinkedAccount
		if err := rows.Scan(&a.ID, &a.Platform, &a.Handle); err != nil {
			return nil, fmt.Errorf("scan linked account: %w", err)
		}
		accounts = append(accounts, a)
	}
	return accounts, rows.Err()
}
