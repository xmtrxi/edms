package database

import "time"

type Category struct {
	ID          RecordID  `json:"id,omitempty"`
	OwnerID     string    `json:"owner_id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	ParentID    string    `json:"parent_id"`
	Color       string    `json:"color"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type DocumentCategory struct {
	ID         RecordID  `json:"id,omitempty"`
	DocumentID string    `json:"document_id"`
	CategoryID string    `json:"category_id"`
	CreatedAt  time.Time `json:"created_at"`
}
