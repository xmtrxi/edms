package search

import (
	"encoding/json"
	"regexp"
	"testing"
)

func TestChunkIDIsMeilisearchSafeAndDeterministic(t *testing.T) {
	chunk := DocumentChunk{DocumentID: "documents:01JAZ8Y7XQ2H8G4K6M9N0P3RST", VersionID: "document_versions:v1", GenerationID: "gen1", ChunkIndex: 3}

	first := chunkID(chunk)
	second := chunkID(chunk)

	if first != second {
		t.Fatalf("documentID must be deterministic: %q != %q", first, second)
	}
	if !regexp.MustCompile(`^[A-Za-z0-9_-]+$`).MatchString(first) {
		t.Fatalf("documentID returned an invalid Meilisearch id: %q", first)
	}
	chunk.ChunkIndex++
	if first == chunkID(chunk) {
		t.Fatal("different chunks produced the same primary key")
	}
}

func TestChunkKeepsSurrealDocumentIDSeparateFromPrimaryKey(t *testing.T) {
	doc := DocumentChunk{
		ID:         "safe_id",
		DocumentID: "documents:abc",
	}

	payload, err := json.Marshal(doc)
	if err != nil {
		t.Fatalf("marshal document: %v", err)
	}

	var fields map[string]any
	if err := json.Unmarshal(payload, &fields); err != nil {
		t.Fatalf("unmarshal document: %v", err)
	}
	if fields["id"] != "safe_id" {
		t.Fatalf("id = %v, want safe_id", fields["id"])
	}
	if fields["document_id"] != "documents:abc" {
		t.Fatalf("document_id = %v, want documents:abc", fields["document_id"])
	}
}
