package database

import "time"

// AutomationRule runs actions on documents when something happens to them.
type AutomationRule struct {
	ID         RecordID            `json:"id,omitempty"`
	OrgID      string              `json:"org_id"`
	Name       string              `json:"name"`
	Enabled    bool                `json:"enabled"`
	Trigger    string              `json:"trigger"`
	FolderID   string              `json:"folder_id,omitempty"`
	Conditions AutomationCondition `json:"conditions"`
	Actions    []AutomationAction  `json:"actions"`
	CreatedBy  string              `json:"created_by"`
	RunCount   int                 `json:"run_count"`
	LastRunAt  time.Time           `json:"last_run_at,omitzero"`
	CreatedAt  time.Time           `json:"created_at"`
	UpdatedAt  time.Time           `json:"updated_at"`
}

// Automation triggers.
const (
	TriggerDocumentCreated    = "document.created"
	TriggerDocumentLabelled   = "document.labelled"
	TriggerVersionCreated     = "version.created"
	TriggerSignatureCompleted = "signature.completed"
)

// AutomationCondition narrows which documents a rule acts on. Empty fields
// match everything.
type AutomationCondition struct {
	NameContains   string   `json:"name_contains,omitempty"`
	Extensions     []string `json:"extensions,omitempty"`
	Sensitivity    string   `json:"sensitivity,omitempty"`
	Classification string   `json:"classification,omitempty"`
	SensitiveData  bool     `json:"sensitive_data,omitempty"`
}

// AutomationAction is one step a rule takes.
type AutomationAction struct {
	Type          string `json:"type"`
	FolderID      string `json:"folder_id,omitempty"`
	Sensitivity   string `json:"sensitivity,omitempty"`
	PrincipalType string `json:"principal_type,omitempty"`
	PrincipalID   string `json:"principal_id,omitempty"`
	// PrincipalName labels the principal in the rule's summary.
	PrincipalName string   `json:"principal_name,omitempty"`
	Role          string   `json:"role,omitempty"`
	Emails        []string `json:"emails,omitempty"`
	Message       string   `json:"message,omitempty"`
}

// Automation actions.
const (
	ActionMoveToFolder   = "move_to_folder"
	ActionSetSensitivity = "set_sensitivity"
	ActionGrantAccess    = "grant_access"
	ActionNotify         = "notify"
)

// AutomationRun records one rule acting on one document.
type AutomationRun struct {
	ID         RecordID  `json:"id,omitempty"`
	OrgID      string    `json:"org_id"`
	RuleID     string    `json:"rule_id"`
	DocumentID string    `json:"document_id"`
	Trigger    string    `json:"trigger"`
	Status     string    `json:"status"`
	Detail     string    `json:"detail,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
}
