package database

import "time"

type DocumentExtraction struct {
	ID         RecordID `json:"id,omitempty"`
	DocumentID string   `json:"document_id"`
	// Text is searchable OCR content and must not contain embedded image data.
	Text      string         `json:"text"`
	Language  string         `json:"language"`
	Pages     int            `json:"pages"`
	Metadata  map[string]any `json:"metadata"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
}
