package service

import (
	"strings"
	"testing"
	"time"

	"github.com/xmtrxi/edms/internal/database"
	"github.com/xmtrxi/edms/internal/docling"
)

func TestBuildSearchChunksCleansImagesAndKeepsPages(t *testing.T) {
	doc := &database.Document{ID: database.RecordID("documents:one"), OwnerID: "users:one", FolderID: "folders:shared", Name: "Contract", OriginalName: "contract.pdf", MimeType: "application/pdf"}
	chunks := buildSearchChunks(doc, "document_versions:v2", "generation", []docling.Chunk{{
		Text: "<!-- image -->\n\nTermination clause", Headings: []string{"Terms"}, PageNumbers: []int{4, 5},
	}}, "", time.Time{})
	if len(chunks) != 1 {
		t.Fatalf("chunks = %d", len(chunks))
	}
	if chunks[0].Content != "Termination clause" || chunks[0].PageStart != 4 || chunks[0].PageEnd != 5 {
		t.Fatalf("unexpected chunk: %#v", chunks[0])
	}
	if strings.Contains(chunks[0].Content, "image") {
		t.Fatalf("placeholder leaked into indexed content: %q", chunks[0].Content)
	}
}

func TestFallbackChunksSplitsLongText(t *testing.T) {
	paragraph := strings.Repeat("searchable ", 180)
	chunks := fallbackChunks(paragraph + "\n\n" + paragraph)
	if len(chunks) < 2 {
		t.Fatalf("fallback chunks = %d, want at least 2", len(chunks))
	}
}
