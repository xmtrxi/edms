package database

import "time"

type ProcessingJob struct {
	ID          RecordID       `json:"id,omitempty"`
	DocumentID  string         `json:"document_id"`
	Type        string         `json:"type"`
	Status      string         `json:"status"`
	Attempts    int            `json:"attempts"`
	LastError   string         `json:"last_error,omitempty"`
	ExternalID  string         `json:"external_id,omitempty"`
	Result      map[string]any `json:"result,omitempty"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	CompletedAt *time.Time     `json:"completed_at,omitempty"`
}

const (
	ProcessingJobStatusQueued    = "queued"
	ProcessingJobStatusRunning   = "running"
	ProcessingJobStatusCompleted = "completed"
	ProcessingJobStatusFailed    = "failed"
)
