package database

import "time"

type SigningRequest struct {
	ID                 RecordID  `json:"id,omitempty"`
	DocumentID         string    `json:"document_id"`
	RequestedBy        string    `json:"requested_by"`
	Provider           string    `json:"provider"`
	ProviderDocumentID string    `json:"provider_document_id"`
	Status             string    `json:"status"`
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
