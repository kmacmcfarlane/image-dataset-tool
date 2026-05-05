package natsutil

import (
	"context"
	"fmt"
	"strconv"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	logrus "github.com/sirupsen/logrus"
)

// ShouldDLQ returns true if the message has reached its max delivery count.
// Workers should call this before processing and route to DLQ if true.
func ShouldDLQ(msg jetstream.Msg, maxDeliver int) bool {
	md, err := msg.Metadata()
	if err != nil {
		return false
	}
	return int(md.NumDelivered) >= maxDeliver
}

// RouteToDLQ publishes the message payload to the DLQ subject with
// original metadata in headers, then ACKs the original message.
func RouteToDLQ(ctx context.Context, js jetstream.JetStream, msg jetstream.Msg) error {
	log := logrus.WithField("component", "nats-dlq")

	md, err := msg.Metadata()
	if err != nil {
		return fmt.Errorf("get message metadata: %w", err)
	}

	// Build DLQ message with original metadata in headers.
	dlqMsg := &nats.Msg{
		Subject: SubjectDLQ,
		Data:    msg.Data(),
		Header:  nats.Header{},
	}

	// Copy original headers.
	for k, v := range msg.Headers() {
		for _, val := range v {
			dlqMsg.Header.Add(k, val)
		}
	}

	// Add DLQ metadata headers.
	dlqMsg.Header.Set("X-DLQ-Original-Subject", msg.Subject())
	dlqMsg.Header.Set("X-DLQ-Stream", md.Stream)
	dlqMsg.Header.Set("X-DLQ-Consumer", md.Consumer)
	dlqMsg.Header.Set("X-DLQ-Num-Delivered", strconv.FormatUint(md.NumDelivered, 10))

	// Publish to DLQ.
	_, err = js.PublishMsg(ctx, dlqMsg)
	if err != nil {
		return fmt.Errorf("publish to DLQ: %w", err)
	}

	// ACK the original message so it's removed from the source consumer.
	if err := msg.Ack(); err != nil {
		return fmt.Errorf("ack original message: %w", err)
	}

	log.WithFields(logrus.Fields{
		"original_subject": msg.Subject(),
		"stream":           md.Stream,
		"consumer":         md.Consumer,
		"num_delivered":    md.NumDelivered,
	}).Info("Message routed to DLQ")

	return nil
}
