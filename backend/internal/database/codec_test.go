package database

import (
	"testing"
	"time"

	"github.com/fxamacker/cbor/v2"
	"github.com/surrealdb/surrealdb.go/surrealcbor"
)

func TestDatetimeMarshalerRewritesTag0(t *testing.T) {
	when := time.Date(2026, 11, 4, 10, 30, 0, 0, time.UTC) // the SDK encodes RFC 3339 without fractions
	in := map[string]any{
		"name":  "x",
		"times": []any{when, &when},
		"n":     int64(-42),
		"f":     1.5,
		"b":     []byte{1, 2, 3},
		"nil":   nil,
	}
	data, err := datetimeMarshaler{inner: surrealcbor.New()}.Marshal(in)
	if err != nil {
		t.Fatal(err)
	}

	dm, _ := cbor.DecOptions{DefaultMapType: nil}.DecMode()
	var got map[string]any
	if err := dm.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}
	times := got["times"].([]any)
	for i, v := range times {
		tag, ok := v.(cbor.Tag)
		if !ok || tag.Number != cborTagDatetimeCompact {
			t.Fatalf("times[%d] = %#v, want tag 12", i, v)
		}
		parts := tag.Content.([]any)
		if parts[0].(uint64) != uint64(when.Unix()) || parts[1].(uint64) != uint64(when.Nanosecond()) {
			t.Fatalf("times[%d] content = %v", i, parts)
		}
	}
	if got["name"] != "x" || got["n"] != int64(-42) || got["f"] != 1.5 || got["nil"] != nil {
		t.Fatalf("other values changed: %#v", got)
	}

	// The driver's own decoder reads tag 12 back as the same instant.
	var back struct {
		Times []time.Time `json:"times"`
	}
	if err := surrealcbor.New().Unmarshal(data, &back); err != nil {
		t.Fatal(err)
	}
	if !back.Times[0].Equal(when) {
		t.Fatalf("round trip = %v, want %v", back.Times[0], when)
	}
}

func TestRewriteDatetimeTagsRejectsTruncated(t *testing.T) {
	data, _ := cbor.Marshal([]any{"abc", 1})
	if _, err := rewriteDatetimeTags(data[:len(data)-2]); err == nil {
		t.Fatal("expected error for truncated input")
	}
}
