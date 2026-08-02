package dto

import (
	"time"

	"github.com/xmtrxi/edms/internal/database"
)

type DocumentResponse struct {
	ID           string         `json:"id"`
	Name         string         `json:"name"`
	OriginalName string         `json:"original_name"`
	MimeType     string         `json:"mime_type"`
	Size         int64          `json:"size"`
	Status       string         `json:"status"`
	Metadata     map[string]any `json:"metadata"`
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
	Owner        *UserResponse  `json:"owner,omitempty"`
	SharedWith   []UserResponse `json:"shared_with,omitempty"`
	OCR          *OCRResponse   `json:"ocr,omitempty"`
}

func NewDocumentResponse(d *database.Document) DocumentResponse {
	if d == nil {
		return DocumentResponse{}
	}
	return DocumentResponse{
		ID:           d.ID,
		Name:         d.Name,
		OriginalName: d.OriginalName,
		MimeType:     d.MimeType,
		Size:         d.Size,
		Status:       d.Status,
		Metadata:     d.Metadata,
		CreatedAt:    d.CreatedAt,
		UpdatedAt:    d.UpdatedAt,
	}
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
