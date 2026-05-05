package pipeline_test

import (
	"encoding/json"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/kmacmcfarlane/image-dataset-tool/backend/internal/pipeline"
)

var _ = Describe("Envelope", func() {
	Describe("Marshal/Unmarshal roundtrip", func() {
		It("preserves all fields", func() {
			original := &pipeline.Envelope{
				JobID:    "job-123",
				TraceID:  "trace-abc",
				Subject:  "media.fetch.instagram",
				SampleID: "sample-xyz",
				Provider: "instagram",
				Payload:  json.RawMessage(`{"url":"https://example.com/img.jpg"}`),
			}

			data, err := pipeline.MarshalEnvelope(original)
			Expect(err).NotTo(HaveOccurred())

			restored, err := pipeline.UnmarshalEnvelope(data)
			Expect(err).NotTo(HaveOccurred())

			Expect(restored.JobID).To(Equal(original.JobID))
			Expect(restored.TraceID).To(Equal(original.TraceID))
			Expect(restored.Subject).To(Equal(original.Subject))
			Expect(restored.SampleID).To(Equal(original.SampleID))
			Expect(restored.Provider).To(Equal(original.Provider))
			Expect(string(restored.Payload)).To(Equal(string(original.Payload)))
		})
	})

	Describe("Unmarshal error handling", func() {
		It("returns error for invalid JSON", func() {
			_, err := pipeline.UnmarshalEnvelope([]byte("not json"))
			Expect(err).To(HaveOccurred())
		})
	})

	Describe("NewTraceID", func() {
		It("generates unique trace IDs", func() {
			id1 := pipeline.NewTraceID()
			id2 := pipeline.NewTraceID()
			Expect(id1).NotTo(BeEmpty())
			Expect(id2).NotTo(BeEmpty())
			Expect(id1).NotTo(Equal(id2))
		})
	})

	Describe("Trace ID propagation", func() {
		It("preserves trace_id when creating child envelope", func() {
			parent := &pipeline.Envelope{
				JobID:   "job-1",
				TraceID: "trace-parent",
			}

			// Simulate creating a child message for the next pipeline stage.
			child := &pipeline.Envelope{
				JobID:   parent.JobID,
				TraceID: parent.TraceID, // propagated
				Subject: "media.process",
			}

			data, err := pipeline.MarshalEnvelope(child)
			Expect(err).NotTo(HaveOccurred())

			restored, err := pipeline.UnmarshalEnvelope(data)
			Expect(err).NotTo(HaveOccurred())

			Expect(restored.TraceID).To(Equal("trace-parent"))
			Expect(restored.JobID).To(Equal("job-1"))
		})
	})
})
