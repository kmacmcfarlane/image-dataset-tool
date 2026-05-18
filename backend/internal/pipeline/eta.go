package pipeline

import (
	"sync"
	"time"
)

// ETATracker calculates estimated time of arrival for job completion
// based on the average duration of completed items.
type ETATracker struct {
	mu        sync.Mutex
	jobs      map[string]*jobETA
}

type jobETA struct {
	startedAt  time.Time
	completions int
}

// NewETATracker creates a new ETA tracker.
func NewETATracker() *ETATracker {
	return &ETATracker{
		jobs: make(map[string]*jobETA),
	}
}

// RecordCompletion records that an item was completed for a job.
func (t *ETATracker) RecordCompletion(jobID string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	j, ok := t.jobs[jobID]
	if !ok {
		j = &jobETA{startedAt: time.Now()}
		t.jobs[jobID] = j
	}
	j.completions++
}

// EstimateETA calculates the ETA in seconds for a job based on average item duration.
// Returns 0 if there is not enough data to estimate.
func (t *ETATracker) EstimateETA(jobID string, completed, total int) float64 {
	t.mu.Lock()
	defer t.mu.Unlock()

	if completed <= 0 || total <= completed {
		return 0
	}

	j, ok := t.jobs[jobID]
	if !ok || j.completions <= 0 {
		return 0
	}

	elapsed := time.Since(j.startedAt).Seconds()
	avgDuration := elapsed / float64(j.completions)
	remaining := total - completed
	return avgDuration * float64(remaining)
}

// Remove cleans up tracking data for a completed job.
func (t *ETATracker) Remove(jobID string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	delete(t.jobs, jobID)
}
