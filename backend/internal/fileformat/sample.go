package fileformat

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/kmacmcfarlane/image-dataset-tool/backend/internal/atomicfile"
)

// SampleMetadata represents a sample's .json metadata file on disk.
type SampleMetadata struct {
	ID          string            `json:"id"`
	Source      SampleSource      `json:"source"`
	File        SampleFile        `json:"file"`
	Status      string            `json:"status"`
	IsDuplicate bool              `json:"is_duplicate"`
	DuplicateOf []string          `json:"duplicate_of"`
	Edits       SampleEdits       `json:"edits"`
	Captions    map[string]Caption `json:"captions"`
	FetchedAt   string            `json:"fetched_at"`
	CreatedAt   string            `json:"created_at"`
	UpdatedAt   string            `json:"updated_at"`
}

// SampleSource holds the source platform information for a sample.
type SampleSource struct {
	Platform   string `json:"platform"`
	PostID     string `json:"post_id"`
	SlideIndex int    `json:"slide_index"`
	URL        string `json:"url"`
	TakenAt    string `json:"taken_at,omitempty"`
}

// SampleFile holds file-level metadata for a sample.
type SampleFile struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
	PHash  string `json:"phash"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
}

// SampleEdits holds edit metadata for a sample.
type SampleEdits struct {
	RotationDeg float64  `json:"rotation_deg"`
	CropBox     *CropBox `json:"crop_box"`
	AutoActions []string `json:"auto_actions"`
}

// CropBox represents a crop region.
type CropBox struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
	W float64 `json:"w"`
	H float64 `json:"h"`
}

// Caption represents a single caption entry keyed by study slug.
type Caption struct {
	StudyID   string `json:"study_id"`
	Text      string `json:"text"`
	CreatedAt string `json:"created_at"`
}

// WriteSampleMetadata atomically writes sample metadata to the given path.
func WriteSampleMetadata(path string, s *SampleMetadata) error {
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal sample metadata: %w", err)
	}
	data = append(data, '\n')
	return atomicfile.Write(path, data, 0644)
}

// ReadSampleMetadata reads and parses sample metadata from the given path.
func ReadSampleMetadata(path string) (*SampleMetadata, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read sample metadata: %w", err)
	}

	var s SampleMetadata
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("unmarshal sample metadata: %w", err)
	}
	return &s, nil
}
