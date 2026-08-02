package dto

import (
	"time"

	"github.com/xmtrxi/edms/internal/database"
)

type OCRResponse struct {
	DocumentID string         `json:"document_id"`
	Text       string         `json:"text"`
	Language   string         `json:"language"`
	Pages      int            `json:"pages"`
	Metadata   map[string]any `json:"metadata"`
	CreatedAt  time.Time      `json:"created_at"`
	UpdatedAt  time.Time      `json:"updated_at"`
}

func NewOCRResponse(e *database.DocumentExtraction) OCRResponse {
	if e == nil {
		return OCRResponse{}
	}
	return OCRResponse{
		DocumentID: e.DocumentID,
		Text:       e.Text,
		Language:   e.Language,
		Pages:      e.Pages,
		Metadata:   e.Metadata,
		CreatedAt:  e.CreatedAt,
		UpdatedAt:  e.UpdatedAt,
	}
}
