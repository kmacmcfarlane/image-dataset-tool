package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/sirupsen/logrus"

	"github.com/kmacmcfarlane/image-dataset-tool/backend/internal/model"
	"github.com/kmacmcfarlane/image-dataset-tool/backend/internal/service"
	"github.com/kmacmcfarlane/image-dataset-tool/backend/internal/store"
)

// EditRequest is the JSON body for PUT /v1/samples/{id}/edits.
type EditRequest struct {
	RotationDeg float64         `json:"rotation_deg"`
	CropBox     *CropBoxRequest `json:"crop_box"`
	AutoActions []string        `json:"auto_actions"`
}

// CropBoxRequest represents a crop region in the API request.
type CropBoxRequest struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
	W float64 `json:"w"`
	H float64 `json:"h"`
}

// EditResponse is the JSON response for edit operations.
type EditResponse struct {
	SampleID    string          `json:"sample_id"`
	RotationDeg float64         `json:"rotation_deg"`
	CropBox     *CropBoxRequest `json:"crop_box"`
	AutoActions []string        `json:"auto_actions"`
	CreatedAt   string          `json:"created_at"`
	UpdatedAt   string          `json:"updated_at"`
}

// EditsHandler serves edit CRUD operations.
type EditsHandler struct {
	editService *service.EditService
}

// NewEditsHandler creates a new edits handler.
func NewEditsHandler(editService *service.EditService) *EditsHandler {
	return &EditsHandler{editService: editService}
}

// ServeGetEdits handles GET /v1/samples/{id}/edits.
func (h *EditsHandler) ServeGetEdits(w http.ResponseWriter, r *http.Request) {
	sampleID := extractSampleIDFromEditsPath(r.URL.Path)
	if sampleID == "" || !sampleIDPattern.MatchString(sampleID) {
		respondError(w, http.StatusBadRequest, "missing or invalid sample id")
		return
	}

	edit, err := h.editService.GetEdit(sampleID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			respondError(w, http.StatusNotFound, "sample not found")
			return
		}
		logrus.WithError(err).WithField("sample_id", sampleID).Error("Failed to get edits")
		respondError(w, http.StatusInternalServerError, "failed to get edits")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if edit == nil {
		// No edits: return defaults.
		if err := json.NewEncoder(w).Encode(EditResponse{
			SampleID:    sampleID,
			RotationDeg: 0,
			CropBox:     nil,
			AutoActions: []string{},
		}); err != nil {
			logrus.WithError(err).Error("Failed to encode edits response")
		}
		return
	}

	if err := json.NewEncoder(w).Encode(mapEditToResponse(edit)); err != nil {
		logrus.WithError(err).Error("Failed to encode edits response")
	}
}

// ServePutEdits handles PUT /v1/samples/{id}/edits.
func (h *EditsHandler) ServePutEdits(w http.ResponseWriter, r *http.Request) {
	sampleID := extractSampleIDFromEditsPath(r.URL.Path)
	if sampleID == "" || !sampleIDPattern.MatchString(sampleID) {
		respondError(w, http.StatusBadRequest, "missing or invalid sample id")
		return
	}

	var req EditRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	edit := &model.Edit{
		SampleID:    sampleID,
		RotationDeg: req.RotationDeg,
	}

	if req.CropBox != nil {
		edit.CropBox = &model.CropBox{
			X: req.CropBox.X,
			Y: req.CropBox.Y,
			W: req.CropBox.W,
			H: req.CropBox.H,
		}
	}

	if len(req.AutoActions) > 0 {
		data, _ := json.Marshal(req.AutoActions)
		edit.AutoActionsJSON = string(data)
	}

	result, err := h.editService.SaveEdit(edit)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			respondError(w, http.StatusNotFound, "sample not found")
			return
		}
		logrus.WithError(err).WithField("sample_id", sampleID).Error("Failed to save edits")
		respondError(w, http.StatusInternalServerError, "failed to save edits")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if result == nil {
		// Edits were empty (reset).
		if err := json.NewEncoder(w).Encode(EditResponse{
			SampleID:    sampleID,
			RotationDeg: 0,
			CropBox:     nil,
			AutoActions: []string{},
		}); err != nil {
			logrus.WithError(err).Error("Failed to encode edits response")
		}
		return
	}

	if err := json.NewEncoder(w).Encode(mapEditToResponse(result)); err != nil {
		logrus.WithError(err).Error("Failed to encode edits response")
	}
}

// ServeDeleteEdits handles DELETE /v1/samples/{id}/edits (reset).
func (h *EditsHandler) ServeDeleteEdits(w http.ResponseWriter, r *http.Request) {
	sampleID := extractSampleIDFromEditsPath(r.URL.Path)
	if sampleID == "" || !sampleIDPattern.MatchString(sampleID) {
		respondError(w, http.StatusBadRequest, "missing or invalid sample id")
		return
	}

	_, err := h.editService.ResetEdits(sampleID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			respondError(w, http.StatusNotFound, "sample not found")
			return
		}
		logrus.WithError(err).WithField("sample_id", sampleID).Error("Failed to reset edits")
		respondError(w, http.StatusInternalServerError, "failed to reset edits")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// extractSampleIDFromEditsPath extracts the sample ID from path /v1/samples/{id}/edits.
func extractSampleIDFromEditsPath(urlPath string) string {
	const prefix = "/v1/samples/"
	rest := strings.TrimPrefix(urlPath, prefix)
	if rest == urlPath {
		return ""
	}
	// Take everything up to "/edits".
	if before, after, found := strings.Cut(rest, "/"); found {
		if after == "edits" {
			return before
		}
	}
	return ""
}

// mapEditToResponse converts a model.Edit to the API response shape.
func mapEditToResponse(edit *model.Edit) EditResponse {
	resp := EditResponse{
		SampleID:    edit.SampleID,
		RotationDeg: edit.RotationDeg,
		AutoActions: []string{},
		CreatedAt:   edit.CreatedAt,
		UpdatedAt:   edit.UpdatedAt,
	}

	if edit.CropBox != nil {
		resp.CropBox = &CropBoxRequest{
			X: edit.CropBox.X,
			Y: edit.CropBox.Y,
			W: edit.CropBox.W,
			H: edit.CropBox.H,
		}
	}

	if edit.AutoActionsJSON != "" && edit.AutoActionsJSON != "[]" && edit.AutoActionsJSON != "null" {
		var actions []string
		if err := json.Unmarshal([]byte(edit.AutoActionsJSON), &actions); err == nil && len(actions) > 0 {
			resp.AutoActions = actions
		}
	}

	return resp
}
