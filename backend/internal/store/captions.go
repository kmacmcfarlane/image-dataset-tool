package store

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/kmacmcfarlane/image-dataset-tool/backend/internal/model"
)

// CaptionStore handles caption and caption study persistence in SQLite.
type CaptionStore struct {
	db *sql.DB
}

// NewCaptionStore creates a new CaptionStore.
func NewCaptionStore(db *sql.DB) *CaptionStore {
	return &CaptionStore{db: db}
}

// ListBySampleID returns all captions for a sample, joined with their study metadata.
func (s *CaptionStore) ListBySampleID(sampleID string) ([]model.CaptionWithStudy, error) {
	rows, err := s.db.Query(`
		SELECT c.id, c.sample_id, c.study_id, c.text, c.created_at,
		       cs.id, cs.project_id, cs.name, cs.slug, cs.provider, cs.model,
		       cs.prompt_template, cs.params_json, cs.created_at, cs.updated_at
		FROM captions c
		JOIN caption_studies cs ON cs.id = c.study_id
		WHERE c.sample_id = ?
		ORDER BY cs.slug, c.created_at DESC`, sampleID)
	if err != nil {
		return nil, fmt.Errorf("list captions by sample: %w", err)
	}
	defer rows.Close()

	var results []model.CaptionWithStudy
	for rows.Next() {
		var cws model.CaptionWithStudy
		if err := rows.Scan(
			&cws.Caption.ID, &cws.Caption.SampleID, &cws.Caption.StudyID,
			&cws.Caption.Text, &cws.Caption.CreatedAt,
			&cws.Study.ID, &cws.Study.ProjectID, &cws.Study.Name, &cws.Study.Slug,
			&cws.Study.Provider, &cws.Study.Model, &cws.Study.PromptTemplate,
			&cws.Study.ParamsJSON, &cws.Study.CreatedAt, &cws.Study.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan caption with study: %w", err)
		}
		results = append(results, cws)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate captions: %w", err)
	}
	return results, nil
}

// UpsertCaption creates or updates a caption for a sample+study pair.
// Uses the UNIQUE(sample_id, study_id) constraint for idempotent upsert.
func (s *CaptionStore) UpsertCaption(caption *model.Caption) (*model.Caption, error) {
	now := time.Now().UTC().Format(time.RFC3339)

	if caption.ID == "" {
		caption.ID = uuid.New().String()
	}

	_, err := s.db.Exec(`
		INSERT INTO captions (id, sample_id, study_id, text, created_at)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(sample_id, study_id) DO UPDATE SET
			text = excluded.text`,
		caption.ID, caption.SampleID, caption.StudyID, caption.Text, now)
	if err != nil {
		if isForeignKeyConstraint(err) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("upsert caption: %w", err)
	}

	// Read back the inserted/updated row.
	return s.GetBySampleAndStudy(caption.SampleID, caption.StudyID)
}

// GetBySampleAndStudy returns a single caption by sample+study pair.
func (s *CaptionStore) GetBySampleAndStudy(sampleID, studyID string) (*model.Caption, error) {
	row := s.db.QueryRow(`
		SELECT id, sample_id, study_id, text, created_at
		FROM captions
		WHERE sample_id = ? AND study_id = ?`, sampleID, studyID)

	var cap model.Caption
	if err := row.Scan(&cap.ID, &cap.SampleID, &cap.StudyID, &cap.Text, &cap.CreatedAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("get caption: %w", err)
	}
	return &cap, nil
}

// GetOrCreateManualStudy returns the "manual" pseudo-study for a project,
// creating it if it does not exist.
func (s *CaptionStore) GetOrCreateManualStudy(projectID string) (*model.CaptionStudy, error) {
	// Try to find existing manual study.
	row := s.db.QueryRow(`
		SELECT id, project_id, name, slug, provider, model, prompt_template, params_json, created_at, updated_at
		FROM caption_studies
		WHERE project_id = ? AND slug = ?`, projectID, model.ManualStudySlug)

	var study model.CaptionStudy
	err := row.Scan(
		&study.ID, &study.ProjectID, &study.Name, &study.Slug,
		&study.Provider, &study.Model, &study.PromptTemplate,
		&study.ParamsJSON, &study.CreatedAt, &study.UpdatedAt,
	)
	if err == nil {
		return &study, nil
	}
	if err != sql.ErrNoRows {
		return nil, fmt.Errorf("get manual study: %w", err)
	}

	// Create the manual study.
	now := time.Now().UTC().Format(time.RFC3339)
	study = model.CaptionStudy{
		ID:             uuid.New().String(),
		ProjectID:      projectID,
		Name:           "Manual",
		Slug:           model.ManualStudySlug,
		Provider:       "manual",
		Model:          "manual",
		PromptTemplate: "",
		ParamsJSON:     "{}",
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	_, err = s.db.Exec(`
		INSERT INTO caption_studies (id, project_id, name, slug, provider, model, prompt_template, params_json, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		study.ID, study.ProjectID, study.Name, study.Slug,
		study.Provider, study.Model, study.PromptTemplate,
		study.ParamsJSON, study.CreatedAt, study.UpdatedAt)
	if err != nil {
		// Race condition: another goroutine created it.
		if isDuplicateConstraint(err) {
			return s.GetOrCreateManualStudy(projectID)
		}
		return nil, fmt.Errorf("create manual study: %w", err)
	}

	return &study, nil
}

// ListStudiesByProject returns all caption studies for a project.
func (s *CaptionStore) ListStudiesByProject(projectID string) ([]model.CaptionStudy, error) {
	rows, err := s.db.Query(`
		SELECT id, project_id, name, slug, provider, model, prompt_template, params_json, created_at, updated_at
		FROM caption_studies
		WHERE project_id = ?
		ORDER BY slug`, projectID)
	if err != nil {
		return nil, fmt.Errorf("list studies: %w", err)
	}
	defer rows.Close()

	var studies []model.CaptionStudy
	for rows.Next() {
		var study model.CaptionStudy
		if err := rows.Scan(
			&study.ID, &study.ProjectID, &study.Name, &study.Slug,
			&study.Provider, &study.Model, &study.PromptTemplate,
			&study.ParamsJSON, &study.CreatedAt, &study.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan study: %w", err)
		}
		studies = append(studies, study)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate studies: %w", err)
	}
	return studies, nil
}
