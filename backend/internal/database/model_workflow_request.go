package database

import "time"

type WorkflowRequest struct {
	ID                RecordID  `json:"id,omitempty"`
	OrgID             string    `json:"org_id"`
	DocumentID        string    `json:"document_id"`
	StartedVersionID  string    `json:"started_version_id"`
	CurrentVersionID  string    `json:"current_version_id"`
	CreatedBy         string    `json:"created_by"`
	Title             string    `json:"title"`
	Status            string    `json:"status"`
	Priority          string    `json:"priority"`
	CurrentStep       int       `json:"current_step"`
	ReturnedVersionID string    `json:"returned_version_id,omitempty"`
	DueAt             time.Time `json:"due_at,omitempty"`
	CompletedAt       time.Time `json:"completed_at,omitempty"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}

type WorkflowStep struct {
	ID                   RecordID  `json:"id,omitempty"`
	WorkflowRequestID    string    `json:"workflow_request_id"`
	Sequence             int       `json:"sequence"`
	Name                 string    `json:"name"`
	ActionType           string    `json:"action_type"`
	AssignmentType       string    `json:"assignment_type"`
	AssignedUserID       string    `json:"assigned_user_id,omitempty"`
	AssignedDepartmentID string    `json:"assigned_department_id,omitempty"`
	ExternalEmail        string    `json:"external_email,omitempty"`
	Status               string    `json:"status"`
	RequireComment       bool      `json:"require_comment"`
	RequireNewVersion    bool      `json:"require_new_version"`
	SigningRequestID     string    `json:"signing_request_id,omitempty"`
	ActivatedAt          time.Time `json:"activated_at,omitempty"`
	CompletedAt          time.Time `json:"completed_at,omitempty"`
	CreatedAt            time.Time `json:"created_at"`
}

const (
	WorkflowStatusDraft               = "draft"
	WorkflowStatusPending             = "pending"
	WorkflowStatusActive              = "active"
	WorkflowStatusWaitingForChanges   = "waiting_for_changes"
	WorkflowStatusWaitingForSignature = "waiting_for_signature"
	WorkflowStatusApproved            = "approved"
	WorkflowStatusRejected            = "rejected"
	WorkflowStatusCompleted           = "completed"
	WorkflowStatusCanceled            = "canceled"

	WorkflowStepPending   = "pending"
	WorkflowStepActive    = "active"
	WorkflowStepApproved  = "approved"
	WorkflowStepRejected  = "rejected"
	WorkflowStepReturned  = "returned"
	WorkflowStepCompleted = "completed"
)
