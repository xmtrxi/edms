package database

import "time"

type DocumentFavorite struct {
	ID         RecordID  `json:"id,omitempty"`
	DocumentID string    `json:"document_id"`
	UserID     string    `json:"user_id"`
	CreatedAt  time.Time `json:"created_at"`
}

type UserSetting struct {
	ID        RecordID       `json:"id,omitempty"`
	UserID    string         `json:"user_id"`
	Values    map[string]any `json:"values"`
	UpdatedAt time.Time      `json:"updated_at"`
}

type LegalHold struct {
	ID          RecordID  `json:"id,omitempty"`
	Name        string    `json:"name"`
	Reason      string    `json:"reason"`
	DocumentIDs []string  `json:"document_ids"`
	CreatedBy   string    `json:"created_by"`
	Active      bool      `json:"active"`
	CreatedAt   time.Time `json:"created_at"`
	ReleasedAt  time.Time `json:"released_at,omitempty"`
}

type PrivacyRequest struct {
	ID          RecordID  `json:"id,omitempty"`
	UserID      string    `json:"user_id"`
	Kind        string    `json:"kind"`
	Status      string    `json:"status"`
	RequestedAt time.Time `json:"requested_at"`
}
