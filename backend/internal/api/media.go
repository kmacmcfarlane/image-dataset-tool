package api

import (
	"fmt"
	"image"
	"image/jpeg"
	_ "image/png"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"log/slog"
)

const thumbnailMaxDim = 256

// sampleIDPattern validates that a sample ID contains only hex characters and hyphens (UUID format).
var sampleIDPattern = regexp.MustCompile(`^[a-fA-F0-9-]+$`)

// MediaResolver looks up file paths for a given sample ID.
type MediaResolver interface {
	// ResolveMedia returns the path to the original media file for a sample.
	// Returns ("", false) if the sample does not exist.
	ResolveMedia(sampleID string) (path string, found bool)
	// ResolveThumbnail returns the path to the thumbnail for a sample.
	// The file may or may not exist on disk.
	ResolveThumbnail(sampleID string) (path string, found bool)
}

// FileMediaResolver implements MediaResolver using the data directory structure.
type FileMediaResolver struct {
	dataDir string
}

// NewFileMediaResolver creates a resolver rooted at the given data directory.
func NewFileMediaResolver(dataDir string) *FileMediaResolver {
	return &FileMediaResolver{dataDir: dataDir}
}

// ResolveMedia finds the original image for a sample by scanning the projects
// directory for a file matching the sample ID (any extension).
func (r *FileMediaResolver) ResolveMedia(sampleID string) (string, bool) {
	// Convention: $DATA_DIR/projects/*/subjects/*/samples/<sample_id>.*
	pattern := filepath.Join(r.dataDir, "projects", "*", "subjects", "*", "samples", sampleID+".*")
	matches, err := filepath.Glob(pattern)
	if err != nil || len(matches) == 0 {
		return "", false
	}
	// Filter out thumbnail files.
	for _, m := range matches {
		if !strings.HasSuffix(m, ".thumb.jpg") {
			return m, true
		}
	}
	return "", false
}

// ResolveThumbnail returns the thumbnail path for a sample.
func (r *FileMediaResolver) ResolveThumbnail(sampleID string) (string, bool) {
	// First find the original to determine base path.
	orig, found := r.ResolveMedia(sampleID)
	if !found {
		return "", false
	}
	// Thumbnail is stored alongside the original as <sample_id>.thumb.jpg.
	dir := filepath.Dir(orig)
	thumbPath := filepath.Join(dir, sampleID+".thumb.jpg")
	return thumbPath, true
}

// MediaHandler serves raw images and thumbnails via HTTP.
type MediaHandler struct {
	resolver MediaResolver
}

// NewMediaHandler creates a new media handler.
func NewMediaHandler(resolver MediaResolver) *MediaHandler {
	return &MediaHandler{resolver: resolver}
}

// ServeImage handles GET /v1/media/{sample_id}.
func (h *MediaHandler) ServeImage(w http.ResponseWriter, r *http.Request) {
	sampleID := extractSampleID(r.URL.Path, "/v1/media/")
	if sampleID == "" {
		respondError(w, http.StatusBadRequest, "missing sample_id")
		return
	}
	if !sampleIDPattern.MatchString(sampleID) {
		respondError(w, http.StatusBadRequest, "invalid sample_id")
		return
	}

	path, found := h.resolver.ResolveMedia(sampleID)
	if !found {
		respondError(w, http.StatusNotFound, "sample not found")
		return
	}

	// Detect content type from file extension.
	ct := contentTypeFromExt(filepath.Ext(path))
	w.Header().Set("Content-Type", ct)
	http.ServeFile(w, r, path)
}

// ServeThumbnail handles GET /v1/media/{sample_id}/thumb.
func (h *MediaHandler) ServeThumbnail(w http.ResponseWriter, r *http.Request) {
	// Extract sample_id: path is /v1/media/{sample_id}/thumb
	trimmed := strings.TrimPrefix(r.URL.Path, "/v1/media/")
	trimmed = strings.TrimSuffix(trimmed, "/thumb")
	sampleID := trimmed
	if sampleID == "" {
		respondError(w, http.StatusBadRequest, "missing sample_id")
		return
	}
	if !sampleIDPattern.MatchString(sampleID) {
		respondError(w, http.StatusBadRequest, "invalid sample_id")
		return
	}

	thumbPath, found := h.resolver.ResolveThumbnail(sampleID)
	if !found {
		respondError(w, http.StatusNotFound, "sample not found")
		return
	}

	// If thumbnail doesn't exist on disk, regenerate it.
	if _, err := os.Stat(thumbPath); os.IsNotExist(err) {
		slog.Warn("Thumbnail missing — regenerating", "sample_id", sampleID)
		origPath, _ := h.resolver.ResolveMedia(sampleID)
		if err := generateThumbnail(origPath, thumbPath); err != nil {
			slog.Error("Failed to regenerate thumbnail", "error", err, "sample_id", sampleID)
			respondError(w, http.StatusInternalServerError, "failed to generate thumbnail")
			return
		}
	}

	w.Header().Set("Content-Type", "image/jpeg")
	http.ServeFile(w, r, thumbPath)
}

// generateThumbnail creates a JPEG thumbnail from the source image.
func generateThumbnail(srcPath, dstPath string) error {
	f, err := os.Open(srcPath)
	if err != nil {
		return fmt.Errorf("open source: %w", err)
	}
	defer f.Close()

	img, format, err := image.Decode(f)
	if err != nil {
		return fmt.Errorf("decode image (format=%s): %w", format, err)
	}

	// Simple nearest-neighbor resize (no external dependency).
	thumb := resizeImage(img, thumbnailMaxDim)

	if err := os.MkdirAll(filepath.Dir(dstPath), 0755); err != nil {
		return fmt.Errorf("create thumbnail dir: %w", err)
	}

	out, err := os.Create(dstPath)
	if err != nil {
		return fmt.Errorf("create thumbnail file: %w", err)
	}
	defer out.Close()

	if err := jpeg.Encode(out, thumb, &jpeg.Options{Quality: 80}); err != nil {
		return fmt.Errorf("encode jpeg: %w", err)
	}
	return nil
}

// resizeImage scales an image so the largest dimension is maxDim, preserving aspect ratio.
func resizeImage(src image.Image, maxDim int) image.Image {
	bounds := src.Bounds()
	w, h := bounds.Dx(), bounds.Dy()

	if w <= maxDim && h <= maxDim {
		return src
	}

	var newW, newH int
	if w >= h {
		newW = maxDim
		newH = h * maxDim / w
	} else {
		newH = maxDim
		newW = w * maxDim / h
	}
	if newW < 1 {
		newW = 1
	}
	if newH < 1 {
		newH = 1
	}

	dst := image.NewRGBA(image.Rect(0, 0, newW, newH))
	for y := 0; y < newH; y++ {
		for x := 0; x < newW; x++ {
			srcX := x * w / newW
			srcY := y * h / newH
			dst.Set(x, y, src.At(bounds.Min.X+srcX, bounds.Min.Y+srcY))
		}
	}
	return dst
}

// extractSampleID extracts the sample_id from a URL path given a prefix.
func extractSampleID(urlPath, prefix string) string {
	rest := strings.TrimPrefix(urlPath, prefix)
	// Take everything up to the next slash or end.
	if idx := strings.Index(rest, "/"); idx >= 0 {
		return rest[:idx]
	}
	return rest
}

func contentTypeFromExt(ext string) string {
	switch strings.ToLower(ext) {
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".png":
		return "image/png"
	case ".gif":
		return "image/gif"
	case ".webp":
		return "image/webp"
	case ".mp4":
		return "video/mp4"
	default:
		return "application/octet-stream"
	}
}

