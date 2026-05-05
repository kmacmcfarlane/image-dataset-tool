package reconciler_test

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/kmacmcfarlane/image-dataset-tool/backend/internal/fileformat"
	"github.com/kmacmcfarlane/image-dataset-tool/backend/internal/reconciler"
	"github.com/kmacmcfarlane/image-dataset-tool/backend/internal/store"
)

func TestReconciler(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Reconciler Suite")
}

var _ = Describe("Reconciler", func() {
	var (
		db      *sql.DB
		dataDir string
	)

	BeforeEach(func() {
		var err error
		dataDir, err = os.MkdirTemp("", "reconciler-test-*")
		Expect(err).NotTo(HaveOccurred())

		// Create projects dir.
		Expect(os.MkdirAll(filepath.Join(dataDir, "projects"), 0755)).To(Succeed())

		// Open temp DB.
		dbPath := filepath.Join(dataDir, "db.sqlite")
		db, err = store.OpenDB(dbPath)
		Expect(err).NotTo(HaveOccurred())
		Expect(store.Migrate(db)).To(Succeed())
	})

	AfterEach(func() {
		if db != nil {
			db.Close()
		}
		os.RemoveAll(dataDir)
	})

	Describe("Run", func() {
		Context("with empty filesystem", func() {
			It("completes without error and leaves DB empty", func() {
				err := reconciler.Run(db, dataDir)
				Expect(err).NotTo(HaveOccurred())

				var count int
				Expect(db.QueryRow("SELECT COUNT(*) FROM projects").Scan(&count)).To(Succeed())
				Expect(count).To(Equal(0))
			})
		})

		Context("with no projects directory", func() {
			It("completes without error", func() {
				noProjectsDir, err := os.MkdirTemp("", "reconciler-noproj-*")
				Expect(err).NotTo(HaveOccurred())
				defer os.RemoveAll(noProjectsDir)

				err = reconciler.Run(db, noProjectsDir)
				Expect(err).NotTo(HaveOccurred())
			})
		})

		Context("adding manifests to DB", func() {
			It("adds project from disk manifest to DB", func() {
				writeProject(dataDir, &fileformat.ProjectManifest{
					ID:          "proj-001",
					Slug:        "my-project",
					Name:        "My Project",
					Description: "Test",
					CreatedAt:   "2026-01-01T00:00:00Z",
					UpdatedAt:   "2026-01-01T00:00:00Z",
				})

				Expect(reconciler.Run(db, dataDir)).To(Succeed())

				var name string
				Expect(db.QueryRow("SELECT name FROM projects WHERE id=?", "proj-001").Scan(&name)).To(Succeed())
				Expect(name).To(Equal("My Project"))
			})

			It("adds subject from disk manifest to DB", func() {
				writeProject(dataDir, &fileformat.ProjectManifest{
					ID:        "proj-001",
					Slug:      "my-project",
					Name:      "My Project",
					CreatedAt: "2026-01-01T00:00:00Z",
					UpdatedAt: "2026-01-01T00:00:00Z",
				})
				writeSubject(dataDir, "my-project", &fileformat.SubjectManifest{
					ID:        "subj-001",
					ProjectID: "proj-001",
					Slug:      "test-subject",
					Name:      "Test Subject",
					CreatedAt: "2026-01-01T00:00:00Z",
					UpdatedAt: "2026-01-01T00:00:00Z",
				})

				Expect(reconciler.Run(db, dataDir)).To(Succeed())

				var name string
				Expect(db.QueryRow("SELECT name FROM subjects WHERE id=?", "subj-001").Scan(&name)).To(Succeed())
				Expect(name).To(Equal("Test Subject"))
			})

			It("adds sample from disk manifest to DB", func() {
				writeProject(dataDir, &fileformat.ProjectManifest{
					ID: "proj-001", Slug: "my-project", Name: "P",
					CreatedAt: "2026-01-01T00:00:00Z", UpdatedAt: "2026-01-01T00:00:00Z",
				})
				writeSubject(dataDir, "my-project", &fileformat.SubjectManifest{
					ID: "subj-001", ProjectID: "proj-001", Slug: "test-subject", Name: "S",
					CreatedAt: "2026-01-01T00:00:00Z", UpdatedAt: "2026-01-01T00:00:00Z",
				})
				writeSample(dataDir, "my-project", "test-subject", &fileformat.SampleMetadata{
					ID: "samp-001",
					Source: fileformat.SampleSource{
						Platform: "instagram", PostID: "POST1", SlideIndex: 0,
					},
					File: fileformat.SampleFile{
						Path: "samp-001.jpg", SHA256: "abc123", PHash: "def456",
						Width: 1080, Height: 1350,
					},
					Status:    "kept",
					Captions:  map[string]fileformat.Caption{},
					FetchedAt: "2026-01-01T00:00:00Z",
					CreatedAt: "2026-01-01T00:00:00Z",
					UpdatedAt: "2026-01-01T00:00:00Z",
				})

				Expect(reconciler.Run(db, dataDir)).To(Succeed())

				var status string
				Expect(db.QueryRow("SELECT status FROM samples WHERE id=?", "samp-001").Scan(&status)).To(Succeed())
				Expect(status).To(Equal("kept"))
			})
		})

		Context("removing stale DB rows", func() {
			It("removes project from DB when manifest deleted from disk", func() {
				// Insert a project directly into DB.
				_, err := db.Exec(`INSERT INTO projects (id, slug, name, description, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?)`,
					"stale-proj", "stale", "Stale", "", "2026-01-01T00:00:00Z", "2026-01-01T00:00:00Z")
				Expect(err).NotTo(HaveOccurred())

				// Run reconciler with empty filesystem.
				Expect(reconciler.Run(db, dataDir)).To(Succeed())

				var count int
				Expect(db.QueryRow("SELECT COUNT(*) FROM projects WHERE id=?", "stale-proj").Scan(&count)).To(Succeed())
				Expect(count).To(Equal(0))
			})

			It("removes sample from DB when manifest deleted from disk", func() {
				// Set up project+subject on disk and in DB.
				writeProject(dataDir, &fileformat.ProjectManifest{
					ID: "proj-001", Slug: "my-project", Name: "P",
					CreatedAt: "2026-01-01T00:00:00Z", UpdatedAt: "2026-01-01T00:00:00Z",
				})
				writeSubject(dataDir, "my-project", &fileformat.SubjectManifest{
					ID: "subj-001", ProjectID: "proj-001", Slug: "test-subject", Name: "S",
					CreatedAt: "2026-01-01T00:00:00Z", UpdatedAt: "2026-01-01T00:00:00Z",
				})

				// Insert sample into DB but NOT on disk.
				_, err := db.Exec(`INSERT INTO projects (id, slug, name, description, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?)`,
					"proj-001", "my-project", "P", "", "2026-01-01T00:00:00Z", "2026-01-01T00:00:00Z")
				Expect(err).NotTo(HaveOccurred())
				_, err = db.Exec(`INSERT INTO subjects (id, project_id, slug, name, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?)`,
					"subj-001", "proj-001", "test-subject", "S", "2026-01-01T00:00:00Z", "2026-01-01T00:00:00Z")
				Expect(err).NotTo(HaveOccurred())
				_, err = db.Exec(`INSERT INTO samples (id, subject_id, source_platform, source_post_id, slide_index, file_path, sha256, phash, width, height, fetched_at, status, is_duplicate, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
					"stale-samp", "subj-001", "instagram", "POST99", 0, "stale.jpg", "xxx", "yyy", 100, 100, "2026-01-01T00:00:00Z", "pending", 0, "2026-01-01T00:00:00Z", "2026-01-01T00:00:00Z")
				Expect(err).NotTo(HaveOccurred())

				Expect(reconciler.Run(db, dataDir)).To(Succeed())

				var count int
				Expect(db.QueryRow("SELECT COUNT(*) FROM samples WHERE id=?", "stale-samp").Scan(&count)).To(Succeed())
				Expect(count).To(Equal(0))
			})
		})

		Context("updating existing DB rows", func() {
			It("updates project in DB when manifest changes", func() {
				// Insert project into DB with old name.
				_, err := db.Exec(`INSERT INTO projects (id, slug, name, description, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?)`,
					"proj-001", "my-project", "Old Name", "", "2026-01-01T00:00:00Z", "2026-01-01T00:00:00Z")
				Expect(err).NotTo(HaveOccurred())

				// Write manifest with new name.
				writeProject(dataDir, &fileformat.ProjectManifest{
					ID: "proj-001", Slug: "my-project", Name: "New Name",
					CreatedAt: "2026-01-01T00:00:00Z", UpdatedAt: "2026-01-02T00:00:00Z",
				})

				Expect(reconciler.Run(db, dataDir)).To(Succeed())

				var name string
				Expect(db.QueryRow("SELECT name FROM projects WHERE id=?", "proj-001").Scan(&name)).To(Succeed())
				Expect(name).To(Equal("New Name"))
			})

			It("updates sample status when manifest changes", func() {
				writeProject(dataDir, &fileformat.ProjectManifest{
					ID: "proj-001", Slug: "my-project", Name: "P",
					CreatedAt: "2026-01-01T00:00:00Z", UpdatedAt: "2026-01-01T00:00:00Z",
				})
				writeSubject(dataDir, "my-project", &fileformat.SubjectManifest{
					ID: "subj-001", ProjectID: "proj-001", Slug: "test-subject", Name: "S",
					CreatedAt: "2026-01-01T00:00:00Z", UpdatedAt: "2026-01-01T00:00:00Z",
				})

				// Insert sample with status=pending.
				_, err := db.Exec(`INSERT INTO projects (id, slug, name, description, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?)`,
					"proj-001", "my-project", "P", "", "2026-01-01T00:00:00Z", "2026-01-01T00:00:00Z")
				Expect(err).NotTo(HaveOccurred())
				_, err = db.Exec(`INSERT INTO subjects (id, project_id, slug, name, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?)`,
					"subj-001", "proj-001", "test-subject", "S", "2026-01-01T00:00:00Z", "2026-01-01T00:00:00Z")
				Expect(err).NotTo(HaveOccurred())
				_, err = db.Exec(`INSERT INTO samples (id, subject_id, source_platform, source_post_id, slide_index, file_path, sha256, phash, width, height, fetched_at, status, is_duplicate, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
					"samp-001", "subj-001", "instagram", "POST1", 0, "samp-001.jpg", "abc", "def", 1080, 1350, "2026-01-01T00:00:00Z", "pending", 0, "2026-01-01T00:00:00Z", "2026-01-01T00:00:00Z")
				Expect(err).NotTo(HaveOccurred())

				// Write manifest with status=kept.
				writeSample(dataDir, "my-project", "test-subject", &fileformat.SampleMetadata{
					ID: "samp-001",
					Source: fileformat.SampleSource{
						Platform: "instagram", PostID: "POST1", SlideIndex: 0,
					},
					File: fileformat.SampleFile{
						Path: "samp-001.jpg", SHA256: "abc", PHash: "def",
						Width: 1080, Height: 1350,
					},
					Status:    "kept",
					Captions:  map[string]fileformat.Caption{},
					FetchedAt: "2026-01-01T00:00:00Z",
					CreatedAt: "2026-01-01T00:00:00Z",
					UpdatedAt: "2026-01-02T00:00:00Z",
				})

				Expect(reconciler.Run(db, dataDir)).To(Succeed())

				var status string
				Expect(db.QueryRow("SELECT status FROM samples WHERE id=?", "samp-001").Scan(&status)).To(Succeed())
				Expect(status).To(Equal("kept"))
			})
		})

		Context("malformed manifest handling", func() {
			It("skips malformed project manifest without error", func() {
				projDir := filepath.Join(dataDir, "projects", "bad-project")
				Expect(os.MkdirAll(projDir, 0755)).To(Succeed())
				Expect(os.WriteFile(filepath.Join(projDir, "manifest.json"), []byte("{invalid json"), 0644)).To(Succeed())

				err := reconciler.Run(db, dataDir)
				Expect(err).NotTo(HaveOccurred())
			})

			It("skips malformed subject manifest without error", func() {
				writeProject(dataDir, &fileformat.ProjectManifest{
					ID: "proj-001", Slug: "my-project", Name: "P",
					CreatedAt: "2026-01-01T00:00:00Z", UpdatedAt: "2026-01-01T00:00:00Z",
				})
				subjDir := filepath.Join(dataDir, "projects", "my-project", "subjects", "bad-subject")
				Expect(os.MkdirAll(subjDir, 0755)).To(Succeed())
				Expect(os.WriteFile(filepath.Join(subjDir, "manifest.json"), []byte("not json"), 0644)).To(Succeed())

				err := reconciler.Run(db, dataDir)
				Expect(err).NotTo(HaveOccurred())
			})

			It("skips malformed sample JSON without deleting file", func() {
				writeProject(dataDir, &fileformat.ProjectManifest{
					ID: "proj-001", Slug: "my-project", Name: "P",
					CreatedAt: "2026-01-01T00:00:00Z", UpdatedAt: "2026-01-01T00:00:00Z",
				})
				writeSubject(dataDir, "my-project", &fileformat.SubjectManifest{
					ID: "subj-001", ProjectID: "proj-001", Slug: "test-subject", Name: "S",
					CreatedAt: "2026-01-01T00:00:00Z", UpdatedAt: "2026-01-01T00:00:00Z",
				})

				samplesDir := filepath.Join(dataDir, "projects", "my-project", "subjects", "test-subject", "samples")
				Expect(os.MkdirAll(samplesDir, 0755)).To(Succeed())
				badPath := filepath.Join(samplesDir, "bad-sample.json")
				Expect(os.WriteFile(badPath, []byte("{corrupt"), 0644)).To(Succeed())

				err := reconciler.Run(db, dataDir)
				Expect(err).NotTo(HaveOccurred())

				// File must still exist (not deleted).
				_, statErr := os.Stat(badPath)
				Expect(statErr).NotTo(HaveOccurred())
			})
		})

		Context("idempotency", func() {
			It("running twice produces identical DB state", func() {
				writeProject(dataDir, &fileformat.ProjectManifest{
					ID: "proj-001", Slug: "my-project", Name: "My Project",
					Description: "Desc", CreatedAt: "2026-01-01T00:00:00Z", UpdatedAt: "2026-01-01T00:00:00Z",
				})
				writeSubject(dataDir, "my-project", &fileformat.SubjectManifest{
					ID: "subj-001", ProjectID: "proj-001", Slug: "test-subject", Name: "Subject",
					CreatedAt: "2026-01-01T00:00:00Z", UpdatedAt: "2026-01-01T00:00:00Z",
				})
				writeSample(dataDir, "my-project", "test-subject", &fileformat.SampleMetadata{
					ID: "samp-001",
					Source: fileformat.SampleSource{
						Platform: "instagram", PostID: "POST1", SlideIndex: 0,
					},
					File: fileformat.SampleFile{
						Path: "samp-001.jpg", SHA256: "abc123", PHash: "def456",
						Width: 1080, Height: 1350,
					},
					Status:    "kept",
					Captions:  map[string]fileformat.Caption{},
					FetchedAt: "2026-01-01T00:00:00Z",
					CreatedAt: "2026-01-01T00:00:00Z",
					UpdatedAt: "2026-01-01T00:00:00Z",
				})

				// Run once.
				Expect(reconciler.Run(db, dataDir)).To(Succeed())

				// Capture state.
				var projCount1, subjCount1, sampCount1 int
				Expect(db.QueryRow("SELECT COUNT(*) FROM projects").Scan(&projCount1)).To(Succeed())
				Expect(db.QueryRow("SELECT COUNT(*) FROM subjects").Scan(&subjCount1)).To(Succeed())
				Expect(db.QueryRow("SELECT COUNT(*) FROM samples").Scan(&sampCount1)).To(Succeed())

				var projName1, subjName1, sampStatus1 string
				Expect(db.QueryRow("SELECT name FROM projects WHERE id='proj-001'").Scan(&projName1)).To(Succeed())
				Expect(db.QueryRow("SELECT name FROM subjects WHERE id='subj-001'").Scan(&subjName1)).To(Succeed())
				Expect(db.QueryRow("SELECT status FROM samples WHERE id='samp-001'").Scan(&sampStatus1)).To(Succeed())

				// Run again.
				Expect(reconciler.Run(db, dataDir)).To(Succeed())

				// Verify identical state.
				var projCount2, subjCount2, sampCount2 int
				Expect(db.QueryRow("SELECT COUNT(*) FROM projects").Scan(&projCount2)).To(Succeed())
				Expect(db.QueryRow("SELECT COUNT(*) FROM subjects").Scan(&subjCount2)).To(Succeed())
				Expect(db.QueryRow("SELECT COUNT(*) FROM samples").Scan(&sampCount2)).To(Succeed())

				Expect(projCount2).To(Equal(projCount1))
				Expect(subjCount2).To(Equal(subjCount1))
				Expect(sampCount2).To(Equal(sampCount1))

				var projName2, subjName2, sampStatus2 string
				Expect(db.QueryRow("SELECT name FROM projects WHERE id='proj-001'").Scan(&projName2)).To(Succeed())
				Expect(db.QueryRow("SELECT name FROM subjects WHERE id='subj-001'").Scan(&subjName2)).To(Succeed())
				Expect(db.QueryRow("SELECT status FROM samples WHERE id='samp-001'").Scan(&sampStatus2)).To(Succeed())

				Expect(projName2).To(Equal(projName1))
				Expect(subjName2).To(Equal(subjName1))
				Expect(sampStatus2).To(Equal(sampStatus1))
			})
		})

		Context("captions", func() {
			It("imports captions from sample manifest into captions table", func() {
				writeProject(dataDir, &fileformat.ProjectManifest{
					ID: "proj-001", Slug: "my-project", Name: "P",
					CreatedAt: "2026-01-01T00:00:00Z", UpdatedAt: "2026-01-01T00:00:00Z",
				})
				writeSubject(dataDir, "my-project", &fileformat.SubjectManifest{
					ID: "subj-001", ProjectID: "proj-001", Slug: "test-subject", Name: "S",
					CreatedAt: "2026-01-01T00:00:00Z", UpdatedAt: "2026-01-01T00:00:00Z",
				})

				// Create caption study in DB first (FK requirement).
				_, err := db.Exec(`INSERT INTO projects (id, slug, name, description, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?)`,
					"proj-001", "my-project", "P", "", "2026-01-01T00:00:00Z", "2026-01-01T00:00:00Z")
				Expect(err).NotTo(HaveOccurred())
				_, err = db.Exec(`INSERT INTO subjects (id, project_id, slug, name, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?)`,
					"subj-001", "proj-001", "test-subject", "S", "2026-01-01T00:00:00Z", "2026-01-01T00:00:00Z")
				Expect(err).NotTo(HaveOccurred())
				_, err = db.Exec(`INSERT INTO caption_studies (id, project_id, name, slug, provider, model, prompt_template, params_json, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
					"study-001", "proj-001", "Natural Claude", "natural-claude", "anthropic", "claude-3", "Describe this image", "{}", "2026-01-01T00:00:00Z", "2026-01-01T00:00:00Z")
				Expect(err).NotTo(HaveOccurred())

				writeSample(dataDir, "my-project", "test-subject", &fileformat.SampleMetadata{
					ID: "samp-001",
					Source: fileformat.SampleSource{
						Platform: "instagram", PostID: "POST1", SlideIndex: 0,
					},
					File: fileformat.SampleFile{
						Path: "samp-001.jpg", SHA256: "abc", PHash: "def",
						Width: 1080, Height: 1350,
					},
					Status: "kept",
					Captions: map[string]fileformat.Caption{
						"natural-claude": {
							StudyID:   "study-001",
							Text:      "A beautiful sunset",
							CreatedAt: "2026-01-02T00:00:00Z",
						},
					},
					FetchedAt: "2026-01-01T00:00:00Z",
					CreatedAt: "2026-01-01T00:00:00Z",
					UpdatedAt: "2026-01-02T00:00:00Z",
				})

				Expect(reconciler.Run(db, dataDir)).To(Succeed())

				var text string
				err = db.QueryRow("SELECT text FROM captions WHERE sample_id=? AND study_id=?", "samp-001", "study-001").Scan(&text)
				Expect(err).NotTo(HaveOccurred())
				Expect(text).To(Equal("A beautiful sunset"))
			})

			It("skips captions when study does not exist in DB", func() {
				writeProject(dataDir, &fileformat.ProjectManifest{
					ID: "proj-001", Slug: "my-project", Name: "P",
					CreatedAt: "2026-01-01T00:00:00Z", UpdatedAt: "2026-01-01T00:00:00Z",
				})
				writeSubject(dataDir, "my-project", &fileformat.SubjectManifest{
					ID: "subj-001", ProjectID: "proj-001", Slug: "test-subject", Name: "S",
					CreatedAt: "2026-01-01T00:00:00Z", UpdatedAt: "2026-01-01T00:00:00Z",
				})
				writeSample(dataDir, "my-project", "test-subject", &fileformat.SampleMetadata{
					ID: "samp-001",
					Source: fileformat.SampleSource{
						Platform: "instagram", PostID: "POST1", SlideIndex: 0,
					},
					File: fileformat.SampleFile{
						Path: "samp-001.jpg", SHA256: "abc", PHash: "def",
						Width: 1080, Height: 1350,
					},
					Status: "kept",
					Captions: map[string]fileformat.Caption{
						"nonexistent-study": {
							StudyID:   "study-nonexistent",
							Text:      "Should not be imported",
							CreatedAt: "2026-01-02T00:00:00Z",
						},
					},
					FetchedAt: "2026-01-01T00:00:00Z",
					CreatedAt: "2026-01-01T00:00:00Z",
					UpdatedAt: "2026-01-02T00:00:00Z",
				})

				Expect(reconciler.Run(db, dataDir)).To(Succeed())

				var count int
				Expect(db.QueryRow("SELECT COUNT(*) FROM captions").Scan(&count)).To(Succeed())
				Expect(count).To(Equal(0))
			})
		})
	})
})

// Helper functions for writing test manifests.

func writeProject(dataDir string, m *fileformat.ProjectManifest) {
	projDir := filepath.Join(dataDir, "projects", m.Slug)
	Expect(os.MkdirAll(projDir, 0755)).To(Succeed())
	Expect(fileformat.WriteProjectManifest(filepath.Join(projDir, "manifest.json"), m)).To(Succeed())
}

func writeSubject(dataDir, projectSlug string, m *fileformat.SubjectManifest) {
	subjDir := filepath.Join(dataDir, "projects", projectSlug, "subjects", m.Slug)
	Expect(os.MkdirAll(subjDir, 0755)).To(Succeed())
	Expect(fileformat.WriteSubjectManifest(filepath.Join(subjDir, "manifest.json"), m)).To(Succeed())
}

func writeSample(dataDir, projectSlug, subjectSlug string, m *fileformat.SampleMetadata) {
	samplesDir := filepath.Join(dataDir, "projects", projectSlug, "subjects", subjectSlug, "samples")
	Expect(os.MkdirAll(samplesDir, 0755)).To(Succeed())
	Expect(fileformat.WriteSampleMetadata(filepath.Join(samplesDir, m.ID+".json"), m)).To(Succeed())
}
