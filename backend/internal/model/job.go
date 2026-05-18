// Package model contains domain structs used across service and store layers.
// Model types must NOT carry serialization tags.
package model

// JobRun represents a single pipeline job run.
type JobRun struct {
	ID               string
	Type             string
	SubjectID        string // optional
	StudyID          string // optional
	Status           string // running | succeeded | failed | cancelled
	TotalItems       *int   // nullable — unknown until pagination exhausted
	CompletedItems   int
	FailedItems      int
	TraceID          string // optional
	StartedAt        string
	FinishedAt       string // optional
	CreatedAt        string
}

// JobMessage is a single log entry attached to a JobRun.
type JobMessage struct {
	ID            string
	JobRunID      string
	Level         string // error | warning | info
	SampleRef     string // optional
	AccountHandle string // optional
	Message       string
	CreatedAt     string
}
