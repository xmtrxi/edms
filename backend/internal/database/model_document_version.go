package database

import "time"

type DocumentVersion struct {
	ID               RecordID       `json:"id,omitempty"`
	DocumentID       string         `json:"document_id"`
	ParentVersionID  string         `json:"parent_version_id,omitempty"`
	Version          int            `json:"version"`
	StorageProvider  string         `json:"storage_provider"`
	StoragePath      string         `json:"storage_path"`
	FileName         string         `json:"file_name"`
	MimeType         string         `json:"mime_type"`
	Checksum         string         `json:"checksum"`
	Size             int64          `json:"size"`
	CreatedBy        string         `json:"created_by"`
	Source           string         `json:"source"`
	ChangeSummary    string         `json:"change_summary"`
	ChangeManifest   map[string]any `json:"change_manifest,omitempty"`
	WorkflowID       string         `json:"workflow_id,omitempty"`
	WorkflowStepID   string         `json:"workflow_step_id,omitempty"`
	SigningRequestID string         `json:"signing_request_id,omitempty"`
	CreatedAt        time.Time      `json:"created_at"`
}

type DocumentVersionContributor struct {
	ID        RecordID  `json:"id,omitempty"`
	VersionID string    `json:"version_id"`
	UserID    string    `json:"user_id"`
	Role      string    `json:"role"`
	CreatedAt time.Time `json:"created_at"`
}

const (
	VersionSourceUpload           = "upload"
	VersionSourceStorageSync      = "storage_sync"
	VersionSourceOnlyOfficeEdit   = "onlyoffice_edit"
	VersionSourceStirlingPDF      = "stirling_pdf"
	VersionSourceWorkflowRevision = "workflow_revision"
	VersionSourceConversion       = "conversion"
	VersionSourceSignature        = "signature"
	VersionSourceRestore          = "restore"
)
