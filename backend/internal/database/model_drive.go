package database

import "time"

// UploadSession is a resumable upload in progress. Bytes are staged on disk
// until the client completes the session.
type UploadSession struct {
	ID            RecordID  `json:"id,omitempty"`
	OrgID         string    `json:"org_id"`
	UserID        string    `json:"user_id"`
	Name          string    `json:"name"`
	MimeType      string    `json:"mime_type"`
	Size          int64     `json:"size"`
	Received      int64     `json:"received"`
	FolderID      string    `json:"folder_id,omitempty"`
	CategoryID    string    `json:"category_id,omitempty"`
	Access        string    `json:"access,omitempty"`
	FileRequestID string    `json:"file_request_id,omitempty"`
	Status        string    `json:"status"`
	ExpiresAt     time.Time `json:"expires_at"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

const (
	UploadStatusOpen      = "open"
	UploadStatusCompleted = "completed"
)

// FileRequest is a link that lets anyone drop files into one folder.
type FileRequest struct {
	ID            RecordID  `json:"id,omitempty"`
	OrgID         string    `json:"org_id"`
	FolderID      string    `json:"folder_id"`
	Title         string    `json:"title"`
	Instructions  string    `json:"instructions,omitempty"`
	TokenHash     string    `json:"token_hash,omitempty"`
	CreatedBy     string    `json:"created_by"`
	MaxFiles      int       `json:"max_files"`
	ReceivedFiles int       `json:"received_files"`
	ExpiresAt     time.Time `json:"expires_at"`
	ClosedAt      time.Time `json:"closed_at,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// APIToken is a personal access token. It acts as its user inside one
// organization, limited to its scopes.
type APIToken struct {
	ID         RecordID  `json:"id,omitempty"`
	OrgID      string    `json:"org_id"`
	UserID     string    `json:"user_id"`
	Name       string    `json:"name"`
	Prefix     string    `json:"prefix"`
	TokenHash  string    `json:"token_hash,omitempty"`
	Scopes     []string  `json:"scopes"`
	LastUsedAt time.Time `json:"last_used_at,omitempty"`
	ExpiresAt  time.Time `json:"expires_at,omitempty"`
	RevokedAt  time.Time `json:"revoked_at,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}
