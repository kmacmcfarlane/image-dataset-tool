// Package fileformat defines external file format types with JSON serialization tags.
// These types map to/from internal model types and represent the on-disk JSON schemas.
package fileformat

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/kmacmcfarlane/image-dataset-tool/backend/internal/atomicfile"
)

// ProjectManifest represents the project manifest.json on disk.
type ProjectManifest struct {
	ID          string `json:"id"`
	Slug        string `json:"slug"`
	Name        string `json:"name"`
	Description string `json:"description"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
}

// WriteProjectManifest atomically writes a project manifest to the given path.
func WriteProjectManifest(path string, m *ProjectManifest) error {
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal project manifest: %w", err)
	}
	data = append(data, '\n')
	return atomicfile.Write(path, data, 0644)
}

// ReadProjectManifest reads and parses a project manifest from the given path.
func ReadProjectManifest(path string) (*ProjectManifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read project manifest: %w", err)
	}

	var m ProjectManifest
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("unmarshal project manifest: %w", err)
	}
	return &m, nil
}
