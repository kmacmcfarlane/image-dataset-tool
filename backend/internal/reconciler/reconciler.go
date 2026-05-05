// Package reconciler implements the filesystem-as-truth reconciler that syncs
// on-disk manifests to the SQLite database on every startup.
package reconciler

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
	logrus "github.com/sirupsen/logrus"

	"github.com/kmacmcfarlane/image-dataset-tool/backend/internal/fileformat"
)

// Run performs a full reconciliation of the filesystem against the database.
// It scans the projects directory for manifests and ensures the DB matches.
// This must complete before NATS consumers start (gated startup).
//
// The reconciler is idempotent: running twice produces identical DB state.
func Run(db *sql.DB, dataDir string) error {
	log := logrus.WithField("component", "reconciler")
	log.Info("Starting filesystem reconciliation")

	projectsDir := filepath.Join(dataDir, "projects")

	// If no projects directory exists, nothing to reconcile.
	if _, err := os.Stat(projectsDir); os.IsNotExist(err) {
		log.Info("No projects directory found, nothing to reconcile")
		return nil
	}

	// Phase 1: Scan filesystem and collect all manifest data.
	diskProjects, diskSubjects, diskSamples, err := scanFilesystem(log, projectsDir)
	if err != nil {
		return fmt.Errorf("scan filesystem: %w", err)
	}

	log.WithFields(logrus.Fields{
		"projects": len(diskProjects),
		"subjects": len(diskSubjects),
		"samples":  len(diskSamples),
	}).Info("Filesystem scan complete")

	// Phase 2: Reconcile projects.
	if err := reconcileProjects(db, diskProjects); err != nil {
		return fmt.Errorf("reconcile projects: %w", err)
	}

	// Phase 3: Reconcile subjects.
	if err := reconcileSubjects(db, diskSubjects); err != nil {
		return fmt.Errorf("reconcile subjects: %w", err)
	}

	// Phase 4: Reconcile samples and captions.
	if err := reconcileSamples(db, diskSamples); err != nil {
		return fmt.Errorf("reconcile samples: %w", err)
	}

	log.Info("Filesystem reconciliation complete")
	return nil
}

// diskProject holds a project manifest read from disk.
type diskProject struct {
	manifest *fileformat.ProjectManifest
}

// diskSubject holds a subject manifest read from disk.
type diskSubject struct {
	manifest *fileformat.SubjectManifest
}

// diskSample holds a sample metadata read from disk along with its subject ID.
type diskSample struct {
	subjectID string
	metadata  *fileformat.SampleMetadata
}

// scanFilesystem walks the projects directory and collects all valid manifests.
// Malformed JSON is logged as a warning and skipped (file is NOT deleted).
func scanFilesystem(log *logrus.Entry, projectsDir string) (
	[]diskProject, []diskSubject, []diskSample, error,
) {
	var projects []diskProject
	var subjects []diskSubject
	var samples []diskSample

	// Iterate project directories.
	projectEntries, err := os.ReadDir(projectsDir)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("read projects dir: %w", err)
	}

	for _, pe := range projectEntries {
		if !pe.IsDir() {
			continue
		}

		projDir := filepath.Join(projectsDir, pe.Name())
		projManifestPath := filepath.Join(projDir, "manifest.json")

		projManifest, err := fileformat.ReadProjectManifest(projManifestPath)
		if err != nil {
			if os.IsNotExist(err) {
				log.WithField("path", projManifestPath).Warn("Project directory missing manifest.json, skipping")
				continue
			}
			log.WithFields(logrus.Fields{
				"path":  projManifestPath,
				"error": err.Error(),
			}).Warn("Malformed project manifest, skipping")
			continue
		}

		projects = append(projects, diskProject{manifest: projManifest})

		// Scan subjects within this project.
		subjectsDir := filepath.Join(projDir, "subjects")
		if _, statErr := os.Stat(subjectsDir); os.IsNotExist(statErr) {
			continue
		}

		subjectEntries, err := os.ReadDir(subjectsDir)
		if err != nil {
			log.WithFields(logrus.Fields{
				"path":  subjectsDir,
				"error": err.Error(),
			}).Warn("Cannot read subjects directory, skipping")
			continue
		}

		for _, se := range subjectEntries {
			if !se.IsDir() {
				continue
			}

			subjDir := filepath.Join(subjectsDir, se.Name())
			subjManifestPath := filepath.Join(subjDir, "manifest.json")

			subjManifest, err := fileformat.ReadSubjectManifest(subjManifestPath)
			if err != nil {
				if os.IsNotExist(err) {
					log.WithField("path", subjManifestPath).Warn("Subject directory missing manifest.json, skipping")
					continue
				}
				log.WithFields(logrus.Fields{
					"path":  subjManifestPath,
					"error": err.Error(),
				}).Warn("Malformed subject manifest, skipping")
				continue
			}

			subjects = append(subjects, diskSubject{manifest: subjManifest})

			// Scan samples within this subject.
			samplesDir := filepath.Join(subjDir, "samples")
			if _, statErr := os.Stat(samplesDir); os.IsNotExist(statErr) {
				continue
			}

			sampleEntries, err := os.ReadDir(samplesDir)
			if err != nil {
				log.WithFields(logrus.Fields{
					"path":  samplesDir,
					"error": err.Error(),
				}).Warn("Cannot read samples directory, skipping")
				continue
			}

			for _, sampleEntry := range sampleEntries {
				if sampleEntry.IsDir() {
					continue
				}
				name := sampleEntry.Name()
				// Only process .json files that are sample metadata (not thumbnails, not images).
				if !strings.HasSuffix(name, ".json") {
					continue
				}

				samplePath := filepath.Join(samplesDir, name)
				sampleMeta, err := fileformat.ReadSampleMetadata(samplePath)
				if err != nil {
					log.WithFields(logrus.Fields{
						"path":  samplePath,
						"error": err.Error(),
					}).Warn("Malformed sample metadata, skipping")
					continue
				}

				samples = append(samples, diskSample{
					subjectID: subjManifest.ID,
					metadata:  sampleMeta,
				})
			}
		}
	}

	return projects, subjects, samples, nil
}

// reconcileProjects ensures DB projects match disk manifests.
// Adds missing, updates changed, removes stale.
func reconcileProjects(db *sql.DB, diskProjects []diskProject) error {
	// Get existing DB project IDs.
	dbIDs, err := queryIDs(db, "SELECT id FROM projects")
	if err != nil {
		return err
	}

	diskIDSet := make(map[string]bool)

	for _, dp := range diskProjects {
		m := dp.manifest
		diskIDSet[m.ID] = true

		if dbIDs[m.ID] {
			// Update existing row (manifest wins).
			_, err := db.Exec(`UPDATE projects SET slug=?, name=?, description=?, created_at=?, updated_at=? WHERE id=?`,
				m.Slug, m.Name, m.Description, m.CreatedAt, m.UpdatedAt, m.ID)
			if err != nil {
				return fmt.Errorf("update project %s: %w", m.ID, err)
			}
		} else {
			// Insert new row.
			_, err := db.Exec(`INSERT INTO projects (id, slug, name, description, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?)`,
				m.ID, m.Slug, m.Name, m.Description, m.CreatedAt, m.UpdatedAt)
			if err != nil {
				return fmt.Errorf("insert project %s: %w", m.ID, err)
			}
		}
	}

	// Remove DB rows whose manifests no longer exist on disk.
	for id := range dbIDs {
		if !diskIDSet[id] {
			_, err := db.Exec("DELETE FROM projects WHERE id=?", id)
			if err != nil {
				return fmt.Errorf("delete project %s: %w", id, err)
			}
		}
	}

	return nil
}

// reconcileSubjects ensures DB subjects match disk manifests.
func reconcileSubjects(db *sql.DB, diskSubjects []diskSubject) error {
	dbIDs, err := queryIDs(db, "SELECT id FROM subjects")
	if err != nil {
		return err
	}

	diskIDSet := make(map[string]bool)

	for _, ds := range diskSubjects {
		m := ds.manifest
		diskIDSet[m.ID] = true

		if dbIDs[m.ID] {
			_, err := db.Exec(`UPDATE subjects SET project_id=?, slug=?, name=?, created_at=?, updated_at=? WHERE id=?`,
				m.ProjectID, m.Slug, m.Name, m.CreatedAt, m.UpdatedAt, m.ID)
			if err != nil {
				return fmt.Errorf("update subject %s: %w", m.ID, err)
			}
		} else {
			_, err := db.Exec(`INSERT INTO subjects (id, project_id, slug, name, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?)`,
				m.ID, m.ProjectID, m.Slug, m.Name, m.CreatedAt, m.UpdatedAt)
			if err != nil {
				return fmt.Errorf("insert subject %s: %w", m.ID, err)
			}
		}
	}

	// Remove stale subjects.
	for id := range dbIDs {
		if !diskIDSet[id] {
			_, err := db.Exec("DELETE FROM subjects WHERE id=?", id)
			if err != nil {
				return fmt.Errorf("delete subject %s: %w", id, err)
			}
		}
	}

	return nil
}

// reconcileSamples ensures DB samples and captions match disk manifests.
func reconcileSamples(db *sql.DB, diskSamples []diskSample) error {
	dbIDs, err := queryIDs(db, "SELECT id FROM samples")
	if err != nil {
		return err
	}

	diskIDSet := make(map[string]bool)

	for _, ds := range diskSamples {
		m := ds.metadata
		diskIDSet[m.ID] = true

		duplicateOfJSON := marshalStringSlice(m.DuplicateOf)
		isDup := 0
		if m.IsDuplicate {
			isDup = 1
		}

		if dbIDs[m.ID] {
			// Update existing sample row.
			_, err := db.Exec(`UPDATE samples SET subject_id=?, source_platform=?, source_post_id=?,
				slide_index=?, file_path=?, sha256=?, phash=?, width=?, height=?,
				taken_at=?, fetched_at=?, status=?, is_duplicate=?, duplicate_of=?,
				created_at=?, updated_at=?
				WHERE id=?`,
				ds.subjectID, m.Source.Platform, m.Source.PostID,
				m.Source.SlideIndex, m.File.Path, m.File.SHA256, m.File.PHash,
				m.File.Width, m.File.Height,
				nullIfEmpty(m.Source.TakenAt), m.FetchedAt, m.Status, isDup, duplicateOfJSON,
				m.CreatedAt, m.UpdatedAt,
				m.ID)
			if err != nil {
				return fmt.Errorf("update sample %s: %w", m.ID, err)
			}
		} else {
			// Insert new sample row.
			_, err := db.Exec(`INSERT INTO samples (id, subject_id, source_platform, source_post_id,
				slide_index, file_path, sha256, phash, width, height,
				taken_at, fetched_at, status, is_duplicate, duplicate_of,
				created_at, updated_at)
				VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
				m.ID, ds.subjectID, m.Source.Platform, m.Source.PostID,
				m.Source.SlideIndex, m.File.Path, m.File.SHA256, m.File.PHash,
				m.File.Width, m.File.Height,
				nullIfEmpty(m.Source.TakenAt), m.FetchedAt, m.Status, isDup, duplicateOfJSON,
				m.CreatedAt, m.UpdatedAt)
			if err != nil {
				return fmt.Errorf("insert sample %s: %w", m.ID, err)
			}
		}

		// Reconcile captions for this sample.
		if err := reconcileCaptions(db, m.ID, m.Captions); err != nil {
			return fmt.Errorf("reconcile captions for sample %s: %w", m.ID, err)
		}
	}

	// Remove stale samples.
	for id := range dbIDs {
		if !diskIDSet[id] {
			_, err := db.Exec("DELETE FROM samples WHERE id=?", id)
			if err != nil {
				return fmt.Errorf("delete sample %s: %w", id, err)
			}
		}
	}

	return nil
}

// reconcileCaptions syncs captions from a sample's manifest into the captions table.
// Captions are keyed by (sample_id, study_id) with UNIQUE constraint.
func reconcileCaptions(db *sql.DB, sampleID string, captions map[string]fileformat.Caption) error {
	if len(captions) == 0 {
		// Remove any existing captions for this sample that are no longer in manifest.
		_, err := db.Exec("DELETE FROM captions WHERE sample_id=?", sampleID)
		return err
	}

	// Get existing caption study_ids for this sample.
	rows, err := db.Query("SELECT id, study_id FROM captions WHERE sample_id=?", sampleID)
	if err != nil {
		return err
	}
	defer rows.Close()

	type existingCaption struct {
		id      string
		studyID string
	}
	var existing []existingCaption
	for rows.Next() {
		var ec existingCaption
		if err := rows.Scan(&ec.id, &ec.studyID); err != nil {
			return err
		}
		existing = append(existing, ec)
	}
	if err := rows.Err(); err != nil {
		return err
	}

	// Build set of study IDs from disk.
	diskStudyIDs := make(map[string]fileformat.Caption)
	for _, cap := range captions {
		diskStudyIDs[cap.StudyID] = cap
	}

	// Build set of existing study IDs.
	existingByStudy := make(map[string]string) // study_id -> caption id
	for _, ec := range existing {
		existingByStudy[ec.studyID] = ec.id
	}

	// Upsert captions from disk.
	for _, cap := range captions {
		if capID, exists := existingByStudy[cap.StudyID]; exists {
			// Update existing caption.
			_, err := db.Exec(`UPDATE captions SET text=?, created_at=? WHERE id=?`,
				cap.Text, cap.CreatedAt, capID)
			if err != nil {
				return fmt.Errorf("update caption for study %s: %w", cap.StudyID, err)
			}
			delete(existingByStudy, cap.StudyID)
		} else {
			// Insert new caption. Need to ensure study exists first — if not, skip.
			// The caption_studies table has a FK constraint. We check existence.
			var studyExists int
			err := db.QueryRow("SELECT COUNT(*) FROM caption_studies WHERE id=?", cap.StudyID).Scan(&studyExists)
			if err != nil {
				return err
			}
			if studyExists == 0 {
				// Study doesn't exist in DB — skip this caption (study may be reconciled later
				// or was never created). Log at debug level.
				logrus.WithFields(logrus.Fields{
					"sample_id": sampleID,
					"study_id":  cap.StudyID,
				}).Debug("Skipping caption: study not found in DB")
				continue
			}

			newID := uuid.New().String()
			_, err = db.Exec(`INSERT INTO captions (id, sample_id, study_id, text, created_at) VALUES (?, ?, ?, ?, ?)`,
				newID, sampleID, cap.StudyID, cap.Text, cap.CreatedAt)
			if err != nil {
				return fmt.Errorf("insert caption for study %s: %w", cap.StudyID, err)
			}
		}
	}

	// Remove captions whose studies are no longer in the manifest.
	for studyID, capID := range existingByStudy {
		_ = studyID
		_, err := db.Exec("DELETE FROM captions WHERE id=?", capID)
		if err != nil {
			return err
		}
	}

	return nil
}

// queryIDs returns a set of IDs from a simple SELECT query.
func queryIDs(db *sql.DB, query string) (map[string]bool, error) {
	rows, err := db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	ids := make(map[string]bool)
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids[id] = true
	}
	return ids, rows.Err()
}

// marshalStringSlice converts a string slice to JSON for storage.
func marshalStringSlice(s []string) *string {
	if len(s) == 0 {
		return nil
	}
	data, _ := json.Marshal(s)
	str := string(data)
	return &str
}

// nullIfEmpty returns nil if the string is empty, otherwise a pointer to it.
func nullIfEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
