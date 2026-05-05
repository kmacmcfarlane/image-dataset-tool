package pipeline

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/nats-io/nats.go/jetstream"

	"github.com/kmacmcfarlane/image-dataset-tool/backend/internal/natsutil"
)

// ErrDiskFull is returned by handlers when the filesystem has no space.
var ErrDiskFull = errors.New("disk full (ENOSPC)")

// Handler processes a single pipeline message. Implementations are stage-specific
// (fetch, process, caption, export). The handler receives the deserialized envelope
// and must return nil on success. Returning ErrDiskFull triggers disk-error handling.
type Handler func(ctx context.Context, env *Envelope) error

// ConsumerOpts configures a pipeline consumer.
type ConsumerOpts struct {
	// ConsumerName is the NATS durable consumer name (e.g. "media-fetch-instagram").
	ConsumerName string

	// Subject is the NATS subject this consumer filters on (for logging).
	Subject string

	// Concurrency is the number of parallel worker goroutines.
	Concurrency int

	// MaxDeliver from the consumer config, used for DLQ routing.
	MaxDeliver int

	// Handler processes each message.
	Handler Handler

	// Provider is the external provider name for rate limiting (empty if none).
	Provider string

	// RateLimiters is the shared provider rate limiter registry.
	RateLimiters *ProviderLimiters

	// Tracker manages job counters in the DB.
	Tracker JobTracker

	// JS is the JetStream interface for DLQ publishing.
	JS jetstream.JetStream

	// Logger is the structured logger for this consumer.
	Logger *slog.Logger

	// EventSink receives SSE events (consumer.stats, job.progress, etc.).
	EventSink EventSink

	// MaxConsecutiveDiskErrors is the number of consecutive disk errors before auto-pausing.
	// Default: 3.
	MaxConsecutiveDiskErrors int

	// InProgressInterval is how often to send msg.InProgress() for long operations.
	// Default: half of the consumer's AckWait.
	InProgressInterval time.Duration
}

// Consumer pulls messages from a NATS JetStream durable consumer and
// dispatches them to a Handler with retry, DLQ, rate limiting, and
// structured logging.
type Consumer struct {
	opts     ConsumerOpts
	consumer jetstream.Consumer
	cancel   context.CancelFunc
	wg       sync.WaitGroup
	stopped  atomic.Bool

	// consecutiveDiskErrors tracks ENOSPC errors for auto-pause.
	// This counter is per-Consumer (not per-job) intentionally: disk full is a
	// system-level condition that affects all jobs on this consumer equally.
	consecutiveDiskErrors atomic.Int32
}

// NewConsumer creates a consumer but does not start it.
// Call Start() to begin pulling messages.
func NewConsumer(consumer jetstream.Consumer, opts ConsumerOpts) *Consumer {
	if opts.Concurrency <= 0 {
		opts.Concurrency = 1
	}
	if opts.MaxConsecutiveDiskErrors <= 0 {
		opts.MaxConsecutiveDiskErrors = 3
	}
	if opts.Logger == nil {
		opts.Logger = slog.Default()
	}
	return &Consumer{
		opts:     opts,
		consumer: consumer,
	}
}

// Start begins pulling messages with the configured concurrency.
// It is non-blocking; workers run in background goroutines.
// Call Stop() to shut down gracefully.
func (c *Consumer) Start(ctx context.Context) {
	ctx, c.cancel = context.WithCancel(ctx)

	c.opts.Logger.Info("consumer starting",
		slog.String("consumer", c.opts.ConsumerName),
		slog.String("subject", c.opts.Subject),
		slog.Int("concurrency", c.opts.Concurrency),
	)

	for i := 0; i < c.opts.Concurrency; i++ {
		c.wg.Add(1)
		go c.worker(ctx, i)
	}
}

// Stop signals all workers to stop and waits for in-flight handlers to finish.
func (c *Consumer) Stop() {
	if c.stopped.Swap(true) {
		return // already stopped
	}
	if c.cancel != nil {
		c.cancel()
	}
	c.wg.Wait()
	c.opts.Logger.Info("consumer stopped",
		slog.String("consumer", c.opts.ConsumerName),
	)
}

// worker is the main loop for a single worker goroutine.
func (c *Consumer) worker(ctx context.Context, workerID int) {
	defer c.wg.Done()

	log := c.opts.Logger.With(
		slog.String("consumer", c.opts.ConsumerName),
		slog.Int("worker", workerID),
	)

	for {
		if ctx.Err() != nil {
			return
		}

		// Fetch one message at a time with a short wait.
		msgs, err := c.consumer.Fetch(1, jetstream.FetchMaxWait(2*time.Second))
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			if errors.Is(err, jetstream.ErrNoMessages) {
				continue
			}
			log.Warn("fetch error", slog.String("error", err.Error()))
			continue
		}

		for msg := range msgs.Messages() {
			c.handleMessage(ctx, log, msg)
		}

		if fetchErr := msgs.Error(); fetchErr != nil {
			if errors.Is(fetchErr, jetstream.ErrNoMessages) {
				continue
			}
			if ctx.Err() != nil {
				return
			}
			log.Warn("fetch batch error", slog.String("error", fetchErr.Error()))
		}
	}
}

// handleMessage processes a single message with DLQ check, rate limiting,
// InProgress keepalive, structured logging, and error handling.
func (c *Consumer) handleMessage(ctx context.Context, log *slog.Logger, msg jetstream.Msg) {
	start := time.Now()

	// Parse metadata for attempt count.
	md, err := msg.Metadata()
	attempt := uint64(1)
	if err == nil {
		attempt = md.NumDelivered
	}

	// Deserialize envelope.
	env, err := UnmarshalEnvelope(msg.Data())
	if err != nil {
		log.Error("unmarshal envelope failed",
			slog.String("error", err.Error()),
			slog.String("subject", msg.Subject()),
			slog.Uint64("attempt", attempt),
		)
		// Bad data: ACK to remove (can't be retried).
		_ = msg.Ack()
		return
	}

	msgLog := log.With(
		slog.String("job_id", env.JobID),
		slog.String("trace_id", env.TraceID),
		slog.String("subject", msg.Subject()),
		slog.Uint64("attempt", attempt),
		slog.String("sample_id", env.SampleID),
	)

	// DLQ check: if max deliveries reached, route to DLQ.
	if natsutil.ShouldDLQ(msg, c.opts.MaxDeliver) {
		msgLog.Warn("max deliveries reached, routing to DLQ")
		if dlqErr := natsutil.RouteToDLQ(ctx, c.opts.JS, msg); dlqErr != nil {
			msgLog.Error("failed to route to DLQ", slog.String("error", dlqErr.Error()))
			return // will be redelivered by NATS
		}
		// Increment failed count in job tracker.
		if c.opts.Tracker != nil {
			if _, _, _, tErr := c.opts.Tracker.IncrFailed(ctx, env.JobID); tErr != nil {
				msgLog.Error("failed to increment failed counter", slog.String("error", tErr.Error()))
			} else {
				c.checkAndEmitCompletion(ctx, msgLog, env)
			}
		}
		c.logDuration(msgLog, start, "dlq")
		return
	}

	// Rate limiting: wait for provider rate limiter.
	if c.opts.Provider != "" && c.opts.RateLimiters != nil {
		if rlErr := c.opts.RateLimiters.Wait(ctx, c.opts.Provider); rlErr != nil {
			if ctx.Err() != nil {
				return // shutting down
			}
			msgLog.Error("rate limiter error", slog.String("error", rlErr.Error()))
			_ = msg.NakWithDelay(5 * time.Second)
			return
		}
	}

	// Start InProgress keepalive for long operations.
	ipCtx, ipCancel := context.WithCancel(ctx)
	defer ipCancel()
	c.startInProgress(ipCtx, msg)

	// Invoke the handler.
	handlerErr := c.opts.Handler(ctx, env)

	// Stop InProgress keepalive.
	ipCancel()

	duration := time.Since(start)

	if handlerErr != nil {
		c.handleError(ctx, msgLog, msg, env, handlerErr, duration)
		return
	}

	// Success: increment DB counter, then ACK.
	if c.opts.Tracker != nil {
		_, _, _, tErr := c.opts.Tracker.IncrCompleted(ctx, env.JobID)
		if tErr != nil {
			// DB failed — NAK so the message is retried.
			msgLog.Error("DB counter increment failed, NAKing",
				slog.String("error", tErr.Error()),
			)
			_ = msg.NakWithDelay(2 * time.Second)
			c.logDuration(msgLog, start, "nak_db_error")
			return
		}
	}

	// ACK the message.
	if ackErr := msg.Ack(); ackErr != nil {
		msgLog.Error("ACK failed", slog.String("error", ackErr.Error()))
	}

	// Reset consecutive disk errors on success.
	c.consecutiveDiskErrors.Store(0)

	c.logDuration(msgLog, start, "ack")

	// Check for job completion.
	if c.opts.Tracker != nil {
		c.checkAndEmitCompletion(ctx, msgLog, env)
	}
}

// handleError handles a handler error: disk full detection, NAK with backoff.
func (c *Consumer) handleError(ctx context.Context, log *slog.Logger, msg jetstream.Msg, env *Envelope, handlerErr error, duration time.Duration) {
	status := "nak"

	if errors.Is(handlerErr, ErrDiskFull) {
		count := c.consecutiveDiskErrors.Add(1)
		log.Error("disk full error",
			slog.String("error", handlerErr.Error()),
			slog.Int("consecutive_disk_errors", int(count)),
		)

		if int(count) >= c.opts.MaxConsecutiveDiskErrors && c.opts.Tracker != nil {
			log.Error("auto-pausing job due to consecutive disk errors",
				slog.String("job_id", env.JobID),
			)
			if pErr := c.opts.Tracker.SetStatus(ctx, env.JobID, JobStatusPaused); pErr != nil {
				log.Error("failed to pause job", slog.String("error", pErr.Error()))
			}
		}

		status = "nak_enospc"
		_ = msg.NakWithDelay(30 * time.Second)
	} else {
		c.consecutiveDiskErrors.Store(0) // reset on non-disk error

		// Exponential backoff: 2^attempt seconds, capped at 60s.
		var numDelivered uint64 = 1
		if md, err := msg.Metadata(); err == nil && md != nil {
			numDelivered = md.NumDelivered
		}
		backoff := time.Duration(1<<min(numDelivered, 6)) * time.Second
		if backoff > 60*time.Second {
			backoff = 60 * time.Second
		}

		log.Error("handler error",
			slog.String("error", handlerErr.Error()),
			slog.Duration("backoff", backoff),
		)
		_ = msg.NakWithDelay(backoff)
	}

	c.logDuration(log, time.Now().Add(-duration), status)
}

// startInProgress periodically sends msg.InProgress() to prevent AckWait redelivery.
func (c *Consumer) startInProgress(ctx context.Context, msg jetstream.Msg) {
	interval := c.opts.InProgressInterval
	if interval <= 0 {
		interval = 15 * time.Second // default fallback
	}

	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := msg.InProgress(); err != nil {
					return // message likely already ACKed/NAKed
				}
			}
		}
	}()
}

// checkAndEmitCompletion checks if the job is complete and emits events.
func (c *Consumer) checkAndEmitCompletion(ctx context.Context, log *slog.Logger, env *Envelope) {
	newStatus, err := c.opts.Tracker.CheckCompletion(ctx, env.JobID)
	if err != nil {
		log.Error("check completion failed", slog.String("error", err.Error()))
		return
	}
	if newStatus != "" && c.opts.EventSink != nil {
		c.opts.EventSink.Emit(SSEEvent{
			Type: "job.state",
			Data: map[string]interface{}{
				"id":       env.JobID,
				"trace_id": env.TraceID,
				"status":   string(newStatus),
			},
		})
	}
}

// logDuration emits the structured log line per AC.
func (c *Consumer) logDuration(log *slog.Logger, start time.Time, status string) {
	log.Info("message processed",
		slog.Int64("duration_ms", time.Since(start).Milliseconds()),
		slog.String("status", status),
	)
}
