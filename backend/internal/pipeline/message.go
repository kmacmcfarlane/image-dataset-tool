// Package pipeline provides the worker framework for processing NATS JetStream
// messages through the media pipeline stages.
package pipeline

import (
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
)

// Envelope wraps every message flowing through the pipeline.
// It carries routing metadata alongside the stage-specific payload.
type Envelope struct {
	// JobID links the message to a job_runs row.
	JobID string `json:"job_id"`

	// TraceID groups all messages originating from a single triggering event.
	// Generated on first-mover events, propagated to all child messages.
	TraceID string `json:"trace_id"`

	// Subject is the NATS subject this message targets (for logging/routing).
	Subject string `json:"subject,omitempty"`

	// SampleID is the sample this message pertains to (if applicable).
	SampleID string `json:"sample_id,omitempty"`

	// Provider identifies the external provider (e.g. "instagram", "anthropic").
	Provider string `json:"provider,omitempty"`

	// Payload carries the stage-specific data as raw JSON.
	Payload json.RawMessage `json:"payload,omitempty"`
}

// NewTraceID generates a new trace ID for first-mover events.
func NewTraceID() string {
	return uuid.New().String()
}

// MarshalEnvelope serializes an envelope to JSON bytes for NATS publishing.
func MarshalEnvelope(env *Envelope) ([]byte, error) {
	data, err := json.Marshal(env)
	if err != nil {
		return nil, fmt.Errorf("marshal envelope: %w", err)
	}
	return data, nil
}

// UnmarshalEnvelope deserializes an envelope from NATS message data.
func UnmarshalEnvelope(data []byte) (*Envelope, error) {
	var env Envelope
	if err := json.Unmarshal(data, &env); err != nil {
		return nil, fmt.Errorf("unmarshal envelope: %w", err)
	}
	return &env, nil
}
