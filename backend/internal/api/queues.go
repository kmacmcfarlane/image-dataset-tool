package api

import (
	"context"
	"encoding/base64"
	"fmt"
	"unicode/utf8"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	logrus "github.com/sirupsen/logrus"

	queues "github.com/kmacmcfarlane/image-dataset-tool/backend/internal/api/gen/queues"
	"github.com/kmacmcfarlane/image-dataset-tool/backend/internal/natsutil"
)

// QueuesService implements the queues service interface.
type QueuesService struct {
	js jetstream.JetStream
}

// NewQueuesService creates a new queue administration service.
func NewQueuesService(js jetstream.JetStream) *QueuesService {
	return &QueuesService{js: js}
}

// Stats returns per-consumer queue statistics.
func (s *QueuesService) Stats(ctx context.Context) (*queues.QueueStatsResult, error) {
	log := logrus.WithField("component", "queues-api")

	consumers := []struct {
		name   string
		filter string
	}{
		{natsutil.ConsumerFetchInstagram, "media.fetch.instagram"},
		{natsutil.ConsumerProcess, natsutil.SubjectProcess},
		{natsutil.ConsumerCaption, natsutil.SubjectCaption},
		{natsutil.ConsumerExport, natsutil.SubjectExport},
		{natsutil.ConsumerDLQ, natsutil.SubjectDLQ},
	}

	result := &queues.QueueStatsResult{
		Consumers: make([]*queues.ConsumerStats, 0, len(consumers)),
	}

	for _, c := range consumers {
		cons, err := s.js.Consumer(ctx, natsutil.StreamName, c.name)
		if err != nil {
			log.WithError(err).WithField("consumer", c.name).Warn("Failed to get consumer")
			continue
		}

		info, err := cons.Info(ctx)
		if err != nil {
			log.WithError(err).WithField("consumer", c.name).Warn("Failed to get consumer info")
			continue
		}

		stat := &queues.ConsumerStats{
			Name:        c.name,
			Subject:     c.filter,
			Pending:     int64(info.NumPending),
			AckPending:  int64(info.NumAckPending),
			Redelivered: int64(info.NumRedelivered),
			Waiting:     int64(info.NumWaiting),
		}
		result.Consumers = append(result.Consumers, stat)
	}

	return result, nil
}

// Peek returns messages from a queue subject without consuming them.
func (s *QueuesService) Peek(ctx context.Context, p *queues.PeekPayload) (*queues.PeekResult, error) {
	log := logrus.WithFields(logrus.Fields{
		"component": "queues-api",
		"subject":   p.Subject,
	})

	offset := 0
	if p.Offset != nil {
		offset = *p.Offset
	}
	limit := 20
	if p.Limit != nil {
		limit = *p.Limit
	}

	// Get stream to access messages directly.
	stream, err := s.js.Stream(ctx, natsutil.StreamName)
	if err != nil {
		return nil, fmt.Errorf("get stream: %w", err)
	}

	// Get subject-level info for total count.
	streamInfo, err := stream.Info(ctx, jetstream.WithSubjectFilter(p.Subject))
	if err != nil {
		return nil, fmt.Errorf("get stream info: %w", err)
	}

	var total int64
	for _, count := range streamInfo.State.Subjects {
		total += int64(count)
	}

	// Iterate messages by subject using GetMsg with NextFor semantics.
	messages := make([]*queues.QueueMessage, 0, limit)
	skipped := 0
	collected := 0
	var lastSeq uint64

	for collected < limit {
		var rawMsg *jetstream.RawStreamMsg
		var getErr error

		if lastSeq == 0 {
			// Get first message for subject (start at seq 1).
			rawMsg, getErr = stream.GetMsg(ctx, 1, jetstream.WithGetMsgSubject(p.Subject))
		} else {
			// Get next message after lastSeq for this subject.
			rawMsg, getErr = stream.GetMsg(ctx, lastSeq+1, jetstream.WithGetMsgSubject(p.Subject))
		}
		if getErr != nil {
			// No more messages for this subject.
			break
		}

		lastSeq = rawMsg.Sequence

		if skipped < offset {
			skipped++
			continue
		}

		// Convert message data to string (UTF-8 or base64).
		data := formatData(rawMsg.Data)

		// Convert headers.
		headers := make(map[string]string)
		for k, v := range rawMsg.Header {
			if len(v) > 0 {
				headers[k] = v[0]
			}
		}

		qMsg := &queues.QueueMessage{
			Sequence:  rawMsg.Sequence,
			Subject:   rawMsg.Subject,
			Data:      data,
			Headers:   headers,
			Timestamp: rawMsg.Time.Format("2006-01-02T15:04:05Z07:00"),
		}
		messages = append(messages, qMsg)
		collected++
	}

	log.WithFields(logrus.Fields{
		"total":    total,
		"returned": len(messages),
		"offset":   offset,
		"limit":    limit,
	}).Debug("Peeked messages")

	return &queues.PeekResult{
		Messages: messages,
		Total:    total,
	}, nil
}

// Retry redelivers a specific message to its original subject.
func (s *QueuesService) Retry(ctx context.Context, p *queues.RetryPayload) error {
	log := logrus.WithFields(logrus.Fields{
		"component": "queues-api",
		"subject":   p.Subject,
		"sequence":  p.Sequence,
	})

	// Get the stream.
	stream, err := s.js.Stream(ctx, natsutil.StreamName)
	if err != nil {
		return fmt.Errorf("get stream: %w", err)
	}

	// Get the message by sequence.
	rawMsg, err := stream.GetMsg(ctx, p.Sequence)
	if err != nil {
		return fmt.Errorf("get message seq %d: %w", p.Sequence, err)
	}

	// Determine original subject from DLQ header or use current subject.
	originalSubject := rawMsg.Header.Get("X-DLQ-Original-Subject")
	if originalSubject == "" {
		originalSubject = rawMsg.Subject
	}

	// Republish to original subject.
	newMsg := &nats.Msg{
		Subject: originalSubject,
		Data:    rawMsg.Data,
		Header:  nats.Header{},
	}

	// Copy headers but remove DLQ metadata.
	for k, v := range rawMsg.Header {
		if k == "X-DLQ-Original-Subject" || k == "X-DLQ-Stream" ||
			k == "X-DLQ-Consumer" || k == "X-DLQ-Num-Delivered" {
			continue
		}
		for _, val := range v {
			newMsg.Header.Add(k, val)
		}
	}
	newMsg.Header.Set("X-Retried-From-Sequence", fmt.Sprintf("%d", p.Sequence))

	_, err = s.js.PublishMsg(ctx, newMsg)
	if err != nil {
		return fmt.Errorf("republish message: %w", err)
	}

	// Delete the original message from the stream.
	err = stream.DeleteMsg(ctx, p.Sequence)
	if err != nil {
		log.WithError(err).Warn("Failed to delete original message after retry")
		// Non-fatal: message was already republished.
	}

	log.WithField("original_subject", originalSubject).Info("Message retried")
	return nil
}

// DeleteMessage deletes a specific message from the stream.
func (s *QueuesService) DeleteMessage(ctx context.Context, p *queues.DeleteMessagePayload) error {
	log := logrus.WithFields(logrus.Fields{
		"component": "queues-api",
		"subject":   p.Subject,
		"sequence":  p.Sequence,
	})

	stream, err := s.js.Stream(ctx, natsutil.StreamName)
	if err != nil {
		return fmt.Errorf("get stream: %w", err)
	}

	err = stream.DeleteMsg(ctx, p.Sequence)
	if err != nil {
		return fmt.Errorf("delete message seq %d: %w", p.Sequence, err)
	}

	log.Info("Message deleted")
	return nil
}

// Purge removes all messages from a specific subject in the stream.
func (s *QueuesService) Purge(ctx context.Context, p *queues.PurgePayload) error {
	log := logrus.WithFields(logrus.Fields{
		"component": "queues-api",
		"subject":   p.Subject,
	})

	stream, err := s.js.Stream(ctx, natsutil.StreamName)
	if err != nil {
		return fmt.Errorf("get stream: %w", err)
	}

	err = stream.Purge(ctx, jetstream.WithPurgeSubject(p.Subject))
	if err != nil {
		return fmt.Errorf("purge subject %s: %w", p.Subject, err)
	}

	log.Info("Subject purged")
	return nil
}

// formatData returns data as UTF-8 string if valid, otherwise base64 encoded.
func formatData(data []byte) string {
	if utf8.Valid(data) {
		return string(data)
	}
	return base64.StdEncoding.EncodeToString(data)
}
