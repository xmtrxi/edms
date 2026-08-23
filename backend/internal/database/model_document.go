package database

import "time"

type Document struct {
	ID               RecordID       `json:"id,omitempty"`
	OwnerID          string         `json:"owner_id"`
	Name             string         `json:"name"`
	OriginalName     string         `json:"original_name"`
	MimeType         string         `json:"mime_type"`
	Size             int64          `json:"size"`
	StorageProvider  string         `json:"storage_provider"`
	StoragePath      string         `json:"storage_path"`
	Checksum         string         `json:"checksum"`
	Status           string         `json:"status"`
	CurrentVersionID string         `json:"current_version_id,omitempty"`
	VersionCount     int            `json:"version_count"`
	Metadata         map[string]any `json:"metadata"`
	CreatedAt        time.Time      `json:"created_at"`
	UpdatedAt        time.Time      `json:"updated_at"`
}

const (
	DocumentStatusUploaded   = "uploaded"
	DocumentStatusProcessing = "processing"
	DocumentStatusReady      = "ready"
	DocumentStatusFailed     = "failed"
	DocumentStatusArchived   = "archived"
)
