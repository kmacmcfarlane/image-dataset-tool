// Package natsutil provides an embedded NATS JetStream server with
// stream and consumer configuration for the media pipeline.
package natsutil

import "time"

// Config holds configuration for the embedded NATS server and JetStream.
type Config struct {
	// DataDir is the directory for JetStream file-backed storage.
	DataDir string

	// MaxPayloadKB is the maximum message payload size in kilobytes.
	// Default: 64.
	MaxPayloadKB int32

	// MaxBytes is the maximum size in bytes for the media stream.
	// Default: 1GB. Uses DiscardOld policy when exceeded.
	MaxBytes int64

	// Consumers holds per-consumer configuration.
	Consumers ConsumersConfig
}

// ConsumerConfig holds configuration for a single durable pull consumer.
type ConsumerConfig struct {
	// MaxAckPending limits in-flight messages for backpressure.
	MaxAckPending int

	// AckWait is the duration before an unacknowledged message is redelivered.
	AckWait time.Duration

	// MaxDeliver is the maximum number of delivery attempts before routing to DLQ.
	// Default: 5.
	MaxDeliver int

	// Concurrency is the number of parallel worker goroutines pulling from this consumer.
	// Default: 1.
	Concurrency int
}

// ConsumersConfig holds configuration for all pipeline consumers.
type ConsumersConfig struct {
	MediaFetchInstagram ConsumerConfig
	MediaProcess        ConsumerConfig
	MediaCaption        ConsumerConfig
	MediaExport         ConsumerConfig
}

// DefaultConfig returns configuration with PRD-specified defaults.
func DefaultConfig(dataDir string) Config {
	return Config{
		DataDir:      dataDir,
		MaxPayloadKB: 64,
		MaxBytes:     1 << 30, // 1GB
		Consumers: ConsumersConfig{
			MediaFetchInstagram: ConsumerConfig{
				MaxAckPending: 1,
				AckWait:       300 * time.Second,
				MaxDeliver:    5,
				Concurrency:   1,
			},
			MediaProcess: ConsumerConfig{
				MaxAckPending: 16,
				AckWait:       60 * time.Second,
				MaxDeliver:    5,
				Concurrency:   4, // CPU-bound, default to GOMAXPROCS-ish
			},
			MediaCaption: ConsumerConfig{
				MaxAckPending: 8,
				AckWait:       120 * time.Second,
				MaxDeliver:    5,
				Concurrency:   4,
			},
			MediaExport: ConsumerConfig{
				MaxAckPending: 4,
				AckWait:       60 * time.Second,
				MaxDeliver:    5,
				Concurrency:   2,
			},
		},
	}
}
