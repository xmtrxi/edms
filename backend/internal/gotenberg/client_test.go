package gotenberg

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestConvertOfficeToPDFSendsFile(t *testing.T) {
	var gotMethod, gotPath, gotFileName, gotFile string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod, gotPath = r.Method, r.URL.Path
		if err := r.ParseMultipartForm(1 << 20); err != nil {
			t.Errorf("parse multipart: %v", err)
			return
		}
		files := r.MultipartForm.File["files"]
		if len(files) != 1 {
			t.Errorf("expected one file part named \"files\", got %d", len(files))
			return
		}
		gotFileName = files[0].Filename
		file, _ := files[0].Open()
		data, _ := io.ReadAll(file)
		_ = file.Close()
		gotFile = string(data)
		w.Header().Set("Content-Type", "application/pdf")
		_, _ = w.Write([]byte("%PDF-1.7 converted"))
	}))
	defer server.Close()

	pdf, err := New(server.URL+"/", time.Second).ConvertOfficeToPDF(context.Background(), "minutes.docx", strings.NewReader("docx bytes"))
	if err != nil {
		t.Fatalf("ConvertOfficeToPDF: %v", err)
	}
	if string(pdf) != "%PDF-1.7 converted" {
		t.Fatalf("unexpected pdf %q", pdf)
	}
	if gotMethod != http.MethodPost || gotPath != "/forms/libreoffice/convert" {
		t.Fatalf("unexpected request %s %s", gotMethod, gotPath)
	}
	if gotFileName != "minutes.docx" || gotFile != "docx bytes" {
		t.Fatalf("unexpected upload %q = %q", gotFileName, gotFile)
	}
}

func TestConvertOfficeToPDFKeepsBasePath(t *testing.T) {
	var gotPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		_, _ = w.Write([]byte("pdf"))
	}))
	defer server.Close()
	if _, err := New(server.URL+"/gotenberg/", 0).ConvertOfficeToPDF(context.Background(), "a.xlsx", strings.NewReader("x")); err != nil {
		t.Fatalf("ConvertOfficeToPDF: %v", err)
	}
	if gotPath != "/gotenberg/forms/libreoffice/convert" {
		t.Fatalf("expected base path to be preserved, got %q", gotPath)
	}
}

func TestEndpoint(t *testing.T) {
	tests := []struct{ base, want string }{
		{"http://gotenberg:3000", "http://gotenberg:3000/forms/libreoffice/convert"},
		{"http://gotenberg:3000/", "http://gotenberg:3000/forms/libreoffice/convert"},
		{"https://proxy.example/convert/api", "https://proxy.example/convert/api/forms/libreoffice/convert"},
	}
	for _, tc := range tests {
		if got := New(tc.base, 0).endpoint("/forms/libreoffice/convert"); got != tc.want {
			t.Errorf("endpoint(%q) = %q, want %q", tc.base, got, tc.want)
		}
	}
}

func TestNewDefaultsTimeout(t *testing.T) {
	if got := New("http://x", 0).http.Timeout; got != 10*time.Minute {
		t.Fatalf("default timeout = %v", got)
	}
	if got := New("http://x", -time.Second).http.Timeout; got != 10*time.Minute {
		t.Fatalf("negative timeout should use default, got %v", got)
	}
	if got := New("http://x", 5*time.Second).http.Timeout; got != 5*time.Second {
		t.Fatalf("timeout = %v", got)
	}
}

func TestConvertOfficeToPDFErrors(t *testing.T) {
	t.Run("not configured", func(t *testing.T) {
		var nilClient *Client
		for _, client := range []*Client{nilClient, New("", 0)} {
			if _, err := client.ConvertOfficeToPDF(context.Background(), "a.docx", strings.NewReader("x")); err == nil || !strings.Contains(err.Error(), "not configured") {
				t.Fatalf("expected not configured error, got %v", err)
			}
		}
	})

	t.Run("non-2xx returns trimmed body", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte("  unsupported file \n"))
		}))
		defer server.Close()
		pdf, err := New(server.URL, time.Second).ConvertOfficeToPDF(context.Background(), "a.bin", strings.NewReader("x"))
		if pdf != nil || err == nil || err.Error() != "gotenberg convert failed: unsupported file" {
			t.Fatalf("unexpected result %q, %v", pdf, err)
		}
	})

	t.Run("reader failure", func(t *testing.T) {
		readErr := errors.New("read failed")
		_, err := New("http://127.0.0.1:1", time.Second).ConvertOfficeToPDF(context.Background(), "a.docx", failingReader{readErr})
		if !errors.Is(err, readErr) {
			t.Fatalf("expected read error, got %v", err)
		}
	})

	t.Run("timeout", func(t *testing.T) {
		release := make(chan struct{})
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			select {
			case <-release:
			case <-r.Context().Done():
			}
		}))
		defer server.Close()
		defer close(release)
		_, err := New(server.URL, 50*time.Millisecond).ConvertOfficeToPDF(context.Background(), "a.docx", strings.NewReader("x"))
		if err == nil {
			t.Fatal("expected timeout error")
		}
	})

	t.Run("cancelled context", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte("pdf"))
		}))
		defer server.Close()
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		_, err := New(server.URL, time.Second).ConvertOfficeToPDF(ctx, "a.docx", strings.NewReader("x"))
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("expected context.Canceled, got %v", err)
		}
	})
}

type failingReader struct{ err error }

func (r failingReader) Read([]byte) (int, error) { return 0, r.err }
