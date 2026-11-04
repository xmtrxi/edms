package database

import "time"

type AIJob struct {
	ID          RecordID       `json:"id,omitempty"`
	UserID      string         `json:"user_id"`
	Kind        string         `json:"kind"`
	Status      string         `json:"status"`
	DocumentIDs []string       `json:"document_ids"`
	Input       map[string]any `json:"input"`
	Result      map[string]any `json:"result,omitempty"`
	Error       string         `json:"error,omitempty"`
	CreatedAt   time.Time      `json:"created_at"`
	CompletedAt time.Time      `json:"completed_at,omitempty"`
}
