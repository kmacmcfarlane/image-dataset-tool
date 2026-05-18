package service_test

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/kmacmcfarlane/image-dataset-tool/backend/internal/service"
	"github.com/kmacmcfarlane/image-dataset-tool/backend/internal/store"
	"github.com/kmacmcfarlane/image-dataset-tool/backend/internal/natsutil"
)

var _ = Describe("ExportService", func() {
	var (
		db        *sql.DB
		tmpDir    string
		natsSrv   *natsutil.Server
		exportSvc *service.ExportService
		jobStore  *store.JobStore
		cleanup   func()
	)

	BeforeEach(func() {
		var err error
		tmpDir, err = os.MkdirTemp("", "export-svc-test-*")
		Expect(err).NotTo(HaveOccurred())

		dbPath := filepath.Join(tmpDir, "test.sqlite")
		db, err = store.OpenDB(dbPath)
		Expect(err).NotTo(HaveOccurred())
		Expect(store.Migrate(db)).To(Succeed())

		natsDir := filepath.Join(tmpDir, "nats")
		natsCfg := natsutil.DefaultConfig(natsDir)
		natsSrv, err = natsutil.New(natsCfg)
		Expect(err).NotTo(HaveOccurred())

		jobStore = store.NewJobStore(db)
		exportSvc = service.NewExportService(db, natsSrv.JS(), jobStore, tmpDir)

		cleanup = func() {
			natsSrv.Shutdown()
			db.Close()
			os.RemoveAll(tmpDir)
		}
	})

	AfterEach(func() {
		cleanup()
	})

	seedProject := func() string {
		now := time.Now().UTC().Format(time.RFC3339)
		_, err := db.Exec(`INSERT INTO projects (id, slug, name, created_at, updated_at)
			VALUES ('proj-1', 'test-project', 'Test Project', ?, ?)`, now, now)
		Expect(err).NotTo(HaveOccurred())
		return "proj-1"
	}

	seedSubject := func(projectID string) string {
		now := time.Now().UTC().Format(time.RFC3339)
		_, err := db.Exec(`INSERT INTO subjects (id, project_id, slug, name, created_at, updated_at)
			VALUES ('subj-1', ?, 'test-subject', 'Test Subject', ?, ?)`, projectID, now, now)
		Expect(err).NotTo(HaveOccurred())
		return "subj-1"
	}

	seedSample := func(subjectID, sampleID string) {
		now := time.Now().UTC().Format(time.RFC3339)
		_, err := db.Exec(`INSERT INTO samples (id, subject_id, source_platform, source_post_id,
			slide_index, file_path, sha256, phash, width, height, fetched_at, status, is_duplicate,
			created_at, updated_at)
			VALUES (?, ?, 'instagram', ?, 0, 'test.jpg', ?, '0', 100, 100, ?, 'kept', 0, ?, ?)`,
			sampleID, subjectID, sampleID, sampleID, now, now, now)
		Expect(err).NotTo(HaveOccurred())
	}

	seedStudy := func(projectID string) string {
		now := time.Now().UTC().Format(time.RFC3339)
		_, err := db.Exec(`INSERT INTO caption_studies (id, project_id, slug, name, provider, model, prompt_template, created_at, updated_at)
			VALUES ('study-1', ?, 'test-study', 'Test Study', 'manual', '', '', ?, ?)`, projectID, now, now)
		Expect(err).NotTo(HaveOccurred())
		return "study-1"
	}

	Describe("TriggerExport", func() {
		It("creates a job run and publishes messages for kept samples", func() {
			pid := seedProject()
			sid := seedSubject(pid)
			seedSample(sid, "sample-1")
			seedSample(sid, "sample-2")

			jr, err := exportSvc.TriggerExport(context.Background(), service.ExportParams{
				SubjectIDs: []string{sid},
				Format:     "kohya-ss",
			})

			Expect(err).NotTo(HaveOccurred())
			Expect(jr).NotTo(BeNil())
			Expect(jr.ID).NotTo(BeEmpty())
			Expect(jr.Status).To(Equal("running"))
			Expect(jr.TotalItems).NotTo(BeNil())
			Expect(*jr.TotalItems).To(Equal(2))
			Expect(jr.OutputPath).To(ContainSubstring("exports/test-project/test-subject/kohya-ss-"))
		})

		It("returns error when no subject_ids provided", func() {
			_, err := exportSvc.TriggerExport(context.Background(), service.ExportParams{
				Format: "kohya-ss",
			})
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("at least one subject_id"))
		})

		It("returns error for unsupported format", func() {
			pid := seedProject()
			sid := seedSubject(pid)
			seedSample(sid, "sample-1")

			_, err := exportSvc.TriggerExport(context.Background(), service.ExportParams{
				SubjectIDs: []string{sid},
				Format:     "unsupported",
			})
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("unsupported format"))
		})

		It("returns error when subject not found", func() {
			_, err := exportSvc.TriggerExport(context.Background(), service.ExportParams{
				SubjectIDs: []string{"nonexistent"},
				Format:     "kohya-ss",
			})
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("not found"))
		})

		It("returns error when no kept samples exist", func() {
			pid := seedProject()
			sid := seedSubject(pid)
			// No samples seeded.

			_, err := exportSvc.TriggerExport(context.Background(), service.ExportParams{
				SubjectIDs: []string{sid},
				Format:     "kohya-ss",
			})
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("no kept samples"))
		})

		It("validates study slug exists", func() {
			pid := seedProject()
			sid := seedSubject(pid)
			seedSample(sid, "sample-1")

			_, err := exportSvc.TriggerExport(context.Background(), service.ExportParams{
				SubjectIDs: []string{sid},
				StudySlug:  "nonexistent-study",
				Format:     "kohya-ss",
			})
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("study"))
		})

		It("accepts valid study slug", func() {
			pid := seedProject()
			sid := seedSubject(pid)
			seedSample(sid, "sample-1")
			seedStudy(pid)

			jr, err := exportSvc.TriggerExport(context.Background(), service.ExportParams{
				SubjectIDs: []string{sid},
				StudySlug:  "test-study",
				Format:     "kohya-ss",
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(jr).NotTo(BeNil())
		})

		It("uses custom destination when provided", func() {
			pid := seedProject()
			sid := seedSubject(pid)
			seedSample(sid, "sample-1")

			dest := filepath.Join(tmpDir, "custom-export")
			jr, err := exportSvc.TriggerExport(context.Background(), service.ExportParams{
				SubjectIDs:  []string{sid},
				Format:      "kohya-ss",
				Destination: dest,
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(jr.OutputPath).To(Equal(dest))
		})

		It("handles export with mixed edit/no-edit samples", func() {
			pid := seedProject()
			sid := seedSubject(pid)
			seedSample(sid, "sample-with-edit")
			seedSample(sid, "sample-no-edit")

			// Add an edit to sample-with-edit.
			now := time.Now().UTC().Format(time.RFC3339)
			_, err := db.Exec(`INSERT INTO edits (sample_id, rotation_deg, created_at, updated_at)
				VALUES ('sample-with-edit', 90, ?, ?)`, now, now)
			Expect(err).NotTo(HaveOccurred())

			jr, err := exportSvc.TriggerExport(context.Background(), service.ExportParams{
				SubjectIDs: []string{sid},
				Format:     "kohya-ss",
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(*jr.TotalItems).To(Equal(2))
		})
	})
})
