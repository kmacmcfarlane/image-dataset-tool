package pipeline_test

import (
	"context"
	"log/slog"
	"os"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/kmacmcfarlane/image-dataset-tool/backend/internal/natsutil"
	"github.com/kmacmcfarlane/image-dataset-tool/backend/internal/pipeline"
)

var _ = Describe("SSE Bridge", func() {
	var (
		srv     *natsutil.Server
		dataDir string
	)

	BeforeEach(func() {
		var err error
		dataDir, err = os.MkdirTemp("", "sse-test-nats-*")
		Expect(err).NotTo(HaveOccurred())

		cfg := natsutil.DefaultConfig(dataDir)
		srv, err = natsutil.New(cfg)
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		if srv != nil {
			srv.Shutdown()
		}
		os.RemoveAll(dataDir)
	})

	Describe("ChannelEventSink", func() {
		It("delivers events via channel", func() {
			sink := pipeline.NewChannelEventSink(10)
			sink.Emit(pipeline.SSEEvent{
				Type: "test.event",
				Data: map[string]interface{}{"key": "value"},
			})

			var event pipeline.SSEEvent
			Eventually(sink.Events()).Should(Receive(&event))
			Expect(event.Type).To(Equal("test.event"))
			Expect(event.Data["key"]).To(Equal("value"))
		})

		It("drops events when channel is full", func() {
			sink := pipeline.NewChannelEventSink(1)
			// Fill the buffer.
			sink.Emit(pipeline.SSEEvent{Type: "first"})
			// This should be dropped (non-blocking).
			sink.Emit(pipeline.SSEEvent{Type: "dropped"})

			var event pipeline.SSEEvent
			Eventually(sink.Events()).Should(Receive(&event))
			Expect(event.Type).To(Equal("first"))
			Consistently(sink.Events(), 100*time.Millisecond).ShouldNot(Receive())
		})
	})

	Describe("ConsumerStatsEmitter", func() {
		It("emits consumer.stats every interval", func() {
			sink := pipeline.NewChannelEventSink(64)
			logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))

			emitter := pipeline.NewConsumerStatsEmitter(srv.JS(), sink, 500*time.Millisecond, logger)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			emitter.Start(ctx, []pipeline.ConsumerStatsDef{
				{ConsumerName: natsutil.ConsumerProcess, Subject: natsutil.SubjectProcess},
			})

			// Should receive at least one consumer.stats event within 2 seconds.
			var statsEvent pipeline.SSEEvent
			Eventually(sink.Events(), 3*time.Second, 100*time.Millisecond).Should(Receive(&statsEvent))
			Expect(statsEvent.Type).To(Equal("consumer.stats"))
			Expect(statsEvent.Data["subject"]).To(Equal(natsutil.SubjectProcess))
			Expect(statsEvent.Data).To(HaveKey("pending"))
			Expect(statsEvent.Data).To(HaveKey("ack_pending"))
			Expect(statsEvent.Data).To(HaveKey("redelivered"))

			emitter.Stop()
		})
	})

	Describe("MarshalSSEEvent", func() {
		It("formats events in SSE wire format", func() {
			event := pipeline.SSEEvent{
				Type: "job.state",
				Data: map[string]interface{}{"id": "j1", "status": "running"},
			}
			data, err := pipeline.MarshalSSEEvent(event)
			Expect(err).NotTo(HaveOccurred())
			Expect(string(data)).To(ContainSubstring("event: job.state"))
			Expect(string(data)).To(ContainSubstring(`"id":"j1"`))
			Expect(string(data)).To(ContainSubstring(`"status":"running"`))
		})
	})
})
