package pipeline_test

import (
	"context"
	"database/sql"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/kmacmcfarlane/image-dataset-tool/backend/internal/natsutil"
	"github.com/kmacmcfarlane/image-dataset-tool/backend/internal/pipeline"
	"github.com/kmacmcfarlane/image-dataset-tool/backend/internal/store"
)

var _ = Describe("Stale Job Detection", func() {
	var (
		srv     *natsutil.Server
		dataDir string
		dbDir   string
		db      *sql.DB
		tracker *pipeline.SQLJobTracker
		logger  *slog.Logger
	)

	BeforeEach(func() {
		var err error
		dataDir, err = os.MkdirTemp("", "stale-test-nats-*")
		Expect(err).NotTo(HaveOccurred())

		dbDir, err = os.MkdirTemp("", "stale-test-db-*")
		Expect(err).NotTo(HaveOccurred())

		cfg := natsutil.DefaultConfig(dataDir)
		srv, err = natsutil.New(cfg)
		Expect(err).NotTo(HaveOccurred())

		dbPath := filepath.Join(dbDir, "test.sqlite")
		db, err = store.OpenDB(dbPath)
		Expect(err).NotTo(HaveOccurred())
		Expect(store.Migrate(db)).To(Succeed())

		tracker = pipeline.NewJobTracker(db)
		logger = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))
	})

	AfterEach(func() {
		if db != nil {
			db.Close()
		}
		if srv != nil {
			srv.Shutdown()
		}
		os.RemoveAll(dataDir)
		os.RemoveAll(dbDir)
	})

	It("marks running jobs as interrupted when no pending NATS messages", func() {
		// Create a running job with no pending messages.
		now := time.Now().UTC().Format(time.RFC3339)
		_, err := db.Exec(`
			INSERT INTO job_runs (id, type, status, total_items, completed_items, failed_items,
				started_at, created_at, trace_id, pagination_exhausted)
			VALUES ('stale-1', 'ig_pull', 'running', 10, 5, 0, ?, ?, 'trace-stale-1', 0)
		`, now, now)
		Expect(err).NotTo(HaveOccurred())

		ctx := context.Background()
		err = pipeline.DetectStaleJobs(ctx, tracker, srv.JS(), logger)
		Expect(err).NotTo(HaveOccurred())

		// Job should now be 'interrupted'.
		var status string
		err = db.QueryRow("SELECT status FROM job_runs WHERE id = 'stale-1'").Scan(&status)
		Expect(err).NotTo(HaveOccurred())
		Expect(status).To(Equal("interrupted"))
	})

	It("leaves running jobs alone when there are pending NATS messages", func() {
		now := time.Now().UTC().Format(time.RFC3339)
		_, err := db.Exec(`
			INSERT INTO job_runs (id, type, status, total_items, completed_items, failed_items,
				started_at, created_at, trace_id, pagination_exhausted)
			VALUES ('active-1', 'ig_pull', 'running', 10, 5, 0, ?, ?, 'trace-active-1', 0)
		`, now, now)
		Expect(err).NotTo(HaveOccurred())

		// Publish a message so there are pending messages.
		ctx := context.Background()
		data, _ := pipeline.MarshalEnvelope(&pipeline.Envelope{
			JobID:   "active-1",
			TraceID: "trace-active-1",
		})
		_, err = srv.JS().Publish(ctx, natsutil.SubjectProcess, data)
		Expect(err).NotTo(HaveOccurred())

		err = pipeline.DetectStaleJobs(ctx, tracker, srv.JS(), logger)
		Expect(err).NotTo(HaveOccurred())

		// Job should still be 'running'.
		var status string
		err = db.QueryRow("SELECT status FROM job_runs WHERE id = 'active-1'").Scan(&status)
		Expect(err).NotTo(HaveOccurred())
		Expect(status).To(Equal("running"))
	})

	It("does nothing when there are no running jobs", func() {
		ctx := context.Background()
		err := pipeline.DetectStaleJobs(ctx, tracker, srv.JS(), logger)
		Expect(err).NotTo(HaveOccurred())
	})
})
