package database

import "time"

// Folder is a logical container. Storage paths intentionally remain independent
// from this hierarchy, so moving a folder never moves an object in storage.
type Folder struct {
	ID                 RecordID  `json:"id,omitempty"`
	OwnerID            string    `json:"owner_id"`
	ParentID           string    `json:"parent_id,omitempty"`
	AncestorIDs        []string  `json:"ancestor_ids,omitempty"`
	Depth              int       `json:"depth"`
	Name               string    `json:"name"`
	NormalizedName     string    `json:"normalized_name"`
	Description        string    `json:"description,omitempty"`
	Color              string    `json:"color,omitempty"`
	InheritPermissions bool      `json:"inherit_permissions"`
	PermissionVersion  int64     `json:"permission_version"`
	StorageProvider    string    `json:"storage_provider,omitempty"`
	ExternalID         *string   `json:"external_id,omitempty"`
	ExternalPath       string    `json:"external_path,omitempty"`
	CreatedBy          string    `json:"created_by"`
	CreatedAt          time.Time `json:"created_at"`
	UpdatedAt          time.Time `json:"updated_at"`
}

type FolderGrant struct {
	ID            RecordID  `json:"id,omitempty"`
	FolderID      string    `json:"folder_id"`
	PrincipalType string    `json:"principal_type"`
	PrincipalID   string    `json:"principal_id"`
	Role          string    `json:"role"`
	Allow         []string  `json:"allow,omitempty"`
	Deny          []string  `json:"deny,omitempty"`
	CreatedBy     string    `json:"created_by"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

type FolderCategory struct {
	ID         RecordID  `json:"id,omitempty"`
	FolderID   string    `json:"folder_id"`
	CategoryID string    `json:"category_id"`
	CreatedAt  time.Time `json:"created_at"`
}
