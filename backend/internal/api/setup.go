package api

import (
	"encoding/json"
	"net/http"
	"os"

	"log/slog"

	"github.com/kmacmcfarlane/image-dataset-tool/backend/internal/crypto"
	"github.com/kmacmcfarlane/image-dataset-tool/backend/internal/datadir"
)

// SetupHandler handles setup wizard API endpoints.
type SetupHandler struct {
	dataDir string
	keyPath string
}

// NewSetupHandler creates a new SetupHandler.
func NewSetupHandler(dir string) *SetupHandler {
	return &SetupHandler{
		dataDir: dir,
		keyPath: datadir.SecretKeyPath(dir),
	}
}

// SetupStatusResponse is the JSON response for GET /api/setup/status.
type SetupStatusResponse struct {
	NeedsSetup      bool   `json:"needs_setup"`
	DataDir         string `json:"data_dir"`
	DataDirWritable bool   `json:"data_dir_writable"`
	KeyStatus       string `json:"key_status"`
}

// ServeStatus handles GET /api/setup/status.
// Returns the current setup state so the frontend can decide whether to show
// the setup wizard.
func (h *SetupHandler) ServeStatus(w http.ResponseWriter, r *http.Request) {
	keyStatus := resolveKeyStatus(h.keyPath)

	// Check data dir writability.
	writable := false
	if f, err := os.CreateTemp(h.dataDir, ".setup-check-*"); err == nil {
		name := f.Name()
		f.Close()
		os.Remove(name)
		writable = true
	}

	needsSetup := keyStatus != "found" || !writable

	resp := SetupStatusResponse{
		NeedsSetup:      needsSetup,
		DataDir:         h.dataDir,
		DataDirWritable: writable,
		KeyStatus:       keyStatus,
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		slog.Error("Failed to encode setup status response", "error", err)
	}
}

// GenerateKeyResponse is the JSON response for POST /api/setup/generate-key.
type GenerateKeyResponse struct {
	OK      bool   `json:"ok"`
	Message string `json:"message"`
}

// ServeGenerateKey handles POST /api/setup/generate-key.
// Generates a new 32-byte AES-256 key at $DATA_DIR/secret.key (mode 0600).
// Returns an error if the key already exists.
func (h *SetupHandler) ServeGenerateKey(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if err := crypto.GenerateKey(h.keyPath); err != nil {
		slog.Error("Failed to generate encryption key", "error", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		resp := GenerateKeyResponse{OK: false, Message: err.Error()}
		json.NewEncoder(w).Encode(resp) //nolint:errcheck
		return
	}

	slog.Info("Encryption key generated via setup wizard", "path", h.keyPath)
	w.Header().Set("Content-Type", "application/json")
	resp := GenerateKeyResponse{OK: true, Message: "Key generated successfully at " + h.keyPath}
	json.NewEncoder(w).Encode(resp) //nolint:errcheck
}
