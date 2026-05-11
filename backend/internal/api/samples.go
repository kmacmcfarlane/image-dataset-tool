package api

import (
	"encoding/json"
	"net/http"
	"strings"

	logrus "github.com/sirupsen/logrus"

	"github.com/kmacmcfarlane/image-dataset-tool/backend/internal/model"
	"github.com/kmacmcfarlane/image-dataset-tool/backend/internal/store"
)

// SampleResponse is the JSON representation of a sample in the API response.
type SampleResponse struct {
	ID             string `json:"id"`
	SubjectID      string `json:"subject_id"`
	SourcePlatform string `json:"source_platform"`
	SourcePostID   string `json:"source_post_id"`
	SlideIndex     int    `json:"slide_index"`
	Width          int    `json:"width"`
	Height         int    `json:"height"`
	Status         string `json:"status"`
	IsDuplicate    bool   `json:"is_duplicate"`
	DuplicateOf    string `json:"duplicate_of,omitempty"`
	ThumbnailPath  string `json:"thumbnail_path"`
	CreatedAt      string `json:"created_at"`
	UpdatedAt      string `json:"updated_at"`
}

// SampleListResponse wraps the samples array.
type SampleListResponse struct {
	Samples []SampleResponse `json:"samples"`
}

// SamplesHandler serves sample listing for a subject.
type SamplesHandler struct {
	store *store.SampleStore
}

// NewSamplesHandler creates a new samples handler.
func NewSamplesHandler(sampleStore *store.SampleStore) *SamplesHandler {
	return &SamplesHandler{store: sampleStore}
}

// ServeList handles GET /v1/subjects/{sid}/samples.
func (h *SamplesHandler) ServeList(w http.ResponseWriter, r *http.Request) {
	// Extract subject ID from URL path: /v1/subjects/{sid}/samples
	subjectID := extractSubjectID(r.URL.Path)
	if subjectID == "" || !sampleIDPattern.MatchString(subjectID) {
		respondError(w, http.StatusBadRequest, "missing or invalid subject_id")
		return
	}

	samples, err := h.store.ListBySubject(subjectID)
	if err != nil {
		logrus.WithError(err).WithField("subject_id", subjectID).Error("Failed to list samples")
		respondError(w, http.StatusInternalServerError, "failed to list samples")
		return
	}

	resp := SampleListResponse{
		Samples: make([]SampleResponse, 0, len(samples)),
	}
	for _, s := range samples {
		resp.Samples = append(resp.Samples, mapSampleToResponse(s))
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		logrus.WithError(err).Error("Failed to encode samples response")
	}
}

// mapSampleToResponse converts a model.Sample to the API response shape.
func mapSampleToResponse(s *model.Sample) SampleResponse {
	return SampleResponse{
		ID:             s.ID,
		SubjectID:      s.SubjectID,
		SourcePlatform: s.SourcePlatform,
		SourcePostID:   s.SourcePostID,
		SlideIndex:     s.SlideIndex,
		Width:          s.Width,
		Height:         s.Height,
		Status:         s.Status,
		IsDuplicate:    s.IsDuplicate,
		DuplicateOf:    s.DuplicateOf,
		ThumbnailPath:  "/v1/media/" + s.ID + "/thumb",
		CreatedAt:      s.CreatedAt,
		UpdatedAt:      s.UpdatedAt,
	}
}

// extractSubjectID extracts the subject ID from path /v1/subjects/{sid}/samples.
func extractSubjectID(urlPath string) string {
	// Path format: /v1/subjects/{sid}/samples
	const prefix = "/v1/subjects/"
	rest := strings.TrimPrefix(urlPath, prefix)
	if rest == urlPath {
		return "" // prefix not found
	}
	// Take everything up to the next slash.
	if idx := strings.Index(rest, "/"); idx >= 0 {
		return rest[:idx]
	}
	return rest
}
