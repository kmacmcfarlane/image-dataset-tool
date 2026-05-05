package natsutil

import (
	"context"
	"fmt"
	"time"

	"github.com/nats-io/nats.go/jetstream"
	logrus "github.com/sirupsen/logrus"
)

const (
	// StreamName is the name of the JetStream stream for the media pipeline.
	StreamName = "MEDIA"

	// Subject constants for the media pipeline.
	SubjectFetchAll = "media.fetch.>"
	SubjectProcess  = "media.process"
	SubjectCaption  = "media.caption"
	SubjectExport   = "media.export"
	SubjectDLQ      = "media.dlq"

	// Consumer name constants.
	ConsumerFetchInstagram = "media-fetch-instagram"
	ConsumerProcess        = "media-process"
	ConsumerCaption        = "media-caption"
	ConsumerExport         = "media-export"
	ConsumerDLQ            = "media-dlq"
)

// setupStream creates (or updates) the media stream and all durable pull consumers.
func (s *Server) setupStream() error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	log := logrus.WithField("component", "nats")

	// Create or update the stream.
	streamCfg := jetstream.StreamConfig{
		Name: StreamName,
		Subjects: []string{
			SubjectFetchAll,
			SubjectProcess,
			SubjectCaption,
			SubjectExport,
			SubjectDLQ,
		},
		MaxBytes:  s.cfg.MaxBytes,
		Discard:   jetstream.DiscardOld,
		Storage:   jetstream.FileStorage,
		Retention: jetstream.WorkQueuePolicy,
	}

	_, err := s.js.CreateOrUpdateStream(ctx, streamCfg)
	if err != nil {
		return fmt.Errorf("create stream %s: %w", StreamName, err)
	}

	log.WithField("stream", StreamName).Info("Stream configured")

	// Create durable pull consumers.
	consumers := []struct {
		name   string
		filter string
		cfg    ConsumerConfig
	}{
		{ConsumerFetchInstagram, "media.fetch.instagram", s.cfg.Consumers.MediaFetchInstagram},
		{ConsumerProcess, SubjectProcess, s.cfg.Consumers.MediaProcess},
		{ConsumerCaption, SubjectCaption, s.cfg.Consumers.MediaCaption},
		{ConsumerExport, SubjectExport, s.cfg.Consumers.MediaExport},
		{ConsumerDLQ, SubjectDLQ, ConsumerConfig{
			MaxAckPending: 16,
			AckWait:       60 * time.Second,
			MaxDeliver:    1, // DLQ messages are not retried.
		}},
	}

	for _, c := range consumers {
		consCfg := jetstream.ConsumerConfig{
			Durable:       c.name,
			FilterSubject: c.filter,
			AckPolicy:     jetstream.AckExplicitPolicy,
			AckWait:       c.cfg.AckWait,
			MaxAckPending: c.cfg.MaxAckPending,
			MaxDeliver:    c.cfg.MaxDeliver,
		}

		_, err := s.js.CreateOrUpdateConsumer(ctx, StreamName, consCfg)
		if err != nil {
			return fmt.Errorf("create consumer %s: %w", c.name, err)
		}

		log.WithFields(logrus.Fields{
			"consumer":       c.name,
			"filter":         c.filter,
			"max_ack_pending": c.cfg.MaxAckPending,
			"ack_wait":       c.cfg.AckWait,
		}).Info("Consumer configured")
	}

	return nil
}
