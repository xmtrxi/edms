package database

import "time"

type WorkflowRequest struct {
	ID         RecordID  `json:"id,omitempty"`
	DocumentID string    `json:"document_id"`
	CreatedBy  string    `json:"created_by"`
	Status     string    `json:"status"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

const (
	WorkflowStatusPending  = "pending"
	WorkflowStatusApproved = "approved"
	WorkflowStatusRejected = "rejected"
	WorkflowStatusCanceled = "canceled"
)
