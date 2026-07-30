package database

import "time"

type Comment struct {
	ID         string    `json:"id,omitempty"`
	DocumentID string    `json:"document_id"`
	AuthorID   string    `json:"author_id"`
	Body       string    `json:"body"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}
