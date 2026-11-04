package database

import "time"

// M365Session is a working copy of a document opened in Microsoft 365 for
// the web. Edits land in the copy and are checked in as new versions, so the
// files behind earlier versions never change.
type M365Session struct {
	ID            RecordID   `json:"id,omitempty"`
	OrgID         string     `json:"org_id"`
	DocumentID    string     `json:"document_id"`
	UserID        string     `json:"user_id"`
	Path          string     `json:"path"`
	WebURL        string     `json:"web_url"`
	BaseVersionID string     `json:"base_version_id"`
	Checksum      string     `json:"checksum"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
	ClosedAt      *time.Time `json:"closed_at,omitempty"`
}
