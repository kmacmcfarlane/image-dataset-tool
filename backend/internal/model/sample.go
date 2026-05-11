// Package model contains domain structs used across service and store layers.
// Model types must NOT carry serialization tags.
package model

// Sample represents a media sample within a subject.
type Sample struct {
	ID             string
	SubjectID      string
	SourcePlatform string
	SourcePostID   string
	SlideIndex     int
	FilePath       string
	SHA256         string
	PHash          string
	Width          int
	Height         int
	TakenAt        string
	FetchedAt      string
	Status         string
	IsDuplicate    bool
	DuplicateOf    string // JSON array of sample UUIDs, nullable
	CreatedAt      string
	UpdatedAt      string
}
