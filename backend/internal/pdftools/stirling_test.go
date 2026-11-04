package pdftools

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRunSendsFilesAndFields(t *testing.T) {
	var gotPath, gotKey string
	var gotFiles []string
	var gotFields map[string]string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath, gotKey = r.URL.Path, r.Header.Get("X-API-KEY")
		if err := r.ParseMultipartForm(1 << 20); err != nil {
			t.Fatal(err)
		}
		gotFields = map[string]string{}
		for k, v := range r.MultipartForm.Value {
			gotFields[k] = v[0]
		}
		for _, fh := range r.MultipartForm.File["fileInput"] {
			gotFiles = append(gotFiles, fh.Filename)
		}
		_, _ = io.WriteString(w, "%PDF-1.7 merged")
	}))
	defer srv.Close()

	out, err := New(srv.URL, "secret", 0).Run(context.Background(), OpMerge,
		[]Input{{Name: "a.pdf", Body: strings.NewReader("%PDF a")}, {Name: "b.pdf", Body: strings.NewReader("%PDF b")}}, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if string(out) != "%PDF-1.7 merged" || gotPath != "/api/v1/general/merge-pdfs" || gotKey != "secret" {
		t.Fatalf("unexpected call: path=%s key=%s out=%q", gotPath, gotKey, out)
	}
	if len(gotFiles) != 2 || gotFiles[0] != "a.pdf" || gotFiles[1] != "b.pdf" || gotFields["sortType"] != "orderProvided" {
		t.Fatalf("files %v fields %v", gotFiles, gotFields)
	}
}

func TestRunRejectsBadOptionsAndResponses(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "rotate") {
			http.Error(w, "boom", http.StatusBadGateway)
			return
		}
		_, _ = io.WriteString(w, "<html>login</html>")
	}))
	defer srv.Close()
	c := New(srv.URL, "", 0)
	one := func() []Input { return []Input{{Name: "a.pdf", Body: strings.NewReader("x")}} }

	if _, err := c.Run(context.Background(), OpMerge, one(), Options{}); err == nil {
		t.Fatal("merging one file should be refused")
	}
	if _, err := c.Run(context.Background(), OpRotate, one(), Options{Angle: 45}); err == nil {
		t.Fatal("45 degrees should be refused")
	}
	_, err := c.Run(context.Background(), OpRotate, one(), Options{Angle: 90})
	var httpErr *HTTPError
	if !errors.As(err, &httpErr) || !httpErr.Retryable() {
		t.Fatalf("a 502 should be a retryable HTTP error, got %v", err)
	}
	if _, err := c.Run(context.Background(), OpCompress, one(), Options{}); err == nil || !strings.Contains(err.Error(), "other than a PDF") {
		t.Fatalf("a non-PDF response should be refused, got %v", err)
	}
}

func TestOptionsDefaults(t *testing.T) {
	o := Options{}
	if err := o.Validate(OpCompress, 1); err != nil || o.Level != 5 {
		t.Fatalf("compress default level: %v %d", err, o.Level)
	}
	o = Options{Text: "  DRAFT "}
	if err := o.Validate(OpWatermark, 1); err != nil || o.Text != "DRAFT" || o.Opacity != 0.3 {
		t.Fatalf("watermark defaults: %v %+v", err, o)
	}
	o = Options{}
	if err := o.Validate(OpPageNumbers, 1); err != nil || o.Position != 8 {
		t.Fatalf("page number default: %v %d", err, o.Position)
	}
}
