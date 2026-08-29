package dto

import (
	"maps"
	"time"

	"github.com/xmtrxi/edms/internal/database"
	markdownutil "github.com/xmtrxi/edms/internal/markdown"
)

type OCRResponse struct {
	DocumentID string         `json:"document_id"`
	Text       string         `json:"text"`
	Markdown   string         `json:"markdown"`
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
	metadata := maps.Clone(e.Metadata)
	markdown := markdownutil.WithoutEmbeddedDataImages(metadataString(metadata, "markdown_preview"))
	if metadata != nil {
		metadata["markdown_preview"] = markdown
	}
	return OCRResponse{
		DocumentID: e.DocumentID,
		Text:       markdownutil.WithoutEmbeddedDataImages(e.Text),
		Markdown:   markdown,
		Language:   e.Language,
		Pages:      e.Pages,
		Metadata:   metadata,
		CreatedAt:  e.CreatedAt,
		UpdatedAt:  e.UpdatedAt,
	}
}
func metadataString(metadata map[string]any, key string) string {
	if value, ok := metadata[key].(string); ok {
		return value
	}
	return ""
}
