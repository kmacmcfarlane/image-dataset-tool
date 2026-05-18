package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/sirupsen/logrus"

	"github.com/kmacmcfarlane/image-dataset-tool/backend/internal/model"
	"github.com/kmacmcfarlane/image-dataset-tool/backend/internal/store"
)

// CaptionStudyResponse is the JSON representation of a caption study.
type CaptionStudyResponse struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Slug     string `json:"slug"`
	Provider string `json:"provider"`
	Model    string `json:"model"`
}

// CaptionResponse is the JSON representation of a single caption.
type CaptionResponse struct {
	ID        string                `json:"id"`
	SampleID  string                `json:"sample_id"`
	StudyID   string                `json:"study_id"`
	Text      string                `json:"text"`
	Study     CaptionStudyResponse  `json:"study"`
	CreatedAt string                `json:"created_at"`
}

// CaptionListResponse wraps captions for a sample.
type CaptionListResponse struct {
	Captions []CaptionResponse `json:"captions"`
}

// SaveCaptionRequest is the JSON body for PUT /v1/samples/{id}/captions.
type SaveCaptionRequest struct {
	// Text is the new caption text.
	Text string `json:"text"`
	// StudyID is the study to save the caption under. If empty and IsManual is
	// true, the manual pseudo-study is auto-created.
	StudyID string `json:"study_id"`
	// IsManual marks this as a manual edit (uses the manual pseudo-study).
	IsManual bool `json:"is_manual"`
}

// CaptionsHandler serves caption operations.
type CaptionsHandler struct {
	captionStore *store.CaptionStore
	sampleStore  *store.SampleStore
}

// NewCaptionsHandler creates a new captions handler.
func NewCaptionsHandler(captionStore *store.CaptionStore, sampleStore *store.SampleStore) *CaptionsHandler {
	return &CaptionsHandler{
		captionStore: captionStore,
		sampleStore:  sampleStore,
	}
}

// ServeListCaptions handles GET /v1/samples/{id}/captions.
func (h *CaptionsHandler) ServeListCaptions(w http.ResponseWriter, r *http.Request) {
	sampleID := extractSampleIDFromCaptionsPath(r.URL.Path)
	if sampleID == "" || !sampleIDPattern.MatchString(sampleID) {
		respondError(w, http.StatusBadRequest, "missing or invalid sample id")
		return
	}

	// Verify sample exists.
	_, err := h.sampleStore.GetByID(sampleID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			respondError(w, http.StatusNotFound, "sample not found")
			return
		}
		logrus.WithError(err).WithField("sample_id", sampleID).Error("Failed to get sample")
		respondError(w, http.StatusInternalServerError, "failed to get sample")
		return
	}

	captions, err := h.captionStore.ListBySampleID(sampleID)
	if err != nil {
		logrus.WithError(err).WithField("sample_id", sampleID).Error("Failed to list captions")
		respondError(w, http.StatusInternalServerError, "failed to list captions")
		return
	}

	resp := CaptionListResponse{
		Captions: make([]CaptionResponse, 0, len(captions)),
	}
	for _, cws := range captions {
		resp.Captions = append(resp.Captions, mapCaptionWithStudyToResponse(cws))
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		logrus.WithError(err).Error("Failed to encode captions response")
	}
}

// ServeSaveCaption handles PUT /v1/samples/{id}/captions.
func (h *CaptionsHandler) ServeSaveCaption(w http.ResponseWriter, r *http.Request) {
	sampleID := extractSampleIDFromCaptionsPath(r.URL.Path)
	if sampleID == "" || !sampleIDPattern.MatchString(sampleID) {
		respondError(w, http.StatusBadRequest, "missing or invalid sample id")
		return
	}

	var req SaveCaptionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	// Verify sample exists and get its subject for project lookup.
	sample, err := h.sampleStore.GetByID(sampleID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			respondError(w, http.StatusNotFound, "sample not found")
			return
		}
		logrus.WithError(err).WithField("sample_id", sampleID).Error("Failed to get sample")
		respondError(w, http.StatusInternalServerError, "failed to get sample")
		return
	}

	studyID := req.StudyID

	// For manual edits, auto-create the manual pseudo-study.
	if req.IsManual {
		// Need to find the project_id for this sample's subject.
		projectID, err := h.resolveProjectID(sample.SubjectID)
		if err != nil {
			logrus.WithError(err).WithField("subject_id", sample.SubjectID).Error("Failed to resolve project")
			respondError(w, http.StatusInternalServerError, "failed to resolve project")
			return
		}

		manualStudy, err := h.captionStore.GetOrCreateManualStudy(projectID)
		if err != nil {
			logrus.WithError(err).Error("Failed to get/create manual study")
			respondError(w, http.StatusInternalServerError, "failed to create manual study")
			return
		}
		studyID = manualStudy.ID
	}

	if studyID == "" {
		respondError(w, http.StatusBadRequest, "study_id is required (or set is_manual=true)")
		return
	}

	caption := &model.Caption{
		SampleID: sampleID,
		StudyID:  studyID,
		Text:     req.Text,
	}

	result, err := h.captionStore.UpsertCaption(caption)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			respondError(w, http.StatusNotFound, "sample or study not found")
			return
		}
		logrus.WithError(err).WithField("sample_id", sampleID).Error("Failed to save caption")
		respondError(w, http.StatusInternalServerError, "failed to save caption")
		return
	}

	// Fetch the study for the response.
	captions, err := h.captionStore.ListBySampleID(sampleID)
	if err != nil {
		logrus.WithError(err).Error("Failed to list captions after save")
		respondError(w, http.StatusInternalServerError, "failed to list captions")
		return
	}

	// Find the matching caption in the list to get study info.
	var resp CaptionResponse
	for _, cws := range captions {
		if cws.Caption.ID == result.ID {
			resp = mapCaptionWithStudyToResponse(cws)
			break
		}
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		logrus.WithError(err).Error("Failed to encode caption response")
	}
}

// resolveProjectID finds the project_id for a given subject_id.
func (h *CaptionsHandler) resolveProjectID(subjectID string) (string, error) {
	return h.sampleStore.GetProjectIDForSubject(subjectID)
}

// extractSampleIDFromCaptionsPath extracts the sample ID from /v1/samples/{id}/captions.
func extractSampleIDFromCaptionsPath(urlPath string) string {
	const prefix = "/v1/samples/"
	rest := strings.TrimPrefix(urlPath, prefix)
	if rest == urlPath {
		return ""
	}
	if before, after, found := strings.Cut(rest, "/"); found {
		if after == "captions" {
			return before
		}
	}
	return ""
}

// mapCaptionWithStudyToResponse converts a CaptionWithStudy to the API response.
func mapCaptionWithStudyToResponse(cws model.CaptionWithStudy) CaptionResponse {
	return CaptionResponse{
		ID:       cws.Caption.ID,
		SampleID: cws.Caption.SampleID,
		StudyID:  cws.Caption.StudyID,
		Text:     cws.Caption.Text,
		Study: CaptionStudyResponse{
			ID:       cws.Study.ID,
			Name:     cws.Study.Name,
			Slug:     cws.Study.Slug,
			Provider: cws.Study.Provider,
			Model:    cws.Study.Model,
		},
		CreatedAt: cws.Caption.CreatedAt,
	}
}
