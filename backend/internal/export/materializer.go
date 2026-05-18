package export

import (
	"bytes"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"io"
	"log/slog"
	"math"
	"os"
	"path/filepath"
	"strings"

	"github.com/kmacmcfarlane/image-dataset-tool/backend/internal/model"
)

// MaterializeResult holds the output of materializing edits for a single sample.
type MaterializeResult struct {
	// Data contains the final image bytes (either raw copy or re-encoded).
	Data []byte
	// Format is the image format of Data (e.g. "jpeg", "png").
	Format string
	// Width is the final image width in pixels.
	Width int
	// Height is the final image height in pixels.
	Height int
}

// EditMaterializer reads a raw image file and applies edits (rotation then crop),
// producing the final image bytes. If no edits are present, the raw bytes are
// returned as-is with no re-encoding.
type EditMaterializer struct {
	dataDir string
	logger  *slog.Logger
}

// NewEditMaterializer creates a new EditMaterializer rooted at dataDir.
func NewEditMaterializer(dataDir string) *EditMaterializer {
	return &EditMaterializer{
		dataDir: dataDir,
		logger:  slog.With("component", "edit_materializer"),
	}
}

// Materialize applies the given edit to the sample's raw image and returns the
// resulting bytes. Order of operations: rotation first, then crop.
//
// If edit is nil or empty (no rotation, no crop), the raw file bytes are copied
// as-is with no re-encoding to preserve original quality.
func (m *EditMaterializer) Materialize(sample *model.Sample, edit *model.Edit) (*MaterializeResult, error) {
	m.logger.Debug("entering Materialize", "sample_id", sample.ID)

	rawPath := filepath.Join(m.dataDir, sample.FilePath)

	// No edits → raw copy (no re-encoding).
	if edit == nil || edit.IsEmpty() {
		return m.rawCopy(rawPath, sample)
	}

	return m.applyEdits(rawPath, sample, edit)
}

// rawCopy reads the file and returns raw bytes without decoding or re-encoding.
func (m *EditMaterializer) rawCopy(path string, sample *model.Sample) (*MaterializeResult, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read raw image %s: %w", path, err)
	}

	format := formatFromExt(filepath.Ext(path))
	m.logger.Debug("raw copy (no edits)", "sample_id", sample.ID, "format", format, "bytes", len(data))

	return &MaterializeResult{
		Data:   data,
		Format: format,
		Width:  sample.Width,
		Height: sample.Height,
	}, nil
}

// applyEdits decodes the image, applies rotation then crop, and re-encodes.
func (m *EditMaterializer) applyEdits(path string, sample *model.Sample, edit *model.Edit) (*MaterializeResult, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open image %s: %w", path, err)
	}
	defer f.Close()

	img, detectedFormat, err := image.Decode(f)
	if err != nil {
		return nil, fmt.Errorf("decode image %s: %w", path, err)
	}

	m.logger.Debug("applying edits",
		"sample_id", sample.ID,
		"rotation_deg", edit.RotationDeg,
		"has_crop", edit.CropBox != nil,
		"src_format", detectedFormat,
	)

	// Step 1: Rotation.
	if edit.RotationDeg != 0 {
		img = rotateImage(img, edit.RotationDeg)
	}

	// Step 2: Crop (coordinates are in pixels, relative to the post-rotation image).
	if edit.CropBox != nil {
		box := pixelRect(edit.CropBox, img.Bounds())
		img = cropImage(img, box)
	}

	// Re-encode in the original format.
	outFormat := formatFromExt(filepath.Ext(path))
	if outFormat == "" {
		outFormat = detectedFormat
	}

	data, err := encodeImage(img, outFormat)
	if err != nil {
		return nil, fmt.Errorf("encode image: %w", err)
	}

	bounds := img.Bounds()
	return &MaterializeResult{
		Data:   data,
		Format: outFormat,
		Width:  bounds.Dx(),
		Height: bounds.Dy(),
	}, nil
}

// pixelRect converts a model.CropBox (pixel coordinates) to an image.Rectangle.
func pixelRect(box *model.CropBox, bounds image.Rectangle) image.Rectangle {
	x := int(math.Round(box.X))
	y := int(math.Round(box.Y))
	w := int(math.Round(box.W))
	h := int(math.Round(box.H))
	return image.Rect(
		bounds.Min.X+x,
		bounds.Min.Y+y,
		bounds.Min.X+x+w,
		bounds.Min.Y+y+h,
	)
}

// encodeImage encodes an image to the specified format.
func encodeImage(img image.Image, format string) ([]byte, error) {
	var buf bytes.Buffer
	var w io.Writer = &buf

	switch strings.ToLower(format) {
	case "jpeg", "jpg":
		if err := jpeg.Encode(w, img, &jpeg.Options{Quality: 95}); err != nil {
			return nil, fmt.Errorf("encode jpeg: %w", err)
		}
	case "png":
		if err := png.Encode(w, img); err != nil {
			return nil, fmt.Errorf("encode png: %w", err)
		}
	default:
		// Default to JPEG for unknown formats.
		if err := jpeg.Encode(w, img, &jpeg.Options{Quality: 95}); err != nil {
			return nil, fmt.Errorf("encode jpeg (fallback): %w", err)
		}
	}

	return buf.Bytes(), nil
}

// formatFromExt returns a normalized format string from a file extension.
func formatFromExt(ext string) string {
	switch strings.ToLower(ext) {
	case ".jpg", ".jpeg":
		return "jpeg"
	case ".png":
		return "png"
	default:
		return ""
	}
}
