package dto

import (
	"time"

	"github.com/xmtrxi/edms/internal/database"
)

type DocumentVersionResponse struct {
	ID              string         `json:"id"`
	DocumentID      string         `json:"document_id"`
	ParentVersionID string         `json:"parent_version_id,omitempty"`
	Version         int            `json:"version"`
	FileName        string         `json:"file_name"`
	MimeType        string         `json:"mime_type"`
	Size            int64          `json:"size"`
	Checksum        string         `json:"checksum"`
	CreatedBy       string         `json:"created_by"`
	Creator         *UserResponse  `json:"creator,omitempty"`
	Source          string         `json:"source"`
	ChangeSummary   string         `json:"change_summary"`
	ChangeManifest  map[string]any `json:"change_manifest,omitempty"`
	CreatedAt       time.Time      `json:"created_at"`
}

func NewDocumentVersionResponse(version *database.DocumentVersion) DocumentVersionResponse {
	if version == nil {
		return DocumentVersionResponse{}
	}
	return DocumentVersionResponse{
		ID: string(version.ID), DocumentID: version.DocumentID, ParentVersionID: version.ParentVersionID,
		Version: version.Version, FileName: version.FileName, MimeType: version.MimeType, Size: version.Size,
		Checksum: version.Checksum, CreatedBy: version.CreatedBy, Source: version.Source,
		ChangeSummary: version.ChangeSummary, ChangeManifest: version.ChangeManifest, CreatedAt: version.CreatedAt,
	}
}

type DocumentContributorResponse struct {
	User            UserResponse `json:"user"`
	VersionsCreated int          `json:"versions_created"`
	FirstEditedAt   time.Time    `json:"first_edited_at"`
	LastEditedAt    time.Time    `json:"last_edited_at"`
}

type DocumentContributorsResponse struct {
	UniqueEditorCount int                           `json:"unique_editor_count"`
	EditCount         int                           `json:"edit_count"`
	Contributors      []DocumentContributorResponse `json:"contributors"`
}

type DocumentHistoryEvent struct {
	ID                string         `json:"id"`
	Type              string         `json:"type"`
	ActorID           string         `json:"actor_id"`
	Action            string         `json:"action"`
	Resource          string         `json:"resource"`
	ResourceID        string         `json:"resource_id"`
	DocumentVersionID string         `json:"document_version_id,omitempty"`
	Summary           string         `json:"summary,omitempty"`
	Metadata          map[string]any `json:"metadata,omitempty"`
	CreatedAt         time.Time      `json:"created_at"`
}
