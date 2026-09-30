package editor

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"io"
	"os"
	"testing"
)

func TestBlankFilesAreWellFormedAndUnique(t *testing.T) {
	for _, kind := range []string{KindDocument, KindSpreadsheet, KindPresentation} {
		first, err := Blank(kind, "Q3 plan & budget", "Alice <a@b>")
		if err != nil {
			t.Fatal(err)
		}
		second, _ := Blank(kind, "Q3 plan & budget", "Alice <a@b>")
		if bytes.Equal(first.Body, second.Body) {
			t.Fatalf("%s: two blank files are identical", kind)
		}
		archive, err := zip.NewReader(bytes.NewReader(first.Body), int64(len(first.Body)))
		if err != nil {
			t.Fatalf("%s: %v", kind, err)
		}
		if archive.File[0].Name != "[Content_Types].xml" {
			t.Fatalf("%s: content types must come first", kind)
		}
		for _, part := range archive.File {
			r, _ := part.Open()
			body, _ := io.ReadAll(r)
			decoder := xml.NewDecoder(bytes.NewReader(body))
			for {
				if _, err := decoder.Token(); err == io.EOF {
					break
				} else if err != nil {
					t.Fatalf("%s %s: %v", kind, part.Name, err)
				}
			}
		}
		if dir := os.Getenv("BLANK_OUT"); dir != "" {
			_ = os.WriteFile(dir+"/blank"+first.Extension, first.Body, 0o644)
		}
	}
	if _, err := Blank("drawing", "x", "y"); err != ErrUnknownKind {
		t.Fatalf("unknown kind: %v", err)
	}
}
