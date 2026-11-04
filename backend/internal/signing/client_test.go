package signing

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCreateSendsMultipartRequest(t *testing.T) {
	var (
		gotMethod, gotPath, gotAuth string
		gotFields                   = map[string]string{}
		gotFileName, gotFile        string
	)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod, gotPath, gotAuth = r.Method, r.URL.Path, r.Header.Get("Authorization")
		if err := r.ParseMultipartForm(1 << 20); err != nil {
			t.Errorf("parse multipart: %v", err)
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		for key, values := range r.MultipartForm.Value {
			gotFields[key] = values[0]
		}
		file, header, err := r.FormFile("file")
		if err != nil {
			t.Errorf("form file: %v", err)
		} else {
			defer file.Close()
			gotFileName = header.Filename
			data, _ := io.ReadAll(file)
			gotFile = string(data)
		}
		_, _ = w.Write([]byte(`{"documentId":17,"signingUrl":"https://sign.example/s/abc"}`))
	}))
	defer server.Close()

	client := NewDocumensoClient(server.URL+"/", "key-123", server.Client())
	result, err := client.Create(context.Background(), CreateRequest{
		Title:      "Contract",
		FileName:   "contract.pdf",
		File:       strings.NewReader("%PDF-1.7 body"),
		Recipients: []Recipient{{Email: "a@example.com", Name: "Alice", Role: "SIGNER"}, {Email: "b@example.com"}},
		ExternalID: "document:42",
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if gotMethod != http.MethodPost || gotPath != "/api/v1/documents" {
		t.Fatalf("unexpected request %s %s (trailing slash in base URL must be trimmed)", gotMethod, gotPath)
	}
	if gotAuth != "Bearer key-123" {
		t.Fatalf("authorization header = %q", gotAuth)
	}
	if gotFields["title"] != "Contract" || gotFields["externalId"] != "document:42" {
		t.Fatalf("unexpected fields %v", gotFields)
	}
	var recipients []Recipient
	if err := json.Unmarshal([]byte(gotFields["recipients"]), &recipients); err != nil {
		t.Fatalf("recipients field is not JSON: %v", err)
	}
	if len(recipients) != 2 || recipients[0] != (Recipient{Email: "a@example.com", Name: "Alice", Role: "SIGNER"}) || recipients[1].Email != "b@example.com" {
		t.Fatalf("unexpected recipients %+v", recipients)
	}
	if !strings.Contains(gotFields["recipients"], `"email":"b@example.com"}`) {
		t.Fatalf("empty name/role should be omitted: %s", gotFields["recipients"])
	}
	if gotFileName != "contract.pdf" || gotFile != "%PDF-1.7 body" {
		t.Fatalf("unexpected file %q = %q", gotFileName, gotFile)
	}
	if result.DocumentID != "17" || result.SigningURL != "https://sign.example/s/abc" {
		t.Fatalf("unexpected result %+v", result)
	}
}

func TestCreateParsesResponseShapes(t *testing.T) {
	tests := []struct {
		name    string
		body    string
		wantID  string
		wantURL string
		wantErr error
	}{
		{name: "documentId number", body: `{"documentId":123456789}`, wantID: "123456789"},
		{name: "documentId string", body: `{"documentId":"doc_abc","signingUrl":"https://x/s"}`, wantID: "doc_abc", wantURL: "https://x/s"},
		{name: "id fallback", body: `{"id":"envelope_1"}`, wantID: "envelope_1"},
		{name: "documentId wins over id", body: `{"id":"a","documentId":"b"}`, wantID: "b"},
		{name: "redirectUrl fallback", body: `{"id":5,"redirectUrl":"https://x/r"}`, wantID: "5", wantURL: "https://x/r"},
		{name: "signingUrl wins over redirectUrl", body: `{"id":5,"signingUrl":"https://x/s","redirectUrl":"https://x/r"}`, wantID: "5", wantURL: "https://x/s"},
		{name: "nested data", body: `{"data":{"id":9,"signingUrl":"https://x/d"}}`, wantID: "9", wantURL: "https://x/d"},
		{name: "top level wins over nested data", body: `{"id":"top","data":{"id":"nested","signingUrl":"https://x/d"}}`, wantID: "top", wantURL: "https://x/d"},
		{name: "unsupported id type", body: `{"id":true}`, wantErr: ErrInvalidResponse},
		{name: "no id", body: `{"signingUrl":"https://x/s"}`, wantErr: ErrInvalidResponse},
		{name: "empty object", body: `{}`, wantErr: ErrInvalidResponse},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				_, _ = w.Write([]byte(tc.body))
			}))
			defer server.Close()
			client := NewDocumensoClient(server.URL, "key", server.Client())
			result, err := client.Create(context.Background(), CreateRequest{FileName: "a.pdf", File: strings.NewReader("x")})
			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) {
					t.Fatalf("expected %v, got result %+v err %v", tc.wantErr, result, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("Create: %v", err)
			}
			if result.DocumentID != tc.wantID || result.SigningURL != tc.wantURL {
				t.Fatalf("got %+v, want id %q url %q", result, tc.wantID, tc.wantURL)
			}
		})
	}
}

func TestCreateErrors(t *testing.T) {
	t.Run("not configured", func(t *testing.T) {
		for _, client := range []*DocumensoClient{
			NewDocumensoClient("", "key", nil),
			NewDocumensoClient("https://sign.example", "", nil),
		} {
			if _, err := client.Create(context.Background(), CreateRequest{File: strings.NewReader("x")}); err == nil || !strings.Contains(err.Error(), "not configured") {
				t.Fatalf("expected not configured error, got %v", err)
			}
		}
	})

	t.Run("non-2xx includes status and trimmed body", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusUnprocessableEntity)
			_, _ = w.Write([]byte("  recipients are required \n"))
		}))
		defer server.Close()
		_, err := NewDocumensoClient(server.URL, "key", server.Client()).Create(context.Background(), CreateRequest{File: strings.NewReader("x")})
		if err == nil || !strings.Contains(err.Error(), "(422): recipients are required") {
			t.Fatalf("unexpected error %v", err)
		}
	})

	t.Run("non-2xx body is limited", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusBadGateway)
			_, _ = w.Write([]byte(strings.Repeat("e", 10000)))
		}))
		defer server.Close()
		_, err := NewDocumensoClient(server.URL, "key", server.Client()).Create(context.Background(), CreateRequest{File: strings.NewReader("x")})
		if err == nil || strings.Count(err.Error(), "e") > 4096+10 {
			t.Fatalf("expected error body capped at 4096 bytes, got %d bytes", len(err.Error()))
		}
	})

	t.Run("malformed JSON", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte("<html>"))
		}))
		defer server.Close()
		_, err := NewDocumensoClient(server.URL, "key", server.Client()).Create(context.Background(), CreateRequest{File: strings.NewReader("x")})
		if err == nil {
			t.Fatal("expected decode error")
		}
	})

	t.Run("file read failure", func(t *testing.T) {
		readErr := errors.New("disk gone")
		client := NewDocumensoClient("http://127.0.0.1:1", "key", nil)
		_, err := client.Create(context.Background(), CreateRequest{File: failingReader{readErr}})
		if !errors.Is(err, readErr) {
			t.Fatalf("expected read error, got %v", err)
		}
	})

	t.Run("cancelled context", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte(`{"id":1}`))
		}))
		defer server.Close()
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		_, err := NewDocumensoClient(server.URL, "key", server.Client()).Create(ctx, CreateRequest{File: strings.NewReader("x")})
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("expected context.Canceled, got %v", err)
		}
	})
}

type failingReader struct{ err error }

func (r failingReader) Read([]byte) (int, error) { return 0, r.err }

func TestDownloadCompletedDefaultURL(t *testing.T) {
	var gotPath, gotRawPath, gotAuth string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath, gotRawPath, gotAuth = r.URL.Path, r.URL.EscapedPath(), r.Header.Get("Authorization")
		_, _ = w.Write([]byte("signed pdf"))
	}))
	defer server.Close()

	client := NewDocumensoClient(server.URL, "key-123", server.Client())
	body, err := client.DownloadCompleted(context.Background(), "doc/1 x", "   ")
	if err != nil {
		t.Fatalf("DownloadCompleted: %v", err)
	}
	defer body.Close()
	data, _ := io.ReadAll(body)
	if string(data) != "signed pdf" {
		t.Fatalf("unexpected body %q", data)
	}
	if gotPath != "/api/v1/documents/doc/1 x/download" || gotRawPath != "/api/v1/documents/doc%2F1%20x/download" {
		t.Fatalf("document ID must be path-escaped, got path %q raw %q", gotPath, gotRawPath)
	}
	if gotAuth != "Bearer key-123" {
		t.Fatalf("authorization header = %q", gotAuth)
	}
}

func TestDownloadCompletedExplicitURL(t *testing.T) {
	var gotPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		_, _ = w.Write([]byte("ok"))
	}))
	defer server.Close()

	client := NewDocumensoClient(server.URL, "key", server.Client())
	body, err := client.DownloadCompleted(context.Background(), "ignored", server.URL+"/files/signed.pdf")
	if err != nil {
		t.Fatalf("DownloadCompleted: %v", err)
	}
	_ = body.Close()
	if gotPath != "/files/signed.pdf" {
		t.Fatalf("expected explicit URL to be used, got %q", gotPath)
	}
}

func TestDownloadCompletedRejectsUntrustedURLs(t *testing.T) {
	hits := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits++
	}))
	defer server.Close()
	client := NewDocumensoClient(server.URL, "key", server.Client())

	tests := []struct {
		name, url, wantErr string
	}{
		{"relative", "/api/v1/documents/1/download", "invalid signed document download URL"},
		{"ftp scheme", "ftp://" + strings.TrimPrefix(server.URL, "http://") + "/x", "invalid signed document download URL"},
		{"file scheme", "file:///etc/passwd", "invalid signed document download URL"},
		{"unparseable", "http://[::1", "invalid signed document download URL"},
		{"other host", "https://attacker.example/x.pdf", "configured provider host"},
		{"same hostname other port", "http://127.0.0.1:1/x.pdf", "configured provider host"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := client.DownloadCompleted(context.Background(), "1", tc.url)
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("expected %q error, got %v", tc.wantErr, err)
			}
		})
	}
	if hits != 0 {
		t.Fatalf("rejected URLs must not be fetched, server saw %d requests", hits)
	}
}

func TestDownloadCompletedHostMatchIsCaseInsensitive(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("ok"))
	}))
	defer server.Close()
	// "localhost" resolves to the test server; compare against an upper-cased base.
	port := server.URL[strings.LastIndex(server.URL, ":"):]
	client := NewDocumensoClient("http://LOCALHOST"+port, "key", server.Client())
	body, err := client.DownloadCompleted(context.Background(), "1", "http://localhost"+port+"/x.pdf")
	if err != nil {
		t.Fatalf("expected case-insensitive host match, got %v", err)
	}
	_ = body.Close()
}

func TestDownloadCompletedErrorStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "document not completed", http.StatusConflict)
	}))
	defer server.Close()
	_, err := NewDocumensoClient(server.URL, "key", server.Client()).DownloadCompleted(context.Background(), "1", "")
	if err == nil || !strings.Contains(err.Error(), "documenso download failed (409): document not completed") {
		t.Fatalf("unexpected error %v", err)
	}
}

func TestDownloadCompletedDoesNotDowngradeScheme(t *testing.T) {
	client := NewDocumensoClient("https://sign.example", "key", &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return nil, errors.New("request should have been rejected before sending")
	})})
	_, err := client.DownloadCompleted(context.Background(), "1", "http://sign.example/x.pdf")
	if err == nil || !strings.Contains(err.Error(), "download URL") {
		t.Fatalf("expected scheme downgrade to be rejected, got %v", err)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestStringifyID(t *testing.T) {
	tests := []struct {
		in   any
		want string
	}{
		{"abc", "abc"},
		{float64(42), "42"},
		{float64(9007199254740991), "9007199254740991"},
		{json.Number("77"), "77"},
		{nil, ""},
		{true, ""},
		{map[string]any{"id": 1}, ""},
	}
	for _, tc := range tests {
		if got := stringifyID(tc.in); got != tc.want {
			t.Errorf("stringifyID(%#v) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestDocumensoClientImplementsProvider(t *testing.T) {
	var _ Provider = NewDocumensoClient("https://sign.example", "key", nil)
}
