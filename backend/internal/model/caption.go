// Package model contains domain structs used across service and store layers.
// Model types must NOT carry serialization tags.
package model

// CaptionStudy represents a caption study preset (prompt template + provider + model).
type CaptionStudy struct {
	ID             string
	ProjectID      string
	Name           string
	Slug           string
	Provider       string
	Model          string
	PromptTemplate string
	ParamsJSON     string
	CreatedAt      string
	UpdatedAt      string
}

// Caption represents a caption text for a sample under a specific study.
type Caption struct {
	ID        string
	SampleID  string
	StudyID   string
	Text      string
	CreatedAt string
}

// CaptionWithStudy combines a caption with its parent study metadata.
type CaptionWithStudy struct {
	Caption Caption
	Study   CaptionStudy
}

// ManualStudySlug is the slug used for the "manual" pseudo-study that stores
// inline user edits. The original auto-generated caption is preserved under
// its original study.
const ManualStudySlug = "manual"
