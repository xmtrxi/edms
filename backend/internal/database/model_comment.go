package database

import "time"

// Comment is a note on a document. Replies point at their thread's first
// comment through ParentID.
type Comment struct {
	ID         RecordID  `json:"id,omitempty"`
	OrgID      string    `json:"org_id,omitempty"`
	DocumentID string    `json:"document_id"`
	ParentID   string    `json:"parent_id,omitempty"`
	AuthorID   string    `json:"author_id"`
	Body       string    `json:"body"`
	Page       int       `json:"page,omitempty"`
	Quote      string    `json:"quote,omitempty"`
	Mentions   []string  `json:"mentions,omitempty"`
	EditedAt   time.Time `json:"edited_at,omitempty"`
	ResolvedAt time.Time `json:"resolved_at,omitempty"`
	ResolvedBy string    `json:"resolved_by,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}
