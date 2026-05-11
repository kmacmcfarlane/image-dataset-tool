// Package model contains domain structs used across service and store layers.
// Model types must NOT carry serialization tags.
package model

// Project represents a project in the domain.
type Project struct {
	ID           string
	Slug         string
	Name         string
	Description  string
	SubjectCount int
	CreatedAt    string
	UpdatedAt    string
}
