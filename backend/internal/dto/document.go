package dto

import (
	"time"

	"github.com/xmtrxi/edms/internal/database"
)

type DocumentResponse struct {
	ID                string            `json:"id"`
	Name              string            `json:"name"`
	OriginalName      string            `json:"original_name"`
	FolderID          string            `json:"folder_id,omitempty"`
	FolderPath        string            `json:"folder_path,omitempty"`
	FolderColor       string            `json:"folder_color,omitempty"`
	FolderCategoryIDs []string          `json:"folder_category_ids,omitempty"`
	MimeType          string            `json:"mime_type"`
	Size              int64             `json:"size"`
	Status            string            `json:"status"`
	CurrentVersionID  string            `json:"current_version_id,omitempty"`
	VersionCount      int               `json:"version_count"`
	Metadata          map[string]any    `json:"metadata"`
	CreatedAt         time.Time         `json:"created_at"`
	UpdatedAt         time.Time         `json:"updated_at"`
	Owner             *UserResponse     `json:"owner,omitempty"`
	SharedWith        []UserResponse    `json:"shared_with,omitempty"`
	OCR               *OCRResponse      `json:"ocr,omitempty"`
	Category          *CategoryResponse `json:"category,omitempty"`
	Sensitivity       string            `json:"sensitivity"`
	// InheritPermissions is false when the document has its own access list.
	InheritPermissions bool `json:"inherit_permissions"`
	// Permissions lists what the signed-in user may do with the document.
	Permissions []string `json:"permissions,omitempty"`
}

func NewDocumentResponse(d *database.Document) DocumentResponse {
	if d == nil {
		return DocumentResponse{}
	}
	return DocumentResponse{
		ID:                 string(d.ID),
		Name:               d.Name,
		OriginalName:       d.OriginalName,
		FolderID:           d.FolderID,
		FolderPath:         documentMetadataString(d.Metadata, "folder_path"),
		FolderColor:        documentMetadataString(d.Metadata, "folder_color"),
		FolderCategoryIDs:  documentMetadataStringSlice(d.Metadata, "folder_category_ids"),
		MimeType:           d.MimeType,
		Size:               d.Size,
		Status:             d.Status,
		CurrentVersionID:   d.CurrentVersionID,
		VersionCount:       d.VersionCount,
		Metadata:           d.Metadata,
		CreatedAt:          d.CreatedAt,
		UpdatedAt:          d.UpdatedAt,
		Sensitivity:        documentSensitivity(d.Sensitivity),
		InheritPermissions: d.InheritPermissions,
	}
}

func documentSensitivity(value string) string {
	if value == "" {
		return database.SensitivityInternal
	}
	return value
}

func (r DocumentResponse) WithPermissions(permissions []string) DocumentResponse {
	r.Permissions = permissions
	return r
}

type FolderResponse struct {
	Path        string             `json:"path"`
	Name        string             `json:"name"`
	ParentPath  string             `json:"parent_path,omitempty"`
	Color       string             `json:"color,omitempty"`
	CategoryIDs []string           `json:"category_ids,omitempty"`
	Categories  []CategoryResponse `json:"categories,omitempty"`
	Count       int                `json:"count"`
	Size        int64              `json:"size"`
	UpdatedAt   time.Time          `json:"updated_at"`
}

func documentMetadataString(metadata map[string]any, key string) string {
	if value, ok := metadata[key].(string); ok {
		return value
	}
	return ""
}

func documentMetadataStringSlice(metadata map[string]any, key string) []string {
	raw, ok := metadata[key]
	if !ok {
		return nil
	}
	switch values := raw.(type) {
	case []string:
		return values
	case []any:
		result := make([]string, 0, len(values))
		for _, value := range values {
			if s, ok := value.(string); ok && s != "" {
				result = append(result, s)
			}
		}
		return result
	}
	return nil
}

func (r DocumentResponse) WithOwner(owner *database.User) DocumentResponse {
	if owner != nil {
		u := NewUserResponse(owner)
		r.Owner = &u
	}
	return r
}

func (r DocumentResponse) WithSharedWith(users []*database.User) DocumentResponse {
	if len(users) == 0 {
		return r
	}
	r.SharedWith = make([]UserResponse, 0, len(users))
	for _, u := range users {
		r.SharedWith = append(r.SharedWith, NewUserResponse(u))
	}
	return r
}

func (r DocumentResponse) WithOCR(e *database.DocumentExtraction) DocumentResponse {
	if e == nil {
		return r
	}
	ocr := NewOCRResponse(e)
	r.OCR = &ocr
	return r
}

func (r DocumentResponse) WithCategory(category *CategoryResponse) DocumentResponse {
	r.Category = category
	return r
}
