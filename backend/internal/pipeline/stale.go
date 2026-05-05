package pipeline

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/nats-io/nats.go/jetstream"

	"github.com/kmacmcfarlane/image-dataset-tool/backend/internal/natsutil"
)

// DetectStaleJobs checks for jobs with status='running' that have no pending
// NATS messages. These are jobs interrupted by a crash or unclean shutdown.
// They are marked 'interrupted' so the UI can prompt the user.
func DetectStaleJobs(ctx context.Context, tracker JobTracker, js jetstream.JetStream, logger *slog.Logger) error {
	if logger == nil {
		logger = slog.Default()
	}

	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	jobs, err := tracker.GetRunningJobs(ctx)
	if err != nil {
		return fmt.Errorf("get running jobs: %w", err)
	}

	if len(jobs) == 0 {
		logger.Info("no stale running jobs found")
		return nil
	}

	// Get total pending across all consumers.
	totalPending, err := getTotalPending(ctx, js)
	if err != nil {
		return fmt.Errorf("get total pending: %w", err)
	}

	for _, job := range jobs {
		// If there are no pending messages anywhere, the job is stale.
		// A more granular check would filter by job_id in message data,
		// but since NATS doesn't support content-based filtering on pending
		// counts, we use the heuristic: if total pending across all consumers
		// is zero, all running jobs are stale.
		if totalPending == 0 {
			logger.Warn("marking stale job as interrupted",
				slog.String("job_id", job.ID),
				slog.String("type", job.Type),
			)
			if mErr := tracker.MarkInterrupted(ctx, job.ID); mErr != nil {
				logger.Error("failed to mark job interrupted",
					slog.String("job_id", job.ID),
					slog.String("error", mErr.Error()),
				)
			}
		} else {
			logger.Info("running job has pending messages, leaving as-is",
				slog.String("job_id", job.ID),
				slog.Uint64("total_pending", totalPending),
			)
		}
	}

	return nil
}

// getTotalPending sums NumPending across all pipeline consumers.
func getTotalPending(ctx context.Context, js jetstream.JetStream) (uint64, error) {
	consumerNames := []string{
		natsutil.ConsumerFetchInstagram,
		natsutil.ConsumerProcess,
		natsutil.ConsumerCaption,
		natsutil.ConsumerExport,
	}

	var total uint64
	for _, name := range consumerNames {
		cons, err := js.Consumer(ctx, natsutil.StreamName, name)
		if err != nil {
			continue // consumer may not exist
		}
		info, err := cons.Info(ctx)
		if err != nil {
			continue
		}
		total += info.NumPending + uint64(info.NumAckPending)
	}
	return total, nil
}
