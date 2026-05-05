package pipeline

import (
	"context"
	"database/sql"
	"log/slog"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/kmacmcfarlane/image-dataset-tool/backend/internal/natsutil"
)

// ShutdownCoordinator manages the graceful shutdown sequence:
// 1. Cancel root context (signals goroutines to stop)
// 2. Stop consumers (no new message pulls)
// 3. Wait for in-flight handlers to finish
// 4. Stop SSE emitter
// 5. Shutdown NATS (flush JetStream to disk)
// 6. Close SQLite (WAL checkpoint)
type ShutdownCoordinator struct {
	consumers    []*Consumer
	statsEmitter *ConsumerStatsEmitter
	natsSrv      *natsutil.Server
	db           *sql.DB
	cancel       context.CancelFunc
	timeout      time.Duration
	logger       *slog.Logger
}

// ShutdownOpts configures the shutdown coordinator.
type ShutdownOpts struct {
	Consumers    []*Consumer
	StatsEmitter *ConsumerStatsEmitter
	NATSSrv      *natsutil.Server
	DB           *sql.DB
	Cancel       context.CancelFunc
	Timeout      time.Duration
	Logger       *slog.Logger
}

// NewShutdownCoordinator creates a coordinator.
func NewShutdownCoordinator(opts ShutdownOpts) *ShutdownCoordinator {
	if opts.Timeout <= 0 {
		opts.Timeout = 30 * time.Second
	}
	if opts.Logger == nil {
		opts.Logger = slog.Default()
	}
	return &ShutdownCoordinator{
		consumers:    opts.Consumers,
		statsEmitter: opts.StatsEmitter,
		natsSrv:      opts.NATSSrv,
		db:           opts.DB,
		cancel:       opts.Cancel,
		timeout:      opts.Timeout,
		logger:       opts.Logger,
	}
}

// WaitForSignal blocks until SIGTERM or SIGINT is received, then runs
// the shutdown sequence. Returns after shutdown completes or timeout.
func (sc *ShutdownCoordinator) WaitForSignal() {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)
	sig := <-sigCh
	sc.logger.Info("received signal, starting graceful shutdown",
		slog.String("signal", sig.String()),
	)
	sc.Shutdown()
}

// Shutdown runs the graceful shutdown sequence with a timeout.
func (sc *ShutdownCoordinator) Shutdown() {
	done := make(chan struct{})
	go func() {
		defer close(done)
		sc.shutdownSequence()
	}()

	select {
	case <-done:
		sc.logger.Info("graceful shutdown complete")
	case <-time.After(sc.timeout):
		sc.logger.Error("shutdown timed out", slog.Duration("timeout", sc.timeout))
	}
}

func (sc *ShutdownCoordinator) shutdownSequence() {
	// 1. Cancel root context.
	if sc.cancel != nil {
		sc.cancel()
	}

	// 2. Stop consumers (parallel).
	var wg sync.WaitGroup
	for _, c := range sc.consumers {
		wg.Add(1)
		go func(c *Consumer) {
			defer wg.Done()
			c.Stop()
		}(c)
	}
	wg.Wait()

	// 3. Stop SSE emitter.
	if sc.statsEmitter != nil {
		sc.statsEmitter.Stop()
	}

	// 4. Shutdown NATS.
	if sc.natsSrv != nil {
		sc.natsSrv.Shutdown()
	}

	// 5. Close SQLite.
	if sc.db != nil {
		// WAL checkpoint happens on close.
		if err := sc.db.Close(); err != nil {
			sc.logger.Error("error closing database", slog.String("error", err.Error()))
		}
	}
}
