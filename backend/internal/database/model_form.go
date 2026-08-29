package database

import "time"

type FormTemplate struct {
	ID           RecordID       `json:"id,omitempty"`
	DepartmentID string         `json:"department_id"`
	Name         string         `json:"name"`
	Description  string         `json:"description"`
	Status       string         `json:"status"`
	Version      int            `json:"version"`
	Definition   map[string]any `json:"definition"`
	CreatedBy    string         `json:"created_by"`
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
}

type FormSubmission struct {
	ID              RecordID       `json:"id,omitempty"`
	TemplateID      string         `json:"template_id"`
	TemplateVersion int            `json:"template_version"`
	DepartmentID    string         `json:"department_id"`
	SubmittedBy     string         `json:"submitted_by"`
	Status          string         `json:"status"`
	Values          map[string]any `json:"values"`
	Definition      map[string]any `json:"definition"`
	CreatedAt       time.Time      `json:"created_at"`
	UpdatedAt       time.Time      `json:"updated_at"`
}

type FormComment struct {
	ID           RecordID  `json:"id,omitempty"`
	SubmissionID string    `json:"submission_id"`
	AuthorID     string    `json:"author_id"`
	Body         string    `json:"body"`
	CreatedAt    time.Time `json:"created_at"`
}
type FormSubmissionDocument struct {
	ID           RecordID  `json:"id,omitempty"`
	SubmissionID string    `json:"submission_id"`
	DocumentID   string    `json:"document_id"`
	CreatedAt    time.Time `json:"created_at"`
}

type FormApprovalStep struct {
	ID             RecordID  `json:"id,omitempty"`
	SubmissionID   string    `json:"submission_id"`
	Sequence       int       `json:"sequence"`
	Name           string    `json:"name"`
	CompletionRule string    `json:"completion_rule"`
	Status         string    `json:"status"`
	CreatedAt      time.Time `json:"created_at"`
	CompletedAt    time.Time `json:"completed_at,omitempty"`
}

type FormApprovalAssignee struct {
	ID           RecordID  `json:"id,omitempty"`
	StepID       string    `json:"step_id"`
	UserID       string    `json:"user_id"`
	DepartmentID string    `json:"department_id"`
	Status       string    `json:"status"`
	DecidedAt    time.Time `json:"decided_at,omitempty"`
}
