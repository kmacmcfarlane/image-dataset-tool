package store_test

import (
	"database/sql"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/kmacmcfarlane/image-dataset-tool/backend/internal/store"
)

var _ = Describe("JobStore", func() {
	var (
		tmpDir string
		db     *sql.DB
		js     *store.JobStore
	)

	BeforeEach(func() {
		var err error
		tmpDir, err = os.MkdirTemp("", "job-store-test-*")
		Expect(err).NotTo(HaveOccurred())

		dbPath := filepath.Join(tmpDir, "test.sqlite")
		db, err = store.OpenDB(dbPath)
		Expect(err).NotTo(HaveOccurred())

		err = store.Migrate(db)
		Expect(err).NotTo(HaveOccurred())

		js = store.NewJobStore(db)

		// Seed a job run and messages.
		_, err = db.Exec(`
			INSERT INTO job_runs (id, type, status, completed_items, failed_items, started_at, created_at)
			VALUES
				('job-1', 'sample_collection', 'succeeded', 10, 1, '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z'),
				('job-2', 'captioning',        'running',  5,  0, '2026-01-02T00:00:00Z', '2026-01-02T00:00:00Z'),
				('job-3', 'sample_collection', 'failed',   3,  3, '2026-01-03T00:00:00Z', '2026-01-03T00:00:00Z')
		`)
		Expect(err).NotTo(HaveOccurred())

		_, err = db.Exec(`
			INSERT INTO job_messages (id, job_run_id, level, message, created_at) VALUES
				('m-1', 'job-1', 'info',    'Job started',        '2026-01-01T00:00:01Z'),
				('m-2', 'job-1', 'warning', 'Rate limit hit',     '2026-01-01T00:00:02Z'),
				('m-3', 'job-1', 'error',   'Download failed',    '2026-01-01T00:00:03Z'),
				('m-4', 'job-2', 'info',    'Captioning started', '2026-01-02T00:00:01Z')
		`)
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		db.Close()
		os.RemoveAll(tmpDir)
	})

	Describe("ListJobs", func() {
		It("returns all jobs ordered by created_at desc", func() {
			jobs, total, err := js.ListJobs(store.JobListFilter{Limit: 20})
			Expect(err).NotTo(HaveOccurred())
			Expect(total).To(Equal(int64(3)))
			Expect(jobs).To(HaveLen(3))
			Expect(jobs[0].ID).To(Equal("job-3")) // newest first
		})

		It("filters by type", func() {
			jobs, total, err := js.ListJobs(store.JobListFilter{Type: "captioning", Limit: 20})
			Expect(err).NotTo(HaveOccurred())
			Expect(total).To(Equal(int64(1)))
			Expect(jobs).To(HaveLen(1))
			Expect(jobs[0].ID).To(Equal("job-2"))
		})

		It("filters by status", func() {
			jobs, total, err := js.ListJobs(store.JobListFilter{Status: "running", Limit: 20})
			Expect(err).NotTo(HaveOccurred())
			Expect(total).To(Equal(int64(1)))
			Expect(jobs[0].Status).To(Equal("running"))
		})

		It("paginates correctly", func() {
			jobs, total, err := js.ListJobs(store.JobListFilter{Limit: 2, Offset: 0})
			Expect(err).NotTo(HaveOccurred())
			Expect(total).To(Equal(int64(3)))
			Expect(jobs).To(HaveLen(2))

			jobs2, _, err := js.ListJobs(store.JobListFilter{Limit: 2, Offset: 2})
			Expect(err).NotTo(HaveOccurred())
			Expect(jobs2).To(HaveLen(1))
		})
	})

	Describe("GetJob", func() {
		It("returns a job by ID", func() {
			jr, err := js.GetJob("job-1")
			Expect(err).NotTo(HaveOccurred())
			Expect(jr.ID).To(Equal("job-1"))
			Expect(jr.Type).To(Equal("sample_collection"))
			Expect(jr.Status).To(Equal("succeeded"))
			Expect(jr.CompletedItems).To(Equal(10))
			Expect(jr.FailedItems).To(Equal(1))
		})

		It("returns ErrNotFound for unknown ID", func() {
			_, err := js.GetJob("nonexistent")
			Expect(err).To(MatchError(ContainSubstring("not found")))
		})
	})

	Describe("ListJobMessages", func() {
		It("returns all messages for a job", func() {
			msgs, total, err := js.ListJobMessages(store.JobMessageFilter{
				JobRunID: "job-1",
				Limit:    50,
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(total).To(Equal(int64(3)))
			Expect(msgs).To(HaveLen(3))
		})

		It("filters warning level (returns warning + error)", func() {
			msgs, total, err := js.ListJobMessages(store.JobMessageFilter{
				JobRunID: "job-1",
				Level:    "warning",
				Limit:    50,
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(total).To(Equal(int64(2)))
			Expect(msgs).To(HaveLen(2))
			levels := []string{msgs[0].Level, msgs[1].Level}
			Expect(levels).To(ContainElements("warning", "error"))
		})

		It("filters error level (returns only errors)", func() {
			msgs, total, err := js.ListJobMessages(store.JobMessageFilter{
				JobRunID: "job-1",
				Level:    "error",
				Limit:    50,
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(total).To(Equal(int64(1)))
			Expect(msgs[0].Level).To(Equal("error"))
		})

		It("returns ErrNotFound for unknown job", func() {
			_, _, err := js.ListJobMessages(store.JobMessageFilter{
				JobRunID: "nonexistent",
				Limit:    50,
			})
			Expect(err).To(MatchError(ContainSubstring("not found")))
		})

		It("returns messages ordered by created_at asc", func() {
			msgs, _, err := js.ListJobMessages(store.JobMessageFilter{
				JobRunID: "job-1",
				Limit:    50,
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(msgs[0].ID).To(Equal("m-1"))
			Expect(msgs[2].ID).To(Equal("m-3"))
		})
	})
})
