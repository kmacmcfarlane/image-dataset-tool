package api_test

import (
	"bufio"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/kmacmcfarlane/image-dataset-tool/backend/internal/api"
	"github.com/kmacmcfarlane/image-dataset-tool/backend/internal/pipeline"
)

var _ = Describe("SSE", func() {
	var (
		sink    *pipeline.ChannelEventSink
		ring    *api.SSERingBuffer
		handler *api.SSEHandler
	)

	BeforeEach(func() {
		sink = pipeline.NewChannelEventSink(256)
		ring = api.NewSSERingBuffer(64)
		handler = api.NewSSEHandler(sink, ring)
		go handler.StartBroadcast()
	})

	Describe("SSERingBuffer", func() {
		It("stores and retrieves events in order", func() {
			rb := api.NewSSERingBuffer(10)
			id1 := rb.Push(pipeline.SSEEvent{Type: "a"})
			id2 := rb.Push(pipeline.SSEEvent{Type: "b"})
			id3 := rb.Push(pipeline.SSEEvent{Type: "c"})

			Expect(id1).To(Equal(uint64(1)))
			Expect(id2).To(Equal(uint64(2)))
			Expect(id3).To(Equal(uint64(3)))

			events := rb.Since(1) // after id 1
			Expect(events).To(HaveLen(2))
		})

		It("returns events in ascending ID order after wrap-around", func() {
			rb := api.NewSSERingBuffer(4)
			// Push 7 events into a buffer of capacity 4, causing wrap-around.
			for i := 0; i < 7; i++ {
				rb.Push(pipeline.SSEEvent{Type: fmt.Sprintf("evt-%d", i+1)})
			}

			// Since(0) should return the 4 surviving events in ascending ID order.
			events := rb.Since(0)
			Expect(events).To(HaveLen(4))

			var ids []uint64
			for _, e := range events {
				ids = append(ids, e.ID())
			}
			for i := 1; i < len(ids); i++ {
				Expect(ids[i]).To(BeNumerically(">", ids[i-1]),
					"IDs must be in ascending order after wrap-around")
			}
			// The surviving events should be IDs 4, 5, 6, 7.
			Expect(ids).To(Equal([]uint64{4, 5, 6, 7}))
		})

		It("returns empty for afterID beyond latest", func() {
			rb := api.NewSSERingBuffer(10)
			rb.Push(pipeline.SSEEvent{Type: "a"})
			events := rb.Since(99)
			Expect(events).To(BeEmpty())
		})
	})

	Describe("SSE handler", func() {
		It("streams events to connected clients", func() {
			srv := httptest.NewServer(http.HandlerFunc(handler.ServeHTTP))
			defer srv.Close()

			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()

			req, err := http.NewRequestWithContext(ctx, "GET", srv.URL+"/api/sse", nil)
			Expect(err).NotTo(HaveOccurred())

			resp, err := http.DefaultClient.Do(req)
			Expect(err).NotTo(HaveOccurred())
			defer resp.Body.Close()

			Expect(resp.Header.Get("Content-Type")).To(Equal("text/event-stream"))

			// Emit an event into the sink.
			sink.Emit(pipeline.SSEEvent{
				Type: "job.state",
				Data: map[string]interface{}{"id": "j1", "status": "running"},
			})

			// Read the SSE output, skipping comment lines (":").
			// The handler sends an initial ": connected\n\n" comment for proxy
			// compatibility, so we skip comment lines to reach the real event.
			scanner := bufio.NewScanner(resp.Body)
			var lines []string
			for scanner.Scan() {
				line := scanner.Text()
				// Skip SSE comment lines (begin with ":").
				if strings.HasPrefix(line, ":") {
					continue
				}
				lines = append(lines, line)
				// SSE events end with a blank line.
				if line == "" && len(lines) > 1 {
					break
				}
			}

			joined := strings.Join(lines, "\n")
			Expect(joined).To(ContainSubstring("id: "))
			Expect(joined).To(ContainSubstring("event: job.state"))
			Expect(joined).To(ContainSubstring(`"status":"running"`))
		})

		It("replays missed events on reconnect via Last-Event-ID", func() {
			// Pre-populate the ring buffer with events by emitting through the sink.
			// We need to wait briefly for the broadcast goroutine to process them.
			sink.Emit(pipeline.SSEEvent{Type: "event1", Data: map[string]interface{}{"n": 1}})
			sink.Emit(pipeline.SSEEvent{Type: "event2", Data: map[string]interface{}{"n": 2}})
			sink.Emit(pipeline.SSEEvent{Type: "event3", Data: map[string]interface{}{"n": 3}})

			// Wait for events to be processed into ring buffer.
			Eventually(func() int {
				return len(ring.Since(0))
			}, 2*time.Second, 50*time.Millisecond).Should(Equal(3))

			srv := httptest.NewServer(http.HandlerFunc(handler.ServeHTTP))
			defer srv.Close()

			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()

			// Reconnect asking for events after ID 1 (should get events 2 and 3).
			req, err := http.NewRequestWithContext(ctx, "GET", srv.URL+"/api/sse", nil)
			Expect(err).NotTo(HaveOccurred())
			req.Header.Set("Last-Event-ID", "1")

			resp, err := http.DefaultClient.Do(req)
			Expect(err).NotTo(HaveOccurred())
			defer resp.Body.Close()

			// Read replayed events.
			scanner := bufio.NewScanner(resp.Body)
			var eventCount int
			var idsSeen []uint64
			for scanner.Scan() {
				line := scanner.Text()
				if strings.HasPrefix(line, "id: ") {
					idStr := strings.TrimPrefix(line, "id: ")
					id, _ := strconv.ParseUint(idStr, 10, 64)
					idsSeen = append(idsSeen, id)
				}
				if strings.HasPrefix(line, "event: ") {
					eventCount++
				}
				// Stop after getting the replayed events (2 events = 2 blank-line-terminated blocks).
				if eventCount >= 2 && line == "" {
					break
				}
			}

			Expect(eventCount).To(BeNumerically(">=", 2))
			// IDs 2 and 3 should be present.
			Expect(idsSeen).To(ContainElements(uint64(2), uint64(3)))
		})
	})
})
