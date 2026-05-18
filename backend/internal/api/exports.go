package api

import (
	"encoding/json"
	"errors"
	"net/http"

	"log/slog"

	"github.com/kmacmcfarlane/image-dataset-tool/backend/internal/service"
)

// CreateExportRequest is the JSON body for POST /v1/exports.
type CreateExportRequest struct {
	SubjectIDs  []string `json:"subject_ids"`
	StudySlug   string   `json:"study_slug"`
	Format      string   `json:"format"`
	Destination string   `json:"destination"`
}

// CreateExportResponse is the JSON response for POST /v1/exports.
type CreateExportResponse struct {
	JobID      string `json:"job_id"`
	TraceID    string `json:"trace_id"`
	Status     string `json:"status"`
	TotalItems int    `json:"total_items"`
	OutputPath string `json:"output_path"`
}

// ExportsHandler serves export API endpoints.
type ExportsHandler struct {
	exportSvc *service.ExportService
}

// NewExportsHandler creates a new exports handler.
func NewExportsHandler(exportSvc *service.ExportService) *ExportsHandler {
	return &ExportsHandler{exportSvc: exportSvc}
}

// ServeCreate handles POST /v1/exports.
func (h *ExportsHandler) ServeCreate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		respondError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var req CreateExportRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	if len(req.SubjectIDs) == 0 {
		respondError(w, http.StatusBadRequest, "subject_ids is required")
		return
	}

	params := service.ExportParams{
		SubjectIDs:  req.SubjectIDs,
		StudySlug:   req.StudySlug,
		Format:      req.Format,
		Destination: req.Destination,
	}

	jr, err := h.exportSvc.TriggerExport(r.Context(), params)
	if err != nil {
		if errors.Is(err, service.ErrBadRequest) {
			respondError(w, http.StatusBadRequest, err.Error())
			return
		}
		if errors.Is(err, service.ErrNotFound) {
			respondError(w, http.StatusNotFound, err.Error())
			return
		}
		slog.Error("Failed to trigger export", "error", err)
		respondError(w, http.StatusInternalServerError, "failed to trigger export")
		return
	}

	totalItems := 0
	if jr.TotalItems != nil {
		totalItems = *jr.TotalItems
	}

	resp := CreateExportResponse{
		JobID:      jr.ID,
		TraceID:    jr.TraceID,
		Status:     jr.Status,
		TotalItems: totalItems,
		OutputPath: jr.OutputPath,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		slog.Error("Failed to encode export response", "error", err)
	}
}
