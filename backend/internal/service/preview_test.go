package service

import "testing"

func TestPreviewKind(t *testing.T) {
	cases := []struct {
		mime, name, kind string
		convert          bool
	}{
		{"application/pdf", "a.pdf", PreviewPDF, false},
		{"image/png", "a.png", PreviewImage, false},
		{"application/vnd.openxmlformats-officedocument.wordprocessingml.document", "a.docx", PreviewPDF, true},
		{"application/octet-stream", "sheet.xlsx", PreviewPDF, true},
		{"text/csv", "a.csv", PreviewText, false},
		{"video/mp4", "a.mp4", PreviewVideo, false},
		{"application/zip", "a.zip", "", false},
	}
	for _, c := range cases {
		kind, convert := PreviewKind(c.mime, c.name)
		if kind != c.kind || convert != c.convert {
			t.Errorf("%s: got %q/%v, want %q/%v", c.name, kind, convert, c.kind, c.convert)
		}
	}
}
