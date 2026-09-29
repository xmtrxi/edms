package database

import "time"

type Document struct {
	ID               RecordID       `json:"id,omitempty"`
	OrgID            string         `json:"org_id"`
	OwnerID          string         `json:"owner_id"`
	FolderID         string         `json:"folder_id,omitempty"`
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
	// InheritPermissions makes the document follow its folder's access. When
	// false only the document's own grants, shares and its creator apply.
	InheritPermissions bool      `json:"inherit_permissions"`
	Sensitivity        string    `json:"sensitivity,omitempty"`
	CreatedAt          time.Time `json:"created_at"`
	UpdatedAt          time.Time `json:"updated_at"`
	DeletedAt          time.Time `json:"deleted_at,omitempty"`
	DeletedFromStatus  string    `json:"deleted_from_status,omitempty"`
}

const (
	DocumentStatusUploaded   = "uploaded"
	DocumentStatusProcessing = "processing"
	DocumentStatusReady      = "ready"
	DocumentStatusFailed     = "failed"
	DocumentStatusArchived   = "archived"
	DocumentStatusTrashed    = "trashed"
)

// Sensitivity labels. Restricted documents never inherit folder access and
// may only be granted to named people and groups; admins cannot bypass them.
const (
	SensitivityPublic       = "public"
	SensitivityInternal     = "internal"
	SensitivityConfidential = "confidential"
	SensitivityRestricted   = "restricted"
)

func IsSensitivity(value string) bool {
	switch value {
	case SensitivityPublic, SensitivityInternal, SensitivityConfidential, SensitivityRestricted:
		return true
	}
	return false
}
