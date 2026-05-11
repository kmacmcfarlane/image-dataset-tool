// Package model contains domain structs used across service and store layers.
// Model types must NOT carry serialization tags.
package model

// Edit represents the edit operations applied to a sample.
// Raw bytes are never mutated; edits are applied at export time.
type Edit struct {
	SampleID       string
	RotationDeg    float64
	CropBox        *CropBox
	AutoActionsJSON string
	CreatedAt      string
	UpdatedAt      string
}

// CropBox represents a crop region in normalized or pixel coordinates.
type CropBox struct {
	X float64
	Y float64
	W float64
	H float64
}

// IsEmpty returns true if the edit has no meaningful operations
// (zero rotation, no crop, no auto actions).
func (e *Edit) IsEmpty() bool {
	return e.RotationDeg == 0 &&
		e.CropBox == nil &&
		(e.AutoActionsJSON == "" || e.AutoActionsJSON == "[]" || e.AutoActionsJSON == "null")
}
