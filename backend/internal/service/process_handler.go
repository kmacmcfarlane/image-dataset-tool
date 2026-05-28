package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/jpeg"
	_ "image/png" // register PNG decoder
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/corona10/goimagehash"
	"github.com/google/uuid"
	_ "github.com/jdeng/goheif" // register HEIC image decoder
	_ "golang.org/x/image/webp"  // register WebP image decoder

	"github.com/kmacmcfarlane/image-dataset-tool/backend/internal/fileformat"
	"github.com/kmacmcfarlane/image-dataset-tool/backend/internal/model"
	"github.com/kmacmcfarlane/image-dataset-tool/backend/internal/pipeline"
)

// ProcessPayload is the message payload for the media.process consumer.
// Published by the media.fetch stage after downloading an image to tmp.
type ProcessPayload struct {
	SubjectID      string `json:"subject_id"`
	SourcePlatform string `json:"source_platform"`
	SourcePostID   string `json:"source_post_id"`
	SlideIndex     int    `json:"slide_index"`
	SourceURL      string `json:"source_url"`
	TakenAt        string `json:"taken_at,omitempty"`
	FetchedAt      string `json:"fetched_at"`
	TmpFilePath    string `json:"tmp_file_path"` // absolute path in $DATA_DIR/tmp/<job-id>/
}

// ProcessSampleStore defines persistence methods used by ProcessHandler.
type ProcessSampleStore interface {
	InsertOrIgnore(sample *model.Sample) (bool, error)
	FindDuplicatesBySHA256(subjectID, sha256 string) ([]string, error)
	FindDuplicatesByPHash(subjectID, targetPHash string, threshold int) ([]string, error)
	GetProjectIDForSubject(subjectID string) (string, error)
}

// ProcessSubjectStore resolves subject metadata for path construction.
type ProcessSubjectStore interface {
	Get(id string) (*model.Subject, error)
}

// ProcessProjectStore resolves project metadata for path construction.
type ProcessProjectStore interface {
	Get(id string) (*model.Project, error)
}

// ProcessJobMessageStore stores job log messages.
type ProcessJobMessageStore interface {
	CreateJobMessage(msg *model.JobMessage) error
}

// ProcessConfig holds configuration for the process handler.
type ProcessConfig struct {
	// MaxFileSizeMB is the maximum file size in megabytes. Default: 50.
	MaxFileSizeMB int
	// ThumbnailMaxDim is the maximum dimension for thumbnails. Default: 768.
	ThumbnailMaxDim int
	// PHashThreshold is the Hamming distance threshold for pHash dedup. Default: 6.
	PHashThreshold int
	// AcceptedFormats is the list of accepted image formats. Default: jpeg, png, webp, heic.
	AcceptedFormats []string
}

// DefaultProcessConfig returns configuration with PRD defaults.
func DefaultProcessConfig() ProcessConfig {
	return ProcessConfig{
		MaxFileSizeMB:   50,
		ThumbnailMaxDim: 768,
		PHashThreshold:  6,
		AcceptedFormats: []string{"jpeg", "png", "webp", "heic"},
	}
}

// ProcessHandler is a pipeline.Handler for the media.process consumer.
type ProcessHandler struct {
	sampleStore  ProcessSampleStore
	subjectStore ProcessSubjectStore
	projectStore ProcessProjectStore
	msgStore     ProcessJobMessageStore
	eventSink    pipeline.EventSink
	dataDir      string
	config       ProcessConfig
	logger       *slog.Logger

	// subjectMu provides per-subject mutex for duplicate detection serialization.
	subjectMu sync.Map // map[string]*sync.Mutex
}

// NewProcessHandler creates a handler for the media.process consumer.
func NewProcessHandler(
	sampleStore ProcessSampleStore,
	subjectStore ProcessSubjectStore,
	projectStore ProcessProjectStore,
	msgStore ProcessJobMessageStore,
	eventSink pipeline.EventSink,
	dataDir string,
	config ProcessConfig,
) *ProcessHandler {
	return &ProcessHandler{
		sampleStore:  sampleStore,
		subjectStore: subjectStore,
		projectStore: projectStore,
		msgStore:     msgStore,
		eventSink:    eventSink,
		dataDir:      dataDir,
		config:       config,
		logger:       slog.With("component", "process_handler"),
	}
}

// Handle processes a single media.process message.
func (h *ProcessHandler) Handle(ctx context.Context, env *pipeline.Envelope) error {
	var payload ProcessPayload
	if err := json.Unmarshal(env.Payload, &payload); err != nil {
		return fmt.Errorf("unmarshal process payload: %w", err)
	}

	h.logger.Debug("processing sample",
		"job_id", env.JobID,
		"subject_id", payload.SubjectID,
		"source_post_id", payload.SourcePostID,
		"slide_index", payload.SlideIndex,
	)

	// Validate file exists.
	fi, err := os.Stat(payload.TmpFilePath)
	if err != nil {
		return fmt.Errorf("stat tmp file: %w", err)
	}

	// Check file size.
	maxBytes := int64(h.config.MaxFileSizeMB) * 1024 * 1024
	if fi.Size() > maxBytes {
		h.logger.Warn("file exceeds max size, skipping",
			"path", payload.TmpFilePath,
			"size_bytes", fi.Size(),
			"max_bytes", maxBytes,
		)
		h.logJobMessage(env.JobID, "error", payload.SourcePostID, "",
			fmt.Sprintf("File exceeds max size (%d bytes > %d MB limit)", fi.Size(), h.config.MaxFileSizeMB))
		return nil // ACK to skip
	}

	// Validate format by decoding the image.
	imgFile, err := os.Open(payload.TmpFilePath)
	if err != nil {
		return fmt.Errorf("open tmp file: %w", err)
	}

	img, format, err := image.Decode(imgFile)
	imgFile.Close()
	if err != nil {
		h.logger.Warn("unsupported image format, skipping",
			"path", payload.TmpFilePath,
			"error", err,
		)
		h.logJobMessage(env.JobID, "error", payload.SourcePostID, "",
			fmt.Sprintf("Unsupported image format: %v", err))
		return nil // ACK to skip
	}

	if !h.isAcceptedFormat(format) {
		h.logger.Warn("unsupported format, skipping",
			"path", payload.TmpFilePath,
			"format", format,
		)
		h.logJobMessage(env.JobID, "error", payload.SourcePostID, "",
			fmt.Sprintf("Unsupported format: %s", format))
		return nil // ACK to skip
	}

	// Compute sha256.
	sha256Hash, err := computeSHA256(payload.TmpFilePath)
	if err != nil {
		return fmt.Errorf("compute sha256: %w", err)
	}

	// Compute pHash.
	pHashValue, err := computePHash(img)
	if err != nil {
		return fmt.Errorf("compute phash: %w", err)
	}

	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()

	// Acquire per-subject mutex for duplicate detection serialization.
	mu := h.getSubjectMutex(payload.SubjectID)
	mu.Lock()
	defer mu.Unlock()

	// Dedup check: sha256 exact match + pHash Hamming distance.
	isDuplicate := false
	var duplicateOf []string

	sha256Dups, err := h.sampleStore.FindDuplicatesBySHA256(payload.SubjectID, sha256Hash)
	if err != nil {
		return fmt.Errorf("find sha256 duplicates: %w", err)
	}
	if len(sha256Dups) > 0 {
		isDuplicate = true
		duplicateOf = append(duplicateOf, sha256Dups...)
	}

	if !isDuplicate {
		pHashDups, err := h.sampleStore.FindDuplicatesByPHash(payload.SubjectID, pHashValue, h.config.PHashThreshold)
		if err != nil {
			return fmt.Errorf("find phash duplicates: %w", err)
		}
		if len(pHashDups) > 0 {
			isDuplicate = true
			duplicateOf = append(duplicateOf, pHashDups...)
		}
	}

	if isDuplicate {
		h.logJobMessage(env.JobID, "warning", payload.SourcePostID, "",
			fmt.Sprintf("Duplicate detected (of %v)", duplicateOf))
	}

	// Resolve paths for final location.
	projectID, err := h.sampleStore.GetProjectIDForSubject(payload.SubjectID)
	if err != nil {
		return fmt.Errorf("get project for subject: %w", err)
	}

	project, err := h.projectStore.Get(projectID)
	if err != nil {
		return fmt.Errorf("get project: %w", err)
	}

	subject, err := h.subjectStore.Get(payload.SubjectID)
	if err != nil {
		return fmt.Errorf("get subject: %w", err)
	}

	// Generate sample ID.
	sampleID := uuid.New().String()
	ext := formatToExt(format)
	sampleFilename := sampleID + ext

	// Build final directory and paths.
	samplesDir := filepath.Join(h.dataDir, "projects", project.Slug, "subjects", subject.Slug, "samples")
	if err := os.MkdirAll(samplesDir, 0755); err != nil {
		if isENOSPC(err) {
			return pipeline.ErrDiskFull
		}
		return fmt.Errorf("create samples dir: %w", err)
	}

	finalPath := filepath.Join(samplesDir, sampleFilename)
	thumbPath := filepath.Join(samplesDir, sampleID+"_thumb.jpg")
	manifestPath := filepath.Join(samplesDir, sampleID+".json")

	// Move file to final path.
	if err := moveFile(payload.TmpFilePath, finalPath); err != nil {
		if isENOSPC(err) {
			return pipeline.ErrDiskFull
		}
		return fmt.Errorf("move to final path: %w", err)
	}

	// Generate thumbnail.
	if err := generateThumbnail(img, thumbPath, h.config.ThumbnailMaxDim); err != nil {
		if isENOSPC(err) {
			return pipeline.ErrDiskFull
		}
		return fmt.Errorf("generate thumbnail: %w", err)
	}

	// Build relative file path (relative to data dir).
	relFilePath := filepath.Join("projects", project.Slug, "subjects", subject.Slug, "samples", sampleFilename)

	// Build duplicate_of JSON.
	var duplicateOfJSON string
	if len(duplicateOf) > 0 {
		data, _ := json.Marshal(duplicateOf)
		duplicateOfJSON = string(data)
	}

	now := time.Now().UTC().Format(time.RFC3339)

	// Write sample manifest (atomic rename).
	manifest := &fileformat.SampleMetadata{
		ID: sampleID,
		Source: fileformat.SampleSource{
			Platform:   payload.SourcePlatform,
			PostID:     payload.SourcePostID,
			SlideIndex: payload.SlideIndex,
			URL:        payload.SourceURL,
			TakenAt:    payload.TakenAt,
		},
		File: fileformat.SampleFile{
			Path:   sampleFilename,
			SHA256: sha256Hash,
			PHash:  pHashValue,
			Width:  width,
			Height: height,
		},
		Status:      "pending",
		IsDuplicate: isDuplicate,
		DuplicateOf: duplicateOf,
		Edits: fileformat.SampleEdits{
			AutoActions: []string{},
		},
		Captions:  map[string]fileformat.Caption{},
		FetchedAt: payload.FetchedAt,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if manifest.DuplicateOf == nil {
		manifest.DuplicateOf = []string{}
	}

	if err := fileformat.WriteSampleMetadata(manifestPath, manifest); err != nil {
		if isENOSPC(err) {
			return pipeline.ErrDiskFull
		}
		return fmt.Errorf("write sample manifest: %w", err)
	}

	// Insert sample row (INSERT OR IGNORE for idempotency).
	sample := &model.Sample{
		ID:             sampleID,
		SubjectID:      payload.SubjectID,
		SourcePlatform: payload.SourcePlatform,
		SourcePostID:   payload.SourcePostID,
		SlideIndex:     payload.SlideIndex,
		FilePath:       relFilePath,
		SHA256:         sha256Hash,
		PHash:          pHashValue,
		Width:          width,
		Height:         height,
		TakenAt:        payload.TakenAt,
		FetchedAt:      payload.FetchedAt,
		Status:         "pending",
		IsDuplicate:    isDuplicate,
		DuplicateOf:    duplicateOfJSON,
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	inserted, err := h.sampleStore.InsertOrIgnore(sample)
	if err != nil {
		return fmt.Errorf("insert sample: %w", err)
	}

	if inserted {
		h.logger.Info("sample created",
			"sample_id", sampleID,
			"subject_id", payload.SubjectID,
			"is_duplicate", isDuplicate,
		)

		// Emit sample.new SSE event.
		if h.eventSink != nil {
			relThumbPath := filepath.Join("projects", project.Slug, "subjects", subject.Slug, "samples", sampleID+"_thumb.jpg")
			h.eventSink.Emit(pipeline.SSEEvent{
				Type: "sample.new",
				Data: map[string]any{
					"id":             sampleID,
					"subject_id":     payload.SubjectID,
					"status":         "pending",
					"is_duplicate":   isDuplicate,
					"thumbnail_path": relThumbPath,
				},
			})
		}
	} else {
		h.logger.Debug("sample already exists (redelivery), skipping",
			"source_post_id", payload.SourcePostID,
			"slide_index", payload.SlideIndex,
		)
	}

	return nil
}

// getSubjectMutex returns or creates a mutex for the given subject.
func (h *ProcessHandler) getSubjectMutex(subjectID string) *sync.Mutex {
	val, _ := h.subjectMu.LoadOrStore(subjectID, &sync.Mutex{})
	return val.(*sync.Mutex)
}

// isAcceptedFormat checks if the detected format is in the accepted list.
func (h *ProcessHandler) isAcceptedFormat(format string) bool {
	normalized := strings.ToLower(format)
	for _, f := range h.config.AcceptedFormats {
		if strings.ToLower(f) == normalized {
			return true
		}
	}
	return false
}

// logJobMessage writes a job message to the store, logging errors but not failing.
func (h *ProcessHandler) logJobMessage(jobID, level, sampleRef, accountHandle, message string) {
	if h.msgStore == nil {
		return
	}
	msg := &model.JobMessage{
		ID:            uuid.New().String(),
		JobRunID:      jobID,
		Level:         level,
		SampleRef:     sampleRef,
		AccountHandle: accountHandle,
		Message:       message,
		CreatedAt:     time.Now().UTC().Format(time.RFC3339),
	}
	if err := h.msgStore.CreateJobMessage(msg); err != nil {
		h.logger.Error("failed to write job message", "error", err)
	}
}

// computeSHA256 returns the hex-encoded SHA-256 hash of the file.
func computeSHA256(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	hasher := sha256.New()
	if _, err := io.Copy(hasher, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(hasher.Sum(nil)), nil
}

// computePHash returns the hex-encoded perceptual hash of the image.
func computePHash(img image.Image) (string, error) {
	hash, err := goimagehash.PerceptionHash(img)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%016x", hash.GetHash()), nil
}

// generateThumbnail creates a JPEG thumbnail with the given max dimension.
func generateThumbnail(img image.Image, destPath string, maxDim int) error {
	bounds := img.Bounds()
	origW := bounds.Dx()
	origH := bounds.Dy()

	// Calculate new dimensions maintaining aspect ratio.
	// maxDim applies to the shortest edge per PRD.
	newW, newH := origW, origH
	if origW <= origH {
		// Width is shorter or equal.
		if origW > maxDim {
			newW = maxDim
			newH = origH * maxDim / origW
		}
	} else {
		// Height is shorter.
		if origH > maxDim {
			newH = maxDim
			newW = origW * maxDim / origH
		}
	}

	// Use nearest-neighbor for speed. The thumbnail is for preview only.
	thumb := resizeImage(img, newW, newH)

	// Write to temp file then rename for atomicity.
	dir := filepath.Dir(destPath)
	tmp, err := os.CreateTemp(dir, ".thumb-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()

	success := false
	defer func() {
		if !success {
			os.Remove(tmpName)
		}
	}()

	if err := jpeg.Encode(tmp, thumb, &jpeg.Options{Quality: 80}); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpName, destPath); err != nil {
		return err
	}
	success = true
	return nil
}

// resizeImage scales an image to the target dimensions using simple sampling.
func resizeImage(src image.Image, width, height int) image.Image {
	srcBounds := src.Bounds()
	srcW := srcBounds.Dx()
	srcH := srcBounds.Dy()

	dst := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			srcX := srcBounds.Min.X + x*srcW/width
			srcY := srcBounds.Min.Y + y*srcH/height
			dst.Set(x, y, src.At(srcX, srcY))
		}
	}
	return dst
}

// formatToExt maps Go image format names to file extensions.
func formatToExt(format string) string {
	switch strings.ToLower(format) {
	case "jpeg":
		return ".jpg"
	case "png":
		return ".png"
	case "webp":
		return ".webp"
	case "heic", "heif":
		return ".heic"
	default:
		return "." + format
	}
}

// moveFile moves a file from src to dst, falling back to copy+delete if
// rename fails (cross-device move).
func moveFile(src, dst string) error {
	if err := os.Rename(src, dst); err == nil {
		return nil
	}

	// Cross-device fallback: copy then delete.
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}

	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		os.Remove(dst)
		return err
	}
	if err := out.Sync(); err != nil {
		out.Close()
		os.Remove(dst)
		return err
	}
	if err := out.Close(); err != nil {
		os.Remove(dst)
		return err
	}
	in.Close()
	return os.Remove(src)
}

// isENOSPC checks if an error is a disk-full error.
func isENOSPC(err error) bool {
	// Check for common "no space" patterns in the error chain.
	var pathErr *os.PathError
	if errors.As(err, &pathErr) {
		return strings.Contains(pathErr.Err.Error(), "no space left on device")
	}
	return strings.Contains(err.Error(), "no space left on device")
}
