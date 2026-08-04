package database

import (
	"testing"

	"github.com/fxamacker/cbor/v2"
	"github.com/surrealdb/surrealdb.go/pkg/models"
)

func TestRecordID_UnmarshalCBOR(t *testing.T) {
	t.Run("decodes record id tag", func(t *testing.T) {
		data, err := cbor.Marshal(cbor.Tag{
			Number:  models.TagRecordID,
			Content: []any{"users", "50cqyhrtc8piv0b6lf51"},
		})
		if err != nil {
			t.Fatal(err)
		}

		var id RecordID
		if err := cbor.Unmarshal(data, &id); err != nil {
			t.Fatal(err)
		}
		if id != "users:50cqyhrtc8piv0b6lf51" {
			t.Fatalf("expected users:50cqyhrtc8piv0b6lf51, got %q", id)
		}
	})

	t.Run("decodes numeric record id", func(t *testing.T) {
		data, err := cbor.Marshal(cbor.Tag{
			Number:  models.TagRecordID,
			Content: []any{"documents", uint64(42)},
		})
		if err != nil {
			t.Fatal(err)
		}

		var id RecordID
		if err := cbor.Unmarshal(data, &id); err != nil {
			t.Fatal(err)
		}
		if id != "documents:42" {
			t.Fatalf("expected documents:42, got %q", id)
		}
	})

	t.Run("passes through plain string", func(t *testing.T) {
		var id RecordID
		if err := cbor.Unmarshal([]byte{0x64, 'a', 'b', 'c', 'd'}, &id); err != nil {
			t.Fatal(err)
		}
		if id != "abcd" {
			t.Fatalf("expected abcd, got %q", id)
		}
	})
}

func TestParseRecordID(t *testing.T) {
	rid, err := ParseRecordID("users:50cqyhrtc8piv0b6lf51")
	if err != nil {
		t.Fatal(err)
	}
	if rid.Table != "users" || rid.ID != "50cqyhrtc8piv0b6lf51" {
		t.Fatalf("unexpected record id: %+v", rid)
	}
	if rid.String() != "users:50cqyhrtc8piv0b6lf51" {
		t.Fatalf("round trip failed: %q", rid.String())
	}

	if _, err := ParseRecordID("not-a-record-id"); err == nil {
		t.Fatal("expected error for malformed id")
	}
}

func TestUserUnmarshalFromRecord(t *testing.T) {
	raw, err := cbor.Marshal(map[string]any{
		"id":    models.NewRecordID("users", "50cqyhrtc8piv0b6lf51"),
		"email": "test@example.com",
	})
	if err != nil {
		t.Fatal(err)
	}

	var out User
	if err := cbor.Unmarshal(raw, &out); err != nil {
		t.Fatal(err)
	}
	if out.ID != "users:50cqyhrtc8piv0b6lf51" {
		t.Fatalf("expected populated id, got %q", out.ID)
	}
	if out.Email != "test@example.com" {
		t.Fatalf("expected email, got %q", out.Email)
	}
}
