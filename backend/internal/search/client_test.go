package search

import (
	"encoding/json"
	"regexp"
	"testing"
)

func TestDocumentIDIsMeilisearchSafeAndDeterministic(t *testing.T) {
	const recordID = "documents:01JAZ8Y7XQ2H8G4K6M9N0P3RST"

	first := documentID(recordID)
	second := documentID(recordID)

	if first != second {
		t.Fatalf("documentID must be deterministic: %q != %q", first, second)
	}
	if !regexp.MustCompile(`^[A-Za-z0-9_-]+$`).MatchString(first) {
		t.Fatalf("documentID returned an invalid Meilisearch id: %q", first)
	}
	if first == documentID("documents:different") {
		t.Fatal("different record ids produced the same document ID")
	}
}

func TestDocumentKeepsSurrealRecordIDSeparateFromPrimaryKey(t *testing.T) {
	doc := Document{
		ID:       "safe_id",
		RecordID: "documents:abc",
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
	if fields["record_id"] != "documents:abc" {
		t.Fatalf("record_id = %v, want documents:abc", fields["record_id"])
	}
}
