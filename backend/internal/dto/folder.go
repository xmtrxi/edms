package dto

import (
	"time"

	"github.com/xmtrxi/edms/internal/database"
)

type ManagedFolderResponse struct {
	ID                 string   `json:"id"`
	ParentID           string   `json:"parent_id,omitempty"`
	AncestorIDs        []string `json:"ancestor_ids,omitempty"`
	Depth              int      `json:"depth"`
	Name               string   `json:"name"`
	Description        string   `json:"description,omitempty"`
	Color              string   `json:"color,omitempty"`
	InheritPermissions bool     `json:"inherit_permissions"`
	DocumentCount      int      `json:"document_count"`
	// CanManage and CanUpload describe what the signed-in user may do here.
	CanManage bool      `json:"can_manage"`
	CanUpload bool      `json:"can_upload"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func NewManagedFolderResponse(folder *database.Folder, documentCount int) ManagedFolderResponse {
	if folder == nil {
		return ManagedFolderResponse{}
	}
	return ManagedFolderResponse{
		ID:                 string(folder.ID),
		ParentID:           folder.ParentID,
		AncestorIDs:        folder.AncestorIDs,
		Depth:              folder.Depth,
		Name:               folder.Name,
		Description:        folder.Description,
		Color:              folder.Color,
		InheritPermissions: folder.InheritPermissions,
		DocumentCount:      documentCount,
		CreatedAt:          folder.CreatedAt,
		UpdatedAt:          folder.UpdatedAt,
	}
}
