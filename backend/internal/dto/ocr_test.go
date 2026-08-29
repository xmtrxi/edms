package dto

import (
	"strings"
	"testing"

	"github.com/xmtrxi/edms/internal/database"
)

func TestNewOCRResponseRemovesEmbeddedImages(t *testing.T) {
	embeddedImage := "![Image](data:image/png;base64,iVBORw0KGgo=)"
	extraction := &database.DocumentExtraction{
		DocumentID: "documents:example",
		Metadata: map[string]any{
			"markdown_preview": "# Title\n\n" + embeddedImage + "\n\nBody text",
		},
	}

	response := NewOCRResponse(extraction)

	if strings.Contains(response.Markdown, "data:image/") {
		t.Fatalf("response markdown contains an embedded image: %q", response.Markdown)
	}
	if response.Markdown != "# Title\n\nBody text" {
		t.Fatalf("unexpected response markdown: %q", response.Markdown)
	}
	metadataMarkdown, ok := response.Metadata["markdown_preview"].(string)
	if !ok || strings.Contains(metadataMarkdown, "data:image/") {
		t.Fatalf("response metadata contains an embedded image: %#v", response.Metadata)
	}
	if strings.Contains(extraction.Metadata["markdown_preview"].(string), "data:image/") == false {
		t.Fatal("source extraction metadata was modified")
	}
}
