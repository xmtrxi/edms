package database

import "time"

type DocumentVersion struct {
	ID              string    `json:"id,omitempty"`
	DocumentID      string    `json:"document_id"`
	Version         int       `json:"version"`
	StorageProvider string    `json:"storage_provider"`
	StoragePath     string    `json:"storage_path"`
	Checksum        string    `json:"checksum"`
	Size            int64     `json:"size"`
	CreatedBy       string    `json:"created_by"`
	CreatedAt       time.Time `json:"created_at"`
}
