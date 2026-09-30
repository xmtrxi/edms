package database

import "time"

// RetentionPolicy archives, trashes or permanently deletes documents once
// they reach an age. Documents on legal hold are always kept.
type RetentionPolicy struct {
	ID          RecordID  `json:"id,omitempty"`
	OrgID       string    `json:"org_id"`
	Name        string    `json:"name"`
	Enabled     bool      `json:"enabled"`
	FolderID    string    `json:"folder_id,omitempty"`
	Sensitivity string    `json:"sensitivity,omitempty"`
	AgeBasis    string    `json:"age_basis"`
	KeepDays    int       `json:"keep_days"`
	Action      string    `json:"action"`
	CreatedBy   string    `json:"created_by"`
	LastRunAt   time.Time `json:"last_run_at,omitzero"`
	LastCount   int       `json:"last_count"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// Retention actions and age bases.
const (
	RetentionArchive = "archive"
	RetentionTrash   = "trash"
	// RetentionPurge permanently deletes documents that sat in the trash for
	// the policy's age.
	RetentionPurge = "purge"

	AgeFromCreated  = "created"
	AgeFromModified = "modified"
)
