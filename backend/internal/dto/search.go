package dto

import "github.com/xmtrxi/edms/internal/database"

type SearchResultResponse struct {
	DocumentResponse
	Snippet string `json:"snippet,omitempty"`
}

func NewSearchResultResponse(doc *database.Document, snippet string) SearchResultResponse {
	return SearchResultResponse{
		DocumentResponse: NewDocumentResponse(doc),
		Snippet:          snippet,
	}
}
