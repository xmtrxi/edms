package dto

import "github.com/xmtrxi/edms/internal/database"

type SearchResultResponse struct {
	DocumentResponse
	Snippet     string   `json:"snippet,omitempty"`
	VersionID   string   `json:"version_id,omitempty"`
	ChunkIndex  int      `json:"chunk_index"`
	Headings    []string `json:"headings,omitempty"`
	PageNumbers []int    `json:"page_numbers,omitempty"`
	PageStart   int      `json:"page_start,omitempty"`
	PageEnd     int      `json:"page_end,omitempty"`
}

func NewSearchResultResponse(doc *database.Document, snippet, versionID string, chunkIndex int, headings []string, pageNumbers []int, pageStart, pageEnd int) SearchResultResponse {
	return SearchResultResponse{
		DocumentResponse: NewDocumentResponse(doc),
		Snippet:          snippet, VersionID: versionID, ChunkIndex: chunkIndex,
		Headings: headings, PageNumbers: pageNumbers, PageStart: pageStart, PageEnd: pageEnd,
	}
}
