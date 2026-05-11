package api

import (
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"sync"

	logrus "github.com/sirupsen/logrus"

	"github.com/kmacmcfarlane/image-dataset-tool/backend/internal/pipeline"
)

// SSERingBuffer stores recent SSE events for Last-Event-ID reconnection replay.
type SSERingBuffer struct {
	mu     sync.RWMutex
	buf    []numberedEvent
	cap    int
	nextID uint64 // monotonically increasing event ID
}

type numberedEvent struct {
	id    uint64
	event pipeline.SSEEvent
}

// ID returns the event's monotonic ID.
func (ne numberedEvent) ID() uint64 { return ne.id }

// NewSSERingBuffer creates a ring buffer of the given capacity.
func NewSSERingBuffer(capacity int) *SSERingBuffer {
	if capacity <= 0 {
		capacity = 1024
	}
	return &SSERingBuffer{
		buf: make([]numberedEvent, 0, capacity),
		cap: capacity,
	}
}

// Push adds an event to the ring buffer and returns its assigned ID.
func (rb *SSERingBuffer) Push(event pipeline.SSEEvent) uint64 {
	rb.mu.Lock()
	defer rb.mu.Unlock()

	rb.nextID++
	ne := numberedEvent{id: rb.nextID, event: event}

	if len(rb.buf) < rb.cap {
		rb.buf = append(rb.buf, ne)
	} else {
		// Overwrite oldest entry (circular).
		rb.buf[int(rb.nextID-1)%rb.cap] = ne
	}
	return rb.nextID
}

// Since returns all events with ID > afterID, in order.
func (rb *SSERingBuffer) Since(afterID uint64) []numberedEvent {
	rb.mu.RLock()
	defer rb.mu.RUnlock()

	if len(rb.buf) == 0 {
		return nil
	}

	var result []numberedEvent
	for _, ne := range rb.buf {
		if ne.id > afterID {
			result = append(result, ne)
		}
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].id < result[j].id
	})
	return result
}

// SSEHandler manages SSE client connections and broadcasts pipeline events.
type SSEHandler struct {
	sink *pipeline.ChannelEventSink
	ring *SSERingBuffer

	mu      sync.Mutex
	clients map[chan numberedEvent]struct{}
}

// NewSSEHandler creates a new SSE handler wired to the pipeline event sink.
func NewSSEHandler(sink *pipeline.ChannelEventSink, ring *SSERingBuffer) *SSEHandler {
	h := &SSEHandler{
		sink:    sink,
		ring:    ring,
		clients: make(map[chan numberedEvent]struct{}),
	}
	return h
}

// StartBroadcast reads events from the sink channel, stores them in the ring
// buffer, and fans out to all connected SSE clients. Call in a goroutine.
func (h *SSEHandler) StartBroadcast() {
	for event := range h.sink.Events() {
		id := h.ring.Push(event)
		ne := numberedEvent{id: id, event: event}

		h.mu.Lock()
		for ch := range h.clients {
			select {
			case ch <- ne:
			default:
				// Drop if client is slow.
			}
		}
		h.mu.Unlock()
	}
}

// ServeHTTP implements http.Handler for GET /api/sse.
func (h *SSEHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	log := logrus.WithField("component", "sse")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no") // nginx passthrough
	w.WriteHeader(http.StatusOK)
	// Send a comment line immediately to flush the proxy's response buffer and
	// allow the browser's EventSource to transition to OPEN state.
	fmt.Fprintf(w, ": connected\n\n")
	flusher.Flush()

	// Replay missed events if Last-Event-ID is provided.
	var lastID uint64
	if leid := r.Header.Get("Last-Event-ID"); leid != "" {
		if parsed, err := strconv.ParseUint(leid, 10, 64); err == nil {
			lastID = parsed
			log.WithField("last_event_id", lastID).Debug("SSE reconnect with Last-Event-ID")
		}
	}

	if lastID > 0 {
		missed := h.ring.Since(lastID)
		for _, ne := range missed {
			if err := writeSSEEvent(w, ne); err != nil {
				log.WithError(err).Debug("Failed to write replay event")
				return
			}
		}
		flusher.Flush()
	}

	// Register client channel.
	ch := make(chan numberedEvent, 64)
	h.mu.Lock()
	h.clients[ch] = struct{}{}
	h.mu.Unlock()

	defer func() {
		h.mu.Lock()
		delete(h.clients, ch)
		h.mu.Unlock()
	}()

	ctx := r.Context()
	for {
		select {
		case <-ctx.Done():
			log.Debug("SSE client disconnected")
			return
		case ne := <-ch:
			if err := writeSSEEvent(w, ne); err != nil {
				log.WithError(err).Debug("Failed to write SSE event")
				return
			}
			flusher.Flush()
		}
	}
}

func writeSSEEvent(w http.ResponseWriter, ne numberedEvent) error {
	data, err := pipeline.MarshalSSEEvent(ne.event)
	if err != nil {
		return err
	}
	// Prepend the id field before the event data.
	idLine := fmt.Sprintf("id: %d\n", ne.id)
	if _, err := w.Write([]byte(idLine)); err != nil {
		return err
	}
	if _, err := w.Write(data); err != nil {
		return err
	}
	return nil
}
