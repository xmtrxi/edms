package database

import "time"

// PDFJob is one run of a PDF tool. It produces exactly one output: a new
// version of the document, or for merges a new document.
type PDFJob struct {
	ID               RecordID       `json:"id,omitempty"`
	OrgID            string         `json:"org_id"`
	UserID           string         `json:"user_id"`
	Operation        string         `json:"operation"`
	DocumentIDs      []string       `json:"document_ids"`
	Options          map[string]any `json:"options,omitempty"`
	FolderID         string         `json:"folder_id,omitempty"`
	OutputName       string         `json:"output_name,omitempty"`
	Status           string         `json:"status"`
	Attempts         int            `json:"attempts"`
	Error            string         `json:"error,omitempty"`
	ResultDocumentID string         `json:"result_document_id,omitempty"`
	ResultVersionID  string         `json:"result_version_id,omitempty"`
	IdempotencyKey   string         `json:"idempotency_key,omitempty"`
	CreatedAt        time.Time      `json:"created_at"`
	UpdatedAt        time.Time      `json:"updated_at"`
	CompletedAt      *time.Time     `json:"completed_at,omitempty"`
}

const (
	PDFJobQueued    = "queued"
	PDFJobRunning   = "running"
	PDFJobRetrying  = "retrying"
	PDFJobSucceeded = "succeeded"
	PDFJobFailed    = "failed"
)
