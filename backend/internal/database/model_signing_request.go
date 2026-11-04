package database

import "time"

type SigningRequest struct {
	ID                 RecordID  `json:"id,omitempty"`
	DocumentID         string    `json:"document_id"`
	DocumentVersionID  string    `json:"document_version_id"`
	WorkflowRequestID  string    `json:"workflow_request_id"`
	WorkflowStepID     string    `json:"workflow_step_id"`
	RequestedBy        string    `json:"requested_by"`
	Provider           string    `json:"provider"`
	ProviderDocumentID string    `json:"provider_document_id"`
	SigningURL         string    `json:"signing_url,omitempty"`
	Status             string    `json:"status"`
	CompletedVersionID string    `json:"completed_version_id,omitempty"`
	CreatedAt          time.Time `json:"created_at"`
	UpdatedAt          time.Time `json:"updated_at"`
}

const (
	SigningStatusDraft     = "draft"
	SigningStatusSent      = "sent"
	SigningStatusCompleted = "completed"
	SigningStatusCancelled = "cancelled"
	SigningStatusFailed    = "failed"
)
