package pipeline_test

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/kmacmcfarlane/image-dataset-tool/backend/internal/pipeline"
	"github.com/kmacmcfarlane/image-dataset-tool/backend/internal/store"
)

var _ = Describe("JobTracker", func() {
	var (
		db      *sql.DB
		dbDir   string
		tracker *pipeline.SQLJobTracker
	)

	BeforeEach(func() {
		var err error
		dbDir, err = os.MkdirTemp("", "tracker-test-*")
		Expect(err).NotTo(HaveOccurred())

		dbPath := filepath.Join(dbDir, "test.sqlite")
		db, err = store.OpenDB(dbPath)
		Expect(err).NotTo(HaveOccurred())
		Expect(store.Migrate(db)).To(Succeed())

		tracker = pipeline.NewJobTracker(db)
	})

	AfterEach(func() {
		if db != nil {
			db.Close()
		}
		os.RemoveAll(dbDir)
	})

	createJob := func(id string, totalItems *int, paginationExhausted bool) {
		total := sql.NullInt64{}
		if totalItems != nil {
			total = sql.NullInt64{Int64: int64(*totalItems), Valid: true}
		}
		pe := 0
		if paginationExhausted {
			pe = 1
		}
		now := time.Now().UTC().Format(time.RFC3339)
		_, err := db.Exec(`
			INSERT INTO job_runs (id, type, status, total_items, completed_items, failed_items,
				started_at, created_at, trace_id, pagination_exhausted)
			VALUES (?, 'ig_pull', 'running', ?, 0, 0, ?, ?, 'trace-1', ?)
		`, id, total, now, now, pe)
		Expect(err).NotTo(HaveOccurred())
	}

	Describe("IncrCompleted", func() {
		It("atomically increments completed_items", func() {
			total := 5
			createJob("j1", &total, false)

			ctx := context.Background()
			completed, failed, totalOut, err := tracker.IncrCompleted(ctx, "j1")
			Expect(err).NotTo(HaveOccurred())
			Expect(completed).To(Equal(1))
			Expect(failed).To(Equal(0))
			Expect(totalOut).To(Equal(5))

			completed, _, _, err = tracker.IncrCompleted(ctx, "j1")
			Expect(err).NotTo(HaveOccurred())
			Expect(completed).To(Equal(2))
		})
	})

	Describe("IncrFailed", func() {
		It("atomically increments failed_items", func() {
			total := 3
			createJob("j2", &total, false)

			ctx := context.Background()
			_, failed, _, err := tracker.IncrFailed(ctx, "j2")
			Expect(err).NotTo(HaveOccurred())
			Expect(failed).To(Equal(1))
		})
	})

	Describe("IncrTotal", func() {
		It("increments total_items from NULL", func() {
			createJob("j3", nil, false)

			ctx := context.Background()
			Expect(tracker.IncrTotal(ctx, "j3", 5)).To(Succeed())

			var total int
			err := db.QueryRow("SELECT total_items FROM job_runs WHERE id = 'j3'").Scan(&total)
			Expect(err).NotTo(HaveOccurred())
			Expect(total).To(Equal(5))

			Expect(tracker.IncrTotal(ctx, "j3", 3)).To(Succeed())
			err = db.QueryRow("SELECT total_items FROM job_runs WHERE id = 'j3'").Scan(&total)
			Expect(err).NotTo(HaveOccurred())
			Expect(total).To(Equal(8))
		})
	})

	Describe("CheckCompletion", func() {
		It("marks job succeeded when completed + failed = total AND pagination exhausted", func() {
			total := 2
			createJob("j4", &total, true) // pagination already exhausted

			ctx := context.Background()
			tracker.IncrCompleted(ctx, "j4")
			tracker.IncrCompleted(ctx, "j4")

			status, err := tracker.CheckCompletion(ctx, "j4")
			Expect(err).NotTo(HaveOccurred())
			Expect(status).To(Equal(pipeline.JobStatusSucceeded))

			// Verify in DB.
			var dbStatus string
			err = db.QueryRow("SELECT status FROM job_runs WHERE id = 'j4'").Scan(&dbStatus)
			Expect(err).NotTo(HaveOccurred())
			Expect(dbStatus).To(Equal("succeeded"))
		})

		It("returns empty status when not yet complete", func() {
			total := 3
			createJob("j5", &total, true)

			ctx := context.Background()
			tracker.IncrCompleted(ctx, "j5") // 1 of 3

			status, err := tracker.CheckCompletion(ctx, "j5")
			Expect(err).NotTo(HaveOccurred())
			Expect(status).To(Equal(pipeline.JobStatus("")))
		})

		It("returns empty status when pagination not exhausted", func() {
			total := 1
			createJob("j6", &total, false) // pagination NOT exhausted

			ctx := context.Background()
			tracker.IncrCompleted(ctx, "j6")

			status, err := tracker.CheckCompletion(ctx, "j6")
			Expect(err).NotTo(HaveOccurred())
			Expect(status).To(Equal(pipeline.JobStatus("")))
		})

		It("marks job failed when all items failed", func() {
			total := 2
			createJob("j7", &total, true)

			ctx := context.Background()
			tracker.IncrFailed(ctx, "j7")
			tracker.IncrFailed(ctx, "j7")

			status, err := tracker.CheckCompletion(ctx, "j7")
			Expect(err).NotTo(HaveOccurred())
			Expect(status).To(Equal(pipeline.JobStatusFailed))
		})
	})

	Describe("GetRunningJobs", func() {
		It("returns only running jobs", func() {
			total := 1
			createJob("running-1", &total, false)

			// Create a succeeded job.
			now := time.Now().UTC().Format(time.RFC3339)
			_, err := db.Exec(`
				INSERT INTO job_runs (id, type, status, total_items, completed_items, failed_items,
					started_at, created_at, trace_id, pagination_exhausted)
				VALUES ('done-1', 'ig_pull', 'succeeded', 1, 1, 0, ?, ?, 'trace-done', 0)
			`, now, now)
			Expect(err).NotTo(HaveOccurred())

			ctx := context.Background()
			jobs, err := tracker.GetRunningJobs(ctx)
			Expect(err).NotTo(HaveOccurred())
			Expect(jobs).To(HaveLen(1))
			Expect(jobs[0].ID).To(Equal("running-1"))
		})
	})

	Describe("SetStatus", func() {
		It("updates status and sets finished_at for terminal states", func() {
			total := 1
			createJob("j-status", &total, false)

			ctx := context.Background()
			Expect(tracker.SetStatus(ctx, "j-status", pipeline.JobStatusCancelled)).To(Succeed())

			var status string
			var finishedAt sql.NullString
			err := db.QueryRow("SELECT status, finished_at FROM job_runs WHERE id = 'j-status'").
				Scan(&status, &finishedAt)
			Expect(err).NotTo(HaveOccurred())
			Expect(status).To(Equal("cancelled"))
			Expect(finishedAt.Valid).To(BeTrue())
		})

		It("does not set finished_at for non-terminal states", func() {
			total := 1
			createJob("j-pause", &total, false)

			ctx := context.Background()
			Expect(tracker.SetStatus(ctx, "j-pause", pipeline.JobStatusPaused)).To(Succeed())

			var finishedAt sql.NullString
			err := db.QueryRow("SELECT finished_at FROM job_runs WHERE id = 'j-pause'").Scan(&finishedAt)
			Expect(err).NotTo(HaveOccurred())
			Expect(finishedAt.Valid).To(BeFalse())
		})
	})
})
