// Package export provides the DatasetWriter interface and edit materializer
// for producing training-ready datasets from curated samples.
package export

import (
	"context"

	"github.com/kmacmcfarlane/image-dataset-tool/backend/internal/model"
)

// DatasetWriter writes a dataset in a specific format (e.g. kohya-ss).
// Implementations receive samples with edits already materialized.
type DatasetWriter interface {
	// Write exports samples for the given options, materializing edits and
	// writing paired captions. Returns a summary of the export run.
	Write(ctx context.Context, opts model.ExportOptions) (*model.ExportResult, error)
}
