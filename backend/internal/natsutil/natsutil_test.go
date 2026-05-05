package natsutil_test

import (
	"context"
	"os"
	"path/filepath"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/nats-io/nats.go/jetstream"
	"github.com/kmacmcfarlane/image-dataset-tool/backend/internal/natsutil"
)

var _ = Describe("Embedded NATS JetStream", func() {
	var (
		srv     *natsutil.Server
		dataDir string
	)

	newServer := func(cfg natsutil.Config) *natsutil.Server {
		s, err := natsutil.New(cfg)
		Expect(err).NotTo(HaveOccurred())
		return s
	}

	BeforeEach(func() {
		var err error
		dataDir, err = os.MkdirTemp("", "nats-test-*")
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		if srv != nil {
			srv.Shutdown()
			srv = nil
		}
		os.RemoveAll(dataDir)
	})

	Describe("Server startup", func() {
		It("starts an embedded server with JetStream enabled", func() {
			cfg := natsutil.DefaultConfig(dataDir)
			srv = newServer(cfg)

			Expect(srv.JS()).NotTo(BeNil())
			Expect(srv.Conn()).NotTo(BeNil())
			Expect(srv.Conn().IsConnected()).To(BeTrue())
		})

		It("creates the MEDIA stream with correct subjects", func() {
			cfg := natsutil.DefaultConfig(dataDir)
			srv = newServer(cfg)

			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			stream, err := srv.JS().Stream(ctx, natsutil.StreamName)
			Expect(err).NotTo(HaveOccurred())

			info, err := stream.Info(ctx)
			Expect(err).NotTo(HaveOccurred())

			Expect(info.Config.Subjects).To(ConsistOf(
				"media.fetch.>",
				"media.process",
				"media.caption",
				"media.export",
				"media.dlq",
			))
			Expect(info.Config.Discard).To(Equal(jetstream.DiscardOld))
			Expect(info.Config.Storage).To(Equal(jetstream.FileStorage))
		})

		It("creates durable pull consumers for each subject", func() {
			cfg := natsutil.DefaultConfig(dataDir)
			srv = newServer(cfg)

			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			expectedConsumers := []string{
				natsutil.ConsumerFetchInstagram,
				natsutil.ConsumerProcess,
				natsutil.ConsumerCaption,
				natsutil.ConsumerExport,
				natsutil.ConsumerDLQ,
			}

			for _, name := range expectedConsumers {
				cons, err := srv.JS().Consumer(ctx, natsutil.StreamName, name)
				Expect(err).NotTo(HaveOccurred(), "consumer %s should exist", name)

				info, err := cons.Info(ctx)
				Expect(err).NotTo(HaveOccurred())
				Expect(info.Config.Durable).To(Equal(name))
			}
		})

		It("configures MaxAckPending and AckWait per consumer from config", func() {
			cfg := natsutil.DefaultConfig(dataDir)
			// Override some values to verify config is respected.
			cfg.Consumers.MediaProcess.MaxAckPending = 32
			cfg.Consumers.MediaProcess.AckWait = 90 * time.Second
			srv = newServer(cfg)

			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			cons, err := srv.JS().Consumer(ctx, natsutil.StreamName, natsutil.ConsumerProcess)
			Expect(err).NotTo(HaveOccurred())

			info, err := cons.Info(ctx)
			Expect(err).NotTo(HaveOccurred())
			Expect(info.Config.MaxAckPending).To(Equal(32))
			Expect(info.Config.AckWait).To(Equal(90 * time.Second))
		})
	})

	Describe("Publish and consume", func() {
		BeforeEach(func() {
			cfg := natsutil.DefaultConfig(dataDir)
			// Use short AckWait for test speed.
			cfg.Consumers.MediaProcess.AckWait = 2 * time.Second
			cfg.Consumers.MediaProcess.MaxDeliver = 3
			srv = newServer(cfg)
		})

		It("publishes, consumes, and ACKs — message is removed", func() {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			// Publish a message.
			_, err := srv.JS().Publish(ctx, natsutil.SubjectProcess, []byte(`{"id":"test-1"}`))
			Expect(err).NotTo(HaveOccurred())

			// Consume the message.
			cons, err := srv.JS().Consumer(ctx, natsutil.StreamName, natsutil.ConsumerProcess)
			Expect(err).NotTo(HaveOccurred())

			msgs, err := cons.Fetch(1, jetstream.FetchMaxWait(2*time.Second))
			Expect(err).NotTo(HaveOccurred())

			var received []jetstream.Msg
			for msg := range msgs.Messages() {
				received = append(received, msg)
			}
			Expect(received).To(HaveLen(1))
			Expect(string(received[0].Data())).To(Equal(`{"id":"test-1"}`))

			// ACK the message.
			err = received[0].Ack()
			Expect(err).NotTo(HaveOccurred())

			// Verify message is removed (no more pending).
			info, err := cons.Info(ctx)
			Expect(err).NotTo(HaveOccurred())
			Expect(info.NumPending).To(BeZero())
		})

		It("NAKs with delay — message is redelivered after delay", func() {
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()

			// Publish a message.
			_, err := srv.JS().Publish(ctx, natsutil.SubjectProcess, []byte(`{"id":"nak-test"}`))
			Expect(err).NotTo(HaveOccurred())

			cons, err := srv.JS().Consumer(ctx, natsutil.StreamName, natsutil.ConsumerProcess)
			Expect(err).NotTo(HaveOccurred())

			// First fetch and NAK with 1s delay.
			msgs, err := cons.Fetch(1, jetstream.FetchMaxWait(2*time.Second))
			Expect(err).NotTo(HaveOccurred())

			for msg := range msgs.Messages() {
				err = msg.NakWithDelay(1 * time.Second)
				Expect(err).NotTo(HaveOccurred())
			}

			// Immediately fetch — should get nothing (message is delayed).
			msgs, err = cons.Fetch(1, jetstream.FetchMaxWait(500*time.Millisecond))
			Expect(err).NotTo(HaveOccurred())

			var immediate []jetstream.Msg
			for msg := range msgs.Messages() {
				immediate = append(immediate, msg)
			}
			Expect(immediate).To(BeEmpty())

			// Wait for delay to expire, then fetch again.
			time.Sleep(1500 * time.Millisecond)

			msgs, err = cons.Fetch(1, jetstream.FetchMaxWait(2*time.Second))
			Expect(err).NotTo(HaveOccurred())

			var redelivered []jetstream.Msg
			for msg := range msgs.Messages() {
				redelivered = append(redelivered, msg)
			}
			Expect(redelivered).To(HaveLen(1))
			Expect(string(redelivered[0].Data())).To(Equal(`{"id":"nak-test"}`))

			// ACK to clean up.
			err = redelivered[0].Ack()
			Expect(err).NotTo(HaveOccurred())
		})

		It("routes to DLQ after max deliveries exceeded", func() {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			// Recreate server with MaxDeliver=3 and short AckWait.
			srv.Shutdown()
			shortCfg := natsutil.DefaultConfig(dataDir)
			shortCfg.Consumers.MediaProcess.MaxDeliver = 3
			shortCfg.Consumers.MediaProcess.AckWait = 2 * time.Second
			shortCfg.Consumers.MediaProcess.MaxAckPending = 1
			var err error
			srv = newServer(shortCfg)

			// Publish a message.
			_, err = srv.JS().Publish(ctx, natsutil.SubjectProcess, []byte(`{"id":"dlq-test"}`))
			Expect(err).NotTo(HaveOccurred())

			cons, err := srv.JS().Consumer(ctx, natsutil.StreamName, natsutil.ConsumerProcess)
			Expect(err).NotTo(HaveOccurred())

			maxDeliver := shortCfg.Consumers.MediaProcess.MaxDeliver

			// Simulate worker: NAK first deliveries, route to DLQ on final delivery.
			for i := 0; i < maxDeliver; i++ {
				msgs, fetchErr := cons.Fetch(1, jetstream.FetchMaxWait(3*time.Second))
				Expect(fetchErr).NotTo(HaveOccurred())

				for msg := range msgs.Messages() {
					if natsutil.ShouldDLQ(msg, maxDeliver) {
						// Final delivery — route to DLQ.
						Expect(natsutil.RouteToDLQ(ctx, srv.JS(), msg)).To(Succeed())
					} else {
						// Simulate processing failure.
						Expect(msg.Nak()).To(Succeed())
					}
				}
			}

			// Verify message is no longer pending on process consumer.
			info, err := cons.Info(ctx)
			Expect(err).NotTo(HaveOccurred())
			Expect(info.NumPending).To(BeZero())

			// Verify message arrived in DLQ.
			dlqCons, err := srv.JS().Consumer(ctx, natsutil.StreamName, natsutil.ConsumerDLQ)
			Expect(err).NotTo(HaveOccurred())

			dlqMsgs, err := dlqCons.Fetch(1, jetstream.FetchMaxWait(2*time.Second))
			Expect(err).NotTo(HaveOccurred())

			var dlqBodies []string
			for msg := range dlqMsgs.Messages() {
				dlqBodies = append(dlqBodies, string(msg.Data()))

				// Verify DLQ metadata headers.
				Expect(msg.Headers().Get("X-DLQ-Original-Subject")).To(Equal("media.process"))
				Expect(msg.Headers().Get("X-DLQ-Stream")).To(Equal(natsutil.StreamName))

				Expect(msg.Ack()).To(Succeed())
			}
			Expect(dlqBodies).To(ConsistOf(`{"id":"dlq-test"}`))
		})
	})

	Describe("Stream persistence", func() {
		It("preserves pending messages across server restart", func() {
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()

			// Use a persistent data dir (not cleaned between server instances).
			persistDir := filepath.Join(dataDir, "persist")
			Expect(os.MkdirAll(persistDir, 0755)).To(Succeed())

			cfg := natsutil.DefaultConfig(persistDir)
			cfg.Consumers.MediaCaption.AckWait = 60 * time.Second
			srv = newServer(cfg)

			// Publish messages.
			_, err := srv.JS().Publish(ctx, natsutil.SubjectCaption, []byte(`{"id":"persist-1"}`))
			Expect(err).NotTo(HaveOccurred())
			_, err = srv.JS().Publish(ctx, natsutil.SubjectCaption, []byte(`{"id":"persist-2"}`))
			Expect(err).NotTo(HaveOccurred())

			// Verify messages are present.
			stream, err := srv.JS().Stream(ctx, natsutil.StreamName)
			Expect(err).NotTo(HaveOccurred())
			info, err := stream.Info(ctx)
			Expect(err).NotTo(HaveOccurred())
			Expect(info.State.Msgs).To(BeNumerically(">=", 2))

			// Stop the server.
			srv.Shutdown()
			srv = nil

			// Restart with same data dir.
			srv = newServer(cfg)

			// Verify messages survived restart.
			stream, err = srv.JS().Stream(ctx, natsutil.StreamName)
			Expect(err).NotTo(HaveOccurred())
			info, err = stream.Info(ctx)
			Expect(err).NotTo(HaveOccurred())
			Expect(info.State.Msgs).To(BeNumerically(">=", 2))

			// Consume and verify content.
			cons, err := srv.JS().Consumer(ctx, natsutil.StreamName, natsutil.ConsumerCaption)
			Expect(err).NotTo(HaveOccurred())

			msgs, err := cons.Fetch(2, jetstream.FetchMaxWait(2*time.Second))
			Expect(err).NotTo(HaveOccurred())

			var bodies []string
			for msg := range msgs.Messages() {
				bodies = append(bodies, string(msg.Data()))
				Expect(msg.Ack()).To(Succeed())
			}
			Expect(bodies).To(ConsistOf(`{"id":"persist-1"}`, `{"id":"persist-2"}`))
		})
	})

	Describe("MaxBytes stream limit", func() {
		It("discards old messages when MaxBytes is exceeded", func() {
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()

			// Create a server with a very small MaxBytes to trigger DiscardOld.
			smallCfg := natsutil.DefaultConfig(dataDir)
			smallCfg.MaxBytes = 4096 // 4KB — very small
			smallCfg.Consumers.MediaExport.AckWait = 60 * time.Second
			srv = newServer(smallCfg)

			// Publish messages until we exceed MaxBytes.
			// Each message with subject + headers is roughly 200+ bytes.
			payload := make([]byte, 512) // 512-byte payload per message
			for i := range payload {
				payload[i] = byte('A' + (i % 26))
			}

			var publishCount int
			for i := 0; i < 50; i++ {
				_, err := srv.JS().Publish(ctx, natsutil.SubjectExport, payload)
				if err != nil {
					break
				}
				publishCount++
			}

			// We should have published many messages but the stream should
			// have fewer due to DiscardOld.
			Expect(publishCount).To(BeNumerically(">", 5),
				"should have published more than 5 messages")

			stream, err := srv.JS().Stream(ctx, natsutil.StreamName)
			Expect(err).NotTo(HaveOccurred())
			info, err := stream.Info(ctx)
			Expect(err).NotTo(HaveOccurred())

			// The stream should have kept only what fits in MaxBytes.
			Expect(info.State.Msgs).To(BeNumerically("<", uint64(publishCount)),
				"stream should have discarded old messages")
			Expect(info.State.Bytes).To(BeNumerically("<=", int64(smallCfg.MaxBytes)),
				"stream bytes should not exceed MaxBytes")
		})
	})
})
