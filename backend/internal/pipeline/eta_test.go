package pipeline_test

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/kmacmcfarlane/image-dataset-tool/backend/internal/pipeline"
)

var _ = Describe("ETATracker", func() {
	var tracker *pipeline.ETATracker

	BeforeEach(func() {
		tracker = pipeline.NewETATracker()
	})

	Describe("EstimateETA", func() {
		It("returns 0 when no completions recorded", func() {
			eta := tracker.EstimateETA("job-1", 0, 10)
			Expect(eta).To(Equal(0.0))
		})

		It("returns 0 when completed equals total", func() {
			tracker.RecordCompletion("job-1")
			eta := tracker.EstimateETA("job-1", 10, 10)
			Expect(eta).To(Equal(0.0))
		})

		It("returns 0 for unknown job", func() {
			eta := tracker.EstimateETA("unknown", 5, 10)
			Expect(eta).To(Equal(0.0))
		})

		It("calculates ETA based on average completion duration", func() {
			// Record some completions with known timing.
			tracker.RecordCompletion("job-1")
			time.Sleep(100 * time.Millisecond)
			tracker.RecordCompletion("job-1")

			// 2 completions in ~100ms = ~50ms per item.
			// 8 remaining items = ~400ms = ~0.4s.
			eta := tracker.EstimateETA("job-1", 2, 10)
			Expect(eta).To(BeNumerically(">", 0))
			Expect(eta).To(BeNumerically("<", 5)) // Sanity: should be < 5 seconds.
		})
	})

	Describe("Remove", func() {
		It("cleans up job tracking data", func() {
			tracker.RecordCompletion("job-1")
			tracker.Remove("job-1")
			eta := tracker.EstimateETA("job-1", 5, 10)
			Expect(eta).To(Equal(0.0))
		})
	})
})
