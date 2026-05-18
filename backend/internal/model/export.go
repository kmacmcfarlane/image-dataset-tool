// Package model contains domain structs used across service and store layers.
// Model types must NOT carry serialization tags.
package model

// ExportOptions configures an export run.
type ExportOptions struct {
	ProjectSlug string
	SubjectSlug string
	StudySlug   string
	Format      string // e.g. "kohya-ss"
	OutputDir   string // absolute path for export output
}

// ExportResult summarizes the outcome of an export run.
type ExportResult struct {
	OutputDir    string
	SampleCount  int
	CaptionCount int
	SkippedCount int
	Errors       []string
}
