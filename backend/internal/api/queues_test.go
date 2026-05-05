package api_test

import (
	"context"
	"os"
	"path/filepath"
	"time"

	"github.com/nats-io/nats.go"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/kmacmcfarlane/image-dataset-tool/backend/internal/api"
	queues "github.com/kmacmcfarlane/image-dataset-tool/backend/internal/api/gen/queues"
	"github.com/kmacmcfarlane/image-dataset-tool/backend/internal/natsutil"
)

var _ = Describe("QueuesService", func() {
	var (
		svc     *api.QueuesService
		natsSrv *natsutil.Server
		ctx     context.Context
		tmpDir  string
	)

	BeforeEach(func() {
		ctx = context.Background()
		var err error
		tmpDir, err = os.MkdirTemp("", "queues-test-*")
		Expect(err).NotTo(HaveOccurred())

		cfg := natsutil.DefaultConfig(filepath.Join(tmpDir, "nats"))
		natsSrv, err = natsutil.New(cfg)
		Expect(err).NotTo(HaveOccurred())

		svc = api.NewQueuesService(natsSrv.JS())
	})

	AfterEach(func() {
		if natsSrv != nil {
			natsSrv.Shutdown()
		}
		os.RemoveAll(tmpDir)
	})

	Describe("Stats", func() {
		It("returns consumer statistics", func() {
			result, err := svc.Stats(ctx)
			Expect(err).NotTo(HaveOccurred())
			Expect(result.Consumers).NotTo(BeEmpty())

			// All configured consumers should be present.
			names := make([]string, len(result.Consumers))
			for i, c := range result.Consumers {
				names[i] = c.Name
			}
			Expect(names).To(ContainElements(
				natsutil.ConsumerFetchInstagram,
				natsutil.ConsumerProcess,
				natsutil.ConsumerCaption,
				natsutil.ConsumerExport,
				natsutil.ConsumerDLQ,
			))
		})

		It("shows pending count after publishing messages", func() {
			// Publish a message to the DLQ.
			msg := &nats.Msg{
				Subject: natsutil.SubjectDLQ,
				Data:    []byte(`{"error":"test failure"}`),
			}
			_, err := natsSrv.JS().PublishMsg(ctx, msg)
			Expect(err).NotTo(HaveOccurred())

			// Wait for message to be available.
			Eventually(func() int64 {
				result, err := svc.Stats(ctx)
				if err != nil {
					return 0
				}
				for _, c := range result.Consumers {
					if c.Name == natsutil.ConsumerDLQ {
						return c.Pending
					}
				}
				return 0
			}, 5*time.Second, 100*time.Millisecond).Should(BeNumerically(">=", 1))
		})
	})

	Describe("Peek", func() {
		BeforeEach(func() {
			// Publish a few messages to DLQ.
			for i := 0; i < 3; i++ {
				msg := &nats.Msg{
					Subject: natsutil.SubjectDLQ,
					Data:    []byte(`{"index":` + string(rune('0'+i)) + `}`),
					Header:  nats.Header{},
				}
				msg.Header.Set("X-DLQ-Original-Subject", "media.process")
				_, err := natsSrv.JS().PublishMsg(ctx, msg)
				Expect(err).NotTo(HaveOccurred())
			}
			// Wait for messages to be stored.
			time.Sleep(200 * time.Millisecond)
		})

		It("returns messages without consuming them", func() {
			limit := 10
			payload := &queues.PeekPayload{
				Subject: natsutil.SubjectDLQ,
				Limit:   &limit,
			}

			result, err := svc.Peek(ctx, payload)
			Expect(err).NotTo(HaveOccurred())
			Expect(result.Total).To(BeNumerically("==", 3))
			Expect(result.Messages).To(HaveLen(3))

			// Messages should still be there (not consumed).
			result2, err := svc.Peek(ctx, payload)
			Expect(err).NotTo(HaveOccurred())
			Expect(result2.Total).To(BeNumerically("==", 3))
		})

		It("supports pagination with offset and limit", func() {
			offset := 1
			limit := 1
			payload := &queues.PeekPayload{
				Subject: natsutil.SubjectDLQ,
				Offset:  &offset,
				Limit:   &limit,
			}

			result, err := svc.Peek(ctx, payload)
			Expect(err).NotTo(HaveOccurred())
			Expect(result.Messages).To(HaveLen(1))
			Expect(result.Total).To(BeNumerically("==", 3))
			// Should be the second message.
			Expect(result.Messages[0].Sequence).To(BeNumerically(">", 1))
		})

		It("includes headers in peeked messages", func() {
			limit := 1
			payload := &queues.PeekPayload{
				Subject: natsutil.SubjectDLQ,
				Limit:   &limit,
			}

			result, err := svc.Peek(ctx, payload)
			Expect(err).NotTo(HaveOccurred())
			Expect(result.Messages).To(HaveLen(1))
			Expect(result.Messages[0].Headers).To(HaveKeyWithValue("X-DLQ-Original-Subject", "media.process"))
		})
	})

	Describe("Retry", func() {
		var seq uint64

		BeforeEach(func() {
			// Publish a DLQ message with original subject header.
			msg := &nats.Msg{
				Subject: natsutil.SubjectDLQ,
				Data:    []byte(`{"item":"retry-test"}`),
				Header:  nats.Header{},
			}
			msg.Header.Set("X-DLQ-Original-Subject", natsutil.SubjectProcess)

			ack, err := natsSrv.JS().PublishMsg(ctx, msg)
			Expect(err).NotTo(HaveOccurred())
			seq = ack.Sequence
			time.Sleep(200 * time.Millisecond)
		})

		It("redelivers message to original subject and removes from DLQ", func() {
			err := svc.Retry(ctx, &queues.RetryPayload{
				Subject:  natsutil.SubjectDLQ,
				Sequence: seq,
			})
			Expect(err).NotTo(HaveOccurred())

			// Verify message was published to the original subject.
			// Check process consumer has a pending message.
			Eventually(func() int64 {
				result, err := svc.Stats(ctx)
				if err != nil {
					return 0
				}
				for _, c := range result.Consumers {
					if c.Name == natsutil.ConsumerProcess {
						return c.Pending
					}
				}
				return 0
			}, 5*time.Second, 100*time.Millisecond).Should(BeNumerically(">=", 1))

			// Original DLQ message should be removed.
			limit := 10
			peekResult, err := svc.Peek(ctx, &queues.PeekPayload{
				Subject: natsutil.SubjectDLQ,
				Limit:   &limit,
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(peekResult.Total).To(BeNumerically("==", 0))
		})
	})

	Describe("DeleteMessage", func() {
		var seq uint64

		BeforeEach(func() {
			msg := &nats.Msg{
				Subject: natsutil.SubjectDLQ,
				Data:    []byte(`{"item":"delete-test"}`),
			}
			ack, err := natsSrv.JS().PublishMsg(ctx, msg)
			Expect(err).NotTo(HaveOccurred())
			seq = ack.Sequence
			time.Sleep(200 * time.Millisecond)
		})

		It("removes the message from the stream", func() {
			err := svc.DeleteMessage(ctx, &queues.DeleteMessagePayload{
				Subject:  natsutil.SubjectDLQ,
				Sequence: seq,
			})
			Expect(err).NotTo(HaveOccurred())

			// Verify message is gone.
			limit := 10
			result, err := svc.Peek(ctx, &queues.PeekPayload{
				Subject: natsutil.SubjectDLQ,
				Limit:   &limit,
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(result.Total).To(BeNumerically("==", 0))
		})
	})

	Describe("Purge", func() {
		BeforeEach(func() {
			// Publish multiple messages.
			for i := 0; i < 5; i++ {
				msg := &nats.Msg{
					Subject: natsutil.SubjectDLQ,
					Data:    []byte(`{"item":"purge-test"}`),
				}
				_, err := natsSrv.JS().PublishMsg(ctx, msg)
				Expect(err).NotTo(HaveOccurred())
			}
			time.Sleep(200 * time.Millisecond)
		})

		It("removes all messages for the subject", func() {
			err := svc.Purge(ctx, &queues.PurgePayload{
				Subject: natsutil.SubjectDLQ,
			})
			Expect(err).NotTo(HaveOccurred())

			// Verify all messages are gone.
			limit := 10
			result, err := svc.Peek(ctx, &queues.PeekPayload{
				Subject: natsutil.SubjectDLQ,
				Limit:   &limit,
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(result.Total).To(BeNumerically("==", 0))
		})
	})
})
