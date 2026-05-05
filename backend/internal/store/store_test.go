package store_test

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/kmacmcfarlane/image-dataset-tool/backend/internal/store"
)

func TestStore(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Store Suite")
}

var _ = Describe("Store", func() {
	var (
		tmpDir string
		dbPath string
	)

	BeforeEach(func() {
		var err error
		tmpDir, err = os.MkdirTemp("", "store-test-*")
		Expect(err).NotTo(HaveOccurred())
		dbPath = filepath.Join(tmpDir, "test.sqlite")
	})

	AfterEach(func() {
		os.RemoveAll(tmpDir)
	})

	Describe("OpenDB", func() {
		It("creates a database with WAL mode and foreign keys", func() {
			db, err := store.OpenDB(dbPath)
			Expect(err).NotTo(HaveOccurred())
			defer db.Close()

			var journalMode string
			err = db.QueryRow("PRAGMA journal_mode").Scan(&journalMode)
			Expect(err).NotTo(HaveOccurred())
			Expect(journalMode).To(Equal("wal"))

			var fkEnabled int
			err = db.QueryRow("PRAGMA foreign_keys").Scan(&fkEnabled)
			Expect(err).NotTo(HaveOccurred())
			Expect(fkEnabled).To(Equal(1))
		})

		It("creates the database file on disk", func() {
			db, err := store.OpenDB(dbPath)
			Expect(err).NotTo(HaveOccurred())
			defer db.Close()

			_, err = os.Stat(dbPath)
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Describe("Migrate", func() {
		var db *sql.DB

		BeforeEach(func() {
			var err error
			db, err = store.OpenDB(dbPath)
			Expect(err).NotTo(HaveOccurred())
		})

		AfterEach(func() {
			db.Close()
		})

		It("creates all tables from database.md", func() {
			err := store.Migrate(db)
			Expect(err).NotTo(HaveOccurred())

			expectedTables := []string{
				"projects",
				"subjects",
				"accounts",
				"subject_accounts",
				"samples",
				"edits",
				"caption_studies",
				"captions",
				"job_runs",
				"job_messages",
				"secrets",
				"schema_migrations",
			}

			for _, table := range expectedTables {
				var name string
				err := db.QueryRow(
					"SELECT name FROM sqlite_master WHERE type='table' AND name=?", table,
				).Scan(&name)
				Expect(err).NotTo(HaveOccurred(), "table %s should exist", table)
			}
		})

		It("is idempotent", func() {
			err := store.Migrate(db)
			Expect(err).NotTo(HaveOccurred())

			err = store.Migrate(db)
			Expect(err).NotTo(HaveOccurred())
		})

		It("enforces ON DELETE CASCADE for projects -> subjects", func() {
			err := store.Migrate(db)
			Expect(err).NotTo(HaveOccurred())

			// Insert project and subject
			_, err = db.Exec(`INSERT INTO projects (id, slug, name, created_at, updated_at)
				VALUES ('p1', 'proj', 'Project', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z')`)
			Expect(err).NotTo(HaveOccurred())

			_, err = db.Exec(`INSERT INTO subjects (id, project_id, slug, name, created_at, updated_at)
				VALUES ('s1', 'p1', 'subj', 'Subject', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z')`)
			Expect(err).NotTo(HaveOccurred())

			// Delete project - should cascade to subject
			_, err = db.Exec("DELETE FROM projects WHERE id = 'p1'")
			Expect(err).NotTo(HaveOccurred())

			var count int
			err = db.QueryRow("SELECT COUNT(*) FROM subjects WHERE id = 's1'").Scan(&count)
			Expect(err).NotTo(HaveOccurred())
			Expect(count).To(Equal(0))
		})

		It("enforces ON DELETE CASCADE for subjects -> samples", func() {
			err := store.Migrate(db)
			Expect(err).NotTo(HaveOccurred())

			_, err = db.Exec(`INSERT INTO projects (id, slug, name, created_at, updated_at)
				VALUES ('p1', 'proj', 'Project', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z')`)
			Expect(err).NotTo(HaveOccurred())

			_, err = db.Exec(`INSERT INTO subjects (id, project_id, slug, name, created_at, updated_at)
				VALUES ('s1', 'p1', 'subj', 'Subject', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z')`)
			Expect(err).NotTo(HaveOccurred())

			_, err = db.Exec(`INSERT INTO samples (id, subject_id, source_platform, source_post_id, slide_index,
				file_path, sha256, phash, width, height, fetched_at, created_at, updated_at)
				VALUES ('sam1', 's1', 'instagram', 'post1', 0, 'img.jpg', 'abc', 'def', 100, 100,
				'2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z')`)
			Expect(err).NotTo(HaveOccurred())

			_, err = db.Exec("DELETE FROM subjects WHERE id = 's1'")
			Expect(err).NotTo(HaveOccurred())

			var count int
			err = db.QueryRow("SELECT COUNT(*) FROM samples WHERE id = 'sam1'").Scan(&count)
			Expect(err).NotTo(HaveOccurred())
			Expect(count).To(Equal(0))
		})

		It("enforces ON DELETE CASCADE for samples -> edits", func() {
			err := store.Migrate(db)
			Expect(err).NotTo(HaveOccurred())

			_, err = db.Exec(`INSERT INTO projects (id, slug, name, created_at, updated_at)
				VALUES ('p1', 'proj', 'Project', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z')`)
			Expect(err).NotTo(HaveOccurred())

			_, err = db.Exec(`INSERT INTO subjects (id, project_id, slug, name, created_at, updated_at)
				VALUES ('s1', 'p1', 'subj', 'Subject', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z')`)
			Expect(err).NotTo(HaveOccurred())

			_, err = db.Exec(`INSERT INTO samples (id, subject_id, source_platform, source_post_id, slide_index,
				file_path, sha256, phash, width, height, fetched_at, created_at, updated_at)
				VALUES ('sam1', 's1', 'instagram', 'post1', 0, 'img.jpg', 'abc', 'def', 100, 100,
				'2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z')`)
			Expect(err).NotTo(HaveOccurred())

			_, err = db.Exec(`INSERT INTO edits (sample_id, created_at, updated_at)
				VALUES ('sam1', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z')`)
			Expect(err).NotTo(HaveOccurred())

			_, err = db.Exec("DELETE FROM samples WHERE id = 'sam1'")
			Expect(err).NotTo(HaveOccurred())

			var count int
			err = db.QueryRow("SELECT COUNT(*) FROM edits WHERE sample_id = 'sam1'").Scan(&count)
			Expect(err).NotTo(HaveOccurred())
			Expect(count).To(Equal(0))
		})

		It("enforces UNIQUE constraint on projects.slug", func() {
			err := store.Migrate(db)
			Expect(err).NotTo(HaveOccurred())

			_, err = db.Exec(`INSERT INTO projects (id, slug, name, created_at, updated_at)
				VALUES ('p1', 'same-slug', 'P1', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z')`)
			Expect(err).NotTo(HaveOccurred())

			_, err = db.Exec(`INSERT INTO projects (id, slug, name, created_at, updated_at)
				VALUES ('p2', 'same-slug', 'P2', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z')`)
			Expect(err).To(HaveOccurred())
		})

		It("enforces UNIQUE constraint on subjects (project_id, slug)", func() {
			err := store.Migrate(db)
			Expect(err).NotTo(HaveOccurred())

			_, err = db.Exec(`INSERT INTO projects (id, slug, name, created_at, updated_at)
				VALUES ('p1', 'proj', 'Project', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z')`)
			Expect(err).NotTo(HaveOccurred())

			_, err = db.Exec(`INSERT INTO subjects (id, project_id, slug, name, created_at, updated_at)
				VALUES ('s1', 'p1', 'same-slug', 'Subject 1', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z')`)
			Expect(err).NotTo(HaveOccurred())

			_, err = db.Exec(`INSERT INTO subjects (id, project_id, slug, name, created_at, updated_at)
				VALUES ('s2', 'p1', 'same-slug', 'Subject 2', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z')`)
			Expect(err).To(HaveOccurred())
		})

		It("enforces UNIQUE constraint on caption_studies (project_id, slug)", func() {
			err := store.Migrate(db)
			Expect(err).NotTo(HaveOccurred())

			_, err = db.Exec(`INSERT INTO projects (id, slug, name, created_at, updated_at)
				VALUES ('p1', 'proj', 'Project', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z')`)
			Expect(err).NotTo(HaveOccurred())

			_, err = db.Exec(`INSERT INTO caption_studies (id, project_id, name, slug, provider, model, prompt_template, created_at, updated_at)
				VALUES ('cs1', 'p1', 'Study 1', 'same-slug', 'anthropic', 'claude', 'template', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z')`)
			Expect(err).NotTo(HaveOccurred())

			_, err = db.Exec(`INSERT INTO caption_studies (id, project_id, name, slug, provider, model, prompt_template, created_at, updated_at)
				VALUES ('cs2', 'p1', 'Study 2', 'same-slug', 'anthropic', 'claude', 'template', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z')`)
			Expect(err).To(HaveOccurred())
		})

		It("enforces UNIQUE constraint on samples (subject_id, source_post_id, slide_index)", func() {
			err := store.Migrate(db)
			Expect(err).NotTo(HaveOccurred())

			_, err = db.Exec(`INSERT INTO projects (id, slug, name, created_at, updated_at)
				VALUES ('p1', 'proj', 'Project', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z')`)
			Expect(err).NotTo(HaveOccurred())

			_, err = db.Exec(`INSERT INTO subjects (id, project_id, slug, name, created_at, updated_at)
				VALUES ('s1', 'p1', 'subj', 'Subject', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z')`)
			Expect(err).NotTo(HaveOccurred())

			_, err = db.Exec(`INSERT INTO samples (id, subject_id, source_platform, source_post_id, slide_index,
				file_path, sha256, phash, width, height, fetched_at, created_at, updated_at)
				VALUES ('sam1', 's1', 'instagram', 'post1', 0, 'img.jpg', 'abc', 'def', 100, 100,
				'2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z')`)
			Expect(err).NotTo(HaveOccurred())

			_, err = db.Exec(`INSERT INTO samples (id, subject_id, source_platform, source_post_id, slide_index,
				file_path, sha256, phash, width, height, fetched_at, created_at, updated_at)
				VALUES ('sam2', 's1', 'instagram', 'post1', 0, 'img2.jpg', 'xyz', 'ghi', 100, 100,
				'2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z')`)
			Expect(err).To(HaveOccurred())
		})

		It("enforces UNIQUE constraint on accounts (platform, handle)", func() {
			err := store.Migrate(db)
			Expect(err).NotTo(HaveOccurred())

			_, err = db.Exec(`INSERT INTO accounts (id, platform, handle, created_at, updated_at)
				VALUES ('a1', 'instagram', 'user1', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z')`)
			Expect(err).NotTo(HaveOccurred())

			_, err = db.Exec(`INSERT INTO accounts (id, platform, handle, created_at, updated_at)
				VALUES ('a2', 'instagram', 'user1', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z')`)
			Expect(err).To(HaveOccurred())
		})

		It("enforces UNIQUE constraint on captions (sample_id, study_id)", func() {
			err := store.Migrate(db)
			Expect(err).NotTo(HaveOccurred())

			_, err = db.Exec(`INSERT INTO projects (id, slug, name, created_at, updated_at)
				VALUES ('p1', 'proj', 'Project', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z')`)
			Expect(err).NotTo(HaveOccurred())

			_, err = db.Exec(`INSERT INTO subjects (id, project_id, slug, name, created_at, updated_at)
				VALUES ('s1', 'p1', 'subj', 'Subject', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z')`)
			Expect(err).NotTo(HaveOccurred())

			_, err = db.Exec(`INSERT INTO samples (id, subject_id, source_platform, source_post_id, slide_index,
				file_path, sha256, phash, width, height, fetched_at, created_at, updated_at)
				VALUES ('sam1', 's1', 'instagram', 'post1', 0, 'img.jpg', 'abc', 'def', 100, 100,
				'2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z')`)
			Expect(err).NotTo(HaveOccurred())

			_, err = db.Exec(`INSERT INTO caption_studies (id, project_id, name, slug, provider, model, prompt_template, created_at, updated_at)
				VALUES ('cs1', 'p1', 'Study', 'study', 'anthropic', 'claude', 'template', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z')`)
			Expect(err).NotTo(HaveOccurred())

			_, err = db.Exec(`INSERT INTO captions (id, sample_id, study_id, text, created_at)
				VALUES ('c1', 'sam1', 'cs1', 'caption text', '2026-01-01T00:00:00Z')`)
			Expect(err).NotTo(HaveOccurred())

			_, err = db.Exec(`INSERT INTO captions (id, sample_id, study_id, text, created_at)
				VALUES ('c2', 'sam1', 'cs1', 'different text', '2026-01-01T00:00:00Z')`)
			Expect(err).To(HaveOccurred())
		})
	})
})
