// Package model contains domain structs used across service and store layers.
// Model types must NOT carry serialization tags.
package model

// Subject represents a subject within a project.
type Subject struct {
	ID             string
	ProjectID      string
	Slug           string
	Name           string
	LinkedAccounts []LinkedAccount
	SampleStats    SampleStats
	CreatedAt      string
	UpdatedAt      string
}

// LinkedAccount represents an account linked to a subject.
type LinkedAccount struct {
	ID       string
	Platform string
	Handle   string
}

// SampleStats holds sample counts by status.
type SampleStats struct {
	Pending  int
	Kept     int
	Rejected int
	Total    int
}
