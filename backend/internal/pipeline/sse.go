package pipeline

import (
	"context"
	"encoding/json"
	"log/slog"
	"sync"
	"time"

	"github.com/nats-io/nats.go/jetstream"

	"github.com/kmacmcfarlane/image-dataset-tool/backend/internal/natsutil"
)

// SSEEvent represents an event to be sent to SSE clients.
type SSEEvent struct {
	Type string                 `json:"type"`
	Data map[string]interface{} `json:"data"`
}

// EventSink is an interface for receiving pipeline events.
// Implementations deliver events to SSE clients.
type EventSink interface {
	// Emit sends an event to all connected SSE clients.
	Emit(event SSEEvent)
}

// ChannelEventSink delivers events via a Go channel.
type ChannelEventSink struct {
	ch chan SSEEvent
}

// NewChannelEventSink creates an EventSink backed by a buffered channel.
func NewChannelEventSink(bufSize int) *ChannelEventSink {
	if bufSize <= 0 {
		bufSize = 256
	}
	return &ChannelEventSink{ch: make(chan SSEEvent, bufSize)}
}

// Emit sends an event to the channel. Drops if the channel is full.
func (s *ChannelEventSink) Emit(event SSEEvent) {
	select {
	case s.ch <- event:
	default:
		// Drop event if channel is full to prevent blocking workers.
	}
}

// Events returns the read-only channel for consuming events.
func (s *ChannelEventSink) Events() <-chan SSEEvent {
	return s.ch
}

// MarshalSSEEvent serializes an SSEEvent to the SSE wire format.
// Note: json.Marshal of map[string]any produces single-line JSON output
// (no embedded newlines), so no newline escaping is required for the SSE data field.
func MarshalSSEEvent(event SSEEvent) ([]byte, error) {
	data, err := json.Marshal(event.Data)
	if err != nil {
		return nil, err
	}
	// SSE format: "event: <type>\ndata: <json>\n\n"
	return []byte("event: " + event.Type + "\ndata: " + string(data) + "\n\n"), nil
}

// ConsumerStatsEmitter periodically queries NATS consumer info and emits
// consumer.stats SSE events every interval.
type ConsumerStatsEmitter struct {
	js       jetstream.JetStream
	sink     EventSink
	interval time.Duration
	logger   *slog.Logger

	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// ConsumerStatsDef defines a consumer to monitor.
type ConsumerStatsDef struct {
	ConsumerName string
	Subject      string
}

// NewConsumerStatsEmitter creates a stats emitter.
func NewConsumerStatsEmitter(js jetstream.JetStream, sink EventSink, interval time.Duration, logger *slog.Logger) *ConsumerStatsEmitter {
	if interval <= 0 {
		interval = 2 * time.Second
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &ConsumerStatsEmitter{
		js:       js,
		sink:     sink,
		interval: interval,
		logger:   logger,
	}
}

// Start begins periodic stats emission for the given consumers.
func (e *ConsumerStatsEmitter) Start(ctx context.Context, consumers []ConsumerStatsDef) {
	ctx, e.cancel = context.WithCancel(ctx)
	e.wg.Add(1)
	go e.loop(ctx, consumers)
}

// Stop halts the emitter and waits for the goroutine to exit.
func (e *ConsumerStatsEmitter) Stop() {
	if e.cancel != nil {
		e.cancel()
	}
	e.wg.Wait()
}

func (e *ConsumerStatsEmitter) loop(ctx context.Context, consumers []ConsumerStatsDef) {
	defer e.wg.Done()
	ticker := time.NewTicker(e.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			e.emitStats(ctx, consumers)
		}
	}
}

func (e *ConsumerStatsEmitter) emitStats(ctx context.Context, consumers []ConsumerStatsDef) {
	for _, cd := range consumers {
		cons, err := e.js.Consumer(ctx, natsutil.StreamName, cd.ConsumerName)
		if err != nil {
			continue // consumer may not exist yet
		}
		info, err := cons.Info(ctx)
		if err != nil {
			continue
		}

		e.sink.Emit(SSEEvent{
			Type: "consumer.stats",
			Data: map[string]interface{}{
				"subject":      cd.Subject,
				"pending":      info.NumPending,
				"ack_pending":  info.NumAckPending,
				"redelivered":  info.NumRedelivered,
			},
		})
	}
}
