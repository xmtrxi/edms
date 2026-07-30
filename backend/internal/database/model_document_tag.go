package database

import "time"

type DocumentTag struct {
	ID         string    `json:"id,omitempty"`
	DocumentID string    `json:"document_id"`
	TagID      string    `json:"tag_id"`
	CreatedBy  string    `json:"created_by"`
	CreatedAt  time.Time `json:"created_at"`
}
