// Package model contains domain structs used across service and store layers.
// Model types must NOT carry serialization tags.
package model

// Account represents a social media account in the domain.
type Account struct {
	ID          string
	Platform    string
	Handle      string
	HasCookies  bool
	LastLoginAt string
	CreatedAt   string
	UpdatedAt   string
}

// AccountDetail represents an account with linked subjects.
type AccountDetail struct {
	ID             string
	Platform       string
	Handle         string
	HasCookies     bool
	LastLoginAt    string
	LinkedSubjects []AccountLinkedSubject
	CreatedAt      string
	UpdatedAt      string
}

// AccountLinkedSubject represents a subject linked to an account.
type AccountLinkedSubject struct {
	ID        string
	ProjectID string
	Name      string
}
