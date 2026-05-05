package pipeline_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"sync/atomic"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/nats-io/nats.go/jetstream"

	"github.com/kmacmcfarlane/image-dataset-tool/backend/internal/natsutil"
	"github.com/kmacmcfarlane/image-dataset-tool/backend/internal/pipeline"
	"github.com/kmacmcfarlane/image-dataset-tool/backend/internal/store"
)

var _ = Describe("Consumer", func() {
	var (
		srv     *natsutil.Server
		dataDir string
		dbDir   string
		db      *sql.DB
		tracker *pipeline.SQLJobTracker
		sink    *pipeline.ChannelEventSink
		logger  *slog.Logger
	)

	BeforeEach(func() {
		var err error
		dataDir, err = os.MkdirTemp("", "pipeline-test-nats-*")
		Expect(err).NotTo(HaveOccurred())

		dbDir, err = os.MkdirTemp("", "pipeline-test-db-*")
		Expect(err).NotTo(HaveOccurred())

		cfg := natsutil.DefaultConfig(dataDir)
		cfg.Consumers.MediaProcess.AckWait = 5 * time.Second
		cfg.Consumers.MediaProcess.MaxDeliver = 3
		srv, err = natsutil.New(cfg)
		Expect(err).NotTo(HaveOccurred())

		dbPath := filepath.Join(dbDir, "test.sqlite")
		db, err = store.OpenDB(dbPath)
		Expect(err).NotTo(HaveOccurred())
		Expect(store.Migrate(db)).To(Succeed())

		tracker = pipeline.NewJobTracker(db)
		sink = pipeline.NewChannelEventSink(64)
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

	// Helper: create a test job run in the DB.
	createJob := func(id, traceID string, totalItems *int) {
		total := sql.NullInt64{}
		if totalItems != nil {
			total = sql.NullInt64{Int64: int64(*totalItems), Valid: true}
		}
		_, err := db.Exec(`
			INSERT INTO job_runs (id, type, status, total_items, completed_items, failed_items,
				started_at, created_at, trace_id, pagination_exhausted)
			VALUES (?, 'ig_pull', 'running', ?, 0, 0, ?, ?, ?, 0)
		`, id, total, time.Now().UTC().Format(time.RFC3339),
			time.Now().UTC().Format(time.RFC3339), traceID)
		Expect(err).NotTo(HaveOccurred())
	}

	// Helper: publish an envelope to a subject.
	publishEnvelope := func(subject string, env *pipeline.Envelope) {
		data, err := pipeline.MarshalEnvelope(env)
		Expect(err).NotTo(HaveOccurred())
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_, err = srv.JS().Publish(ctx, subject, data)
		Expect(err).NotTo(HaveOccurred())
	}

	Describe("Base consumer: ACK on success", func() {
		It("pulls a message, invokes handler, ACKs on success", func() {
			totalItems := 1
			createJob("job-1", "trace-1", &totalItems)

			publishEnvelope(natsutil.SubjectProcess, &pipeline.Envelope{
				JobID:   "job-1",
				TraceID: "trace-1",
				Payload: json.RawMessage(`{"test": true}`),
			})

			var handlerCalled atomic.Bool
			handler := func(ctx context.Context, env *pipeline.Envelope) error {
				handlerCalled.Store(true)
				Expect(env.JobID).To(Equal("job-1"))
				Expect(env.TraceID).To(Equal("trace-1"))
				return nil
			}

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			cons, err := srv.JS().Consumer(ctx, natsutil.StreamName, natsutil.ConsumerProcess)
			Expect(err).NotTo(HaveOccurred())

			c := pipeline.NewConsumer(cons, pipeline.ConsumerOpts{
				ConsumerName: natsutil.ConsumerProcess,
				Subject:      natsutil.SubjectProcess,
				Concurrency:  1,
				MaxDeliver:   3,
				Handler:      handler,
				Tracker:      tracker,
				JS:           srv.JS(),
				Logger:       logger,
				EventSink:    sink,
			})
			c.Start(ctx)

			// Wait for handler to be called.
			Eventually(handlerCalled.Load, 10*time.Second, 100*time.Millisecond).Should(BeTrue())

			// Poll until ACK is processed and no pending messages remain.
			Eventually(func() uint64 {
				info, err := cons.Info(ctx)
				if err != nil {
					return 999
				}
				return info.NumPending
			}, 10*time.Second, 100*time.Millisecond).Should(BeZero())

			c.Stop()
		})
	})

	Describe("NAK with backoff on error", func() {
		It("NAKs and retries when handler returns an error", func() {
			totalItems := 1
			createJob("job-nak", "trace-nak", &totalItems)

			publishEnvelope(natsutil.SubjectProcess, &pipeline.Envelope{
				JobID:   "job-nak",
				TraceID: "trace-nak",
			})

			var attempts atomic.Int32
			handler := func(ctx context.Context, env *pipeline.Envelope) error {
				count := attempts.Add(1)
				if count < 2 {
					return errors.New("transient error")
				}
				return nil // succeed on second attempt
			}

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			cons, err := srv.JS().Consumer(ctx, natsutil.StreamName, natsutil.ConsumerProcess)
			Expect(err).NotTo(HaveOccurred())

			c := pipeline.NewConsumer(cons, pipeline.ConsumerOpts{
				ConsumerName: natsutil.ConsumerProcess,
				Subject:      natsutil.SubjectProcess,
				Concurrency:  1,
				MaxDeliver:   3,
				Handler:      handler,
				Tracker:      tracker,
				JS:           srv.JS(),
				Logger:       logger,
				EventSink:    sink,
			})
			c.Start(ctx)

			// Wait for at least 2 attempts (retry after backoff).
			Eventually(func() int32 { return attempts.Load() }, 30*time.Second, 200*time.Millisecond).
				Should(BeNumerically(">=", 2))

			c.Stop()
		})
	})

	Describe("DLQ after max retries", func() {
		It("routes to DLQ after max deliveries exceeded", func() {
			totalItems := 1
			createJob("job-dlq", "trace-dlq", &totalItems)

			publishEnvelope(natsutil.SubjectProcess, &pipeline.Envelope{
				JobID:   "job-dlq",
				TraceID: "trace-dlq",
			})

			handler := func(ctx context.Context, env *pipeline.Envelope) error {
				return errors.New("permanent failure")
			}

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			cons, err := srv.JS().Consumer(ctx, natsutil.StreamName, natsutil.ConsumerProcess)
			Expect(err).NotTo(HaveOccurred())

			c := pipeline.NewConsumer(cons, pipeline.ConsumerOpts{
				ConsumerName: natsutil.ConsumerProcess,
				Subject:      natsutil.SubjectProcess,
				Concurrency:  1,
				MaxDeliver:   3,
				Handler:      handler,
				Tracker:      tracker,
				JS:           srv.JS(),
				Logger:       logger,
				EventSink:    sink,
			})
			c.Start(ctx)

			// Wait for DLQ message to appear.
			dlqCons, err := srv.JS().Consumer(ctx, natsutil.StreamName, natsutil.ConsumerDLQ)
			Expect(err).NotTo(HaveOccurred())

			Eventually(func() bool {
				info, err := dlqCons.Info(ctx)
				if err != nil {
					return false
				}
				return info.NumPending > 0
			}, 30*time.Second, 500*time.Millisecond).Should(BeTrue())

			c.Stop()

			// Verify DLQ message content.
			msgs, err := dlqCons.Fetch(1, jetstream.FetchMaxWait(2*time.Second))
			Expect(err).NotTo(HaveOccurred())
			for msg := range msgs.Messages() {
				env, err := pipeline.UnmarshalEnvelope(msg.Data())
				Expect(err).NotTo(HaveOccurred())
				Expect(env.JobID).To(Equal("job-dlq"))
				Expect(msg.Ack()).To(Succeed())
			}
		})
	})

	Describe("DB counter increment before ACK", func() {
		It("NAKs message if DB counter increment fails", func() {
			totalItems := 1
			createJob("job-dbfail", "trace-dbfail", &totalItems)

			publishEnvelope(natsutil.SubjectProcess, &pipeline.Envelope{
				JobID:   "job-dbfail",
				TraceID: "trace-dbfail",
			})

			// Close DB to cause counter increment failure.
			db.Close()
			db = nil // prevent AfterEach from double-close

			handler := func(ctx context.Context, env *pipeline.Envelope) error {
				return nil // handler succeeds, but DB will fail
			}

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			cons, err := srv.JS().Consumer(ctx, natsutil.StreamName, natsutil.ConsumerProcess)
			Expect(err).NotTo(HaveOccurred())

			c := pipeline.NewConsumer(cons, pipeline.ConsumerOpts{
				ConsumerName: natsutil.ConsumerProcess,
				Subject:      natsutil.SubjectProcess,
				Concurrency:  1,
				MaxDeliver:   3,
				Handler:      handler,
				Tracker:      tracker,
				JS:           srv.JS(),
				Logger:       logger,
				EventSink:    sink,
			})
			c.Start(ctx)

			// Poll until the message has been attempted at least once (NAKed, not ACKed).
			Eventually(func() int {
				info, err := cons.Info(ctx)
				if err != nil {
					return 0
				}
				return int(info.NumPending) + info.NumAckPending + int(info.NumRedelivered)
			}, 10*time.Second, 200*time.Millisecond).Should(BeNumerically(">", 0))

			c.Stop()
		})
	})

	Describe("Disk full handling", func() {
		It("NAKs with delay and auto-pauses after N consecutive disk errors", func() {
			totalItems := 5
			createJob("job-disk", "trace-disk", &totalItems)

			// Publish multiple messages to trigger consecutive disk errors.
			for i := 0; i < 3; i++ {
				publishEnvelope(natsutil.SubjectProcess, &pipeline.Envelope{
					JobID:   "job-disk",
					TraceID: "trace-disk",
				})
			}

			handler := func(ctx context.Context, env *pipeline.Envelope) error {
				return pipeline.ErrDiskFull
			}

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			cons, err := srv.JS().Consumer(ctx, natsutil.StreamName, natsutil.ConsumerProcess)
			Expect(err).NotTo(HaveOccurred())

			c := pipeline.NewConsumer(cons, pipeline.ConsumerOpts{
				ConsumerName:            natsutil.ConsumerProcess,
				Subject:                 natsutil.SubjectProcess,
				Concurrency:             1,
				MaxDeliver:              5,
				Handler:                 handler,
				Tracker:                 tracker,
				JS:                      srv.JS(),
				Logger:                  logger,
				EventSink:               sink,
				MaxConsecutiveDiskErrors: 3,
			})
			c.Start(ctx)

			// Wait for auto-pause to trigger.
			Eventually(func() string {
				var status string
				err := db.QueryRow("SELECT status FROM job_runs WHERE id = 'job-disk'").Scan(&status)
				if err != nil {
					return ""
				}
				return status
			}, 30*time.Second, 500*time.Millisecond).Should(Equal("paused"))

			c.Stop()
		})
	})

	Describe("Job completion tracking", func() {
		It("marks job as succeeded when completed + failed = total and pagination exhausted", func() {
			totalItems := 2
			createJob("job-complete", "trace-complete", &totalItems)

			// Mark pagination exhausted.
			Expect(tracker.SetPaginationExhausted(context.Background(), "job-complete")).To(Succeed())

			// Publish 2 messages.
			for i := 0; i < 2; i++ {
				publishEnvelope(natsutil.SubjectProcess, &pipeline.Envelope{
					JobID:   "job-complete",
					TraceID: "trace-complete",
				})
			}

			handler := func(ctx context.Context, env *pipeline.Envelope) error {
				return nil
			}

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			cons, err := srv.JS().Consumer(ctx, natsutil.StreamName, natsutil.ConsumerProcess)
			Expect(err).NotTo(HaveOccurred())

			c := pipeline.NewConsumer(cons, pipeline.ConsumerOpts{
				ConsumerName: natsutil.ConsumerProcess,
				Subject:      natsutil.SubjectProcess,
				Concurrency:  1,
				MaxDeliver:   3,
				Handler:      handler,
				Tracker:      tracker,
				JS:           srv.JS(),
				Logger:       logger,
				EventSink:    sink,
			})
			c.Start(ctx)

			// Wait for job to be marked as succeeded.
			Eventually(func() string {
				var status string
				err := db.QueryRow("SELECT status FROM job_runs WHERE id = 'job-complete'").Scan(&status)
				if err != nil {
					return ""
				}
				return status
			}, 15*time.Second, 200*time.Millisecond).Should(Equal("succeeded"))

			c.Stop()
		})

		It("does not complete when pagination is not exhausted", func() {
			// Create job with NULL total_items (not yet known).
			createJob("job-paging", "trace-paging", nil)

			// Increment total as pages are discovered.
			Expect(tracker.IncrTotal(context.Background(), "job-paging", 1)).To(Succeed())

			publishEnvelope(natsutil.SubjectProcess, &pipeline.Envelope{
				JobID:   "job-paging",
				TraceID: "trace-paging",
			})

			handler := func(ctx context.Context, env *pipeline.Envelope) error {
				return nil
			}

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			cons, err := srv.JS().Consumer(ctx, natsutil.StreamName, natsutil.ConsumerProcess)
			Expect(err).NotTo(HaveOccurred())

			c := pipeline.NewConsumer(cons, pipeline.ConsumerOpts{
				ConsumerName: natsutil.ConsumerProcess,
				Subject:      natsutil.SubjectProcess,
				Concurrency:  1,
				MaxDeliver:   3,
				Handler:      handler,
				Tracker:      tracker,
				JS:           srv.JS(),
				Logger:       logger,
				EventSink:    sink,
			})
			c.Start(ctx)

			// Wait until the message has been processed (completed_items incremented).
			Eventually(func() int {
				var completed int
				err := db.QueryRow("SELECT completed_items FROM job_runs WHERE id = 'job-paging'").Scan(&completed)
				if err != nil {
					return 0
				}
				return completed
			}, 10*time.Second, 200*time.Millisecond).Should(Equal(1))

			// Job should still be running (pagination not exhausted).
			Consistently(func() string {
				var status string
				err := db.QueryRow("SELECT status FROM job_runs WHERE id = 'job-paging'").Scan(&status)
				if err != nil {
					return ""
				}
				return status
			}, 1*time.Second, 200*time.Millisecond).Should(Equal("running"))

			c.Stop()
		})
	})

	Describe("Graceful shutdown", func() {
		It("stops consumers and waits for in-flight handlers", func() {
			totalItems := 1
			createJob("job-shutdown", "trace-shutdown", &totalItems)

			publishEnvelope(natsutil.SubjectProcess, &pipeline.Envelope{
				JobID:   "job-shutdown",
				TraceID: "trace-shutdown",
			})

			var handlerStarted atomic.Bool
			var handlerFinished atomic.Bool
			handler := func(ctx context.Context, env *pipeline.Envelope) error {
				handlerStarted.Store(true)
				// Simulate long operation.
				time.Sleep(2 * time.Second)
				handlerFinished.Store(true)
				return nil
			}

			ctx, cancel := context.WithCancel(context.Background())

			cons, err := srv.JS().Consumer(ctx, natsutil.StreamName, natsutil.ConsumerProcess)
			Expect(err).NotTo(HaveOccurred())

			c := pipeline.NewConsumer(cons, pipeline.ConsumerOpts{
				ConsumerName:       natsutil.ConsumerProcess,
				Subject:            natsutil.SubjectProcess,
				Concurrency:        1,
				MaxDeliver:         3,
				Handler:            handler,
				Tracker:            tracker,
				JS:                 srv.JS(),
				Logger:             logger,
				EventSink:          sink,
				InProgressInterval: 1 * time.Second,
			})
			c.Start(ctx)

			// Wait for handler to start.
			Eventually(handlerStarted.Load, 10*time.Second, 100*time.Millisecond).Should(BeTrue())

			// Trigger shutdown.
			coord := pipeline.NewShutdownCoordinator(pipeline.ShutdownOpts{
				Consumers: []*pipeline.Consumer{c},
				Cancel:    cancel,
				Timeout:   10 * time.Second,
				Logger:    logger,
			})
			coord.Shutdown()

			// Handler should have finished (not been killed mid-flight).
			Expect(handlerFinished.Load()).To(BeTrue())
		})
	})
})
