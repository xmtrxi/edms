package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/xmtrxi/edms/internal/config"
	"github.com/xmtrxi/edms/internal/handler"
)

func newTestServer(mutate func(*config.Config)) http.Handler {
	cfg := config.Config{
		App:      config.AppConfig{Env: "test"},
		Uploads:  config.UploadConfig{MaxBytes: 1 << 20},
		Security: config.SecurityConfig{BodyLimit: 1024, CORSOrigins: []string{"https://edms.example.com"}},
	}
	if mutate != nil {
		mutate(&cfg)
	}
	return New(cfg, handler.Deps{Cfg: cfg}).Handler()
}

func serve(h http.Handler, req *http.Request) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestCORSUsesConfiguredOrigins(t *testing.T) {
	h := newTestServer(nil)
	preflight := func(origin string) string {
		req := httptest.NewRequest(http.MethodOptions, "/api/health", nil)
		req.Header.Set("Origin", origin)
		req.Header.Set("Access-Control-Request-Method", http.MethodGet)
		return serve(h, req).Header().Get("Access-Control-Allow-Origin")
	}
	if got := preflight("https://edms.example.com"); got != "https://edms.example.com" {
		t.Fatalf("configured origin not allowed, got %q", got)
	}
	if got := preflight("http://localhost:3000"); got != "" {
		t.Fatalf("unconfigured origin allowed: %q", got)
	}
}

func TestSecurityHeaders(t *testing.T) {
	rec := serve(newTestServer(nil), httptest.NewRequest(http.MethodGet, "/health", nil))
	for header, want := range map[string]string{
		"X-Content-Type-Options": "nosniff",
		"X-Frame-Options":        "DENY",
		"Referrer-Policy":        "no-referrer",
	} {
		if got := rec.Header().Get(header); got != want {
			t.Errorf("%s = %q, want %q", header, got, want)
		}
	}
}

func TestBodyLimit(t *testing.T) {
	h := newTestServer(nil)
	big := strings.Repeat("x", 4096)

	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader(big))
	req.Header.Set("Content-Type", "application/json")
	if rec := serve(h, req); rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("oversized JSON: status %d, want 413", rec.Code)
	}

	// Upload chunks may be larger than ordinary requests.
	req = httptest.NewRequest(http.MethodPut, "/api/uploads/uploads:1", strings.NewReader(big))
	if rec := serve(h, req); rec.Code == http.StatusRequestEntityTooLarge {
		t.Fatal("upload chunk under the upload limit was refused")
	}
}

func TestMonitoringNeedsCredentials(t *testing.T) {
	open := newTestServer(nil)
	if rec := serve(open, httptest.NewRequest(http.MethodGet, "/monitoring/tasks/", nil)); rec.Code != http.StatusNotFound {
		t.Fatalf("task dashboard without credentials configured: status %d, want 404", rec.Code)
	}
	if rec := serve(open, httptest.NewRequest(http.MethodGet, "/metrics", nil)); rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "edms_http_requests_total") {
		t.Fatalf("metrics without credentials configured: status %d", rec.Code)
	}

	guarded := newTestServer(func(cfg *config.Config) {
		cfg.Security.MonitoringUser, cfg.Security.MonitoringPassword = "ops", "secret"
		cfg.Queue.RedisURL = "redis://127.0.0.1:1"
	})
	for _, path := range []string{"/metrics", "/monitoring/tasks/"} {
		if rec := serve(guarded, httptest.NewRequest(http.MethodGet, path, nil)); rec.Code != http.StatusUnauthorized {
			t.Fatalf("%s without credentials: status %d, want 401", path, rec.Code)
		}
		req := httptest.NewRequest(http.MethodGet, path, nil)
		req.SetBasicAuth("ops", "wrong")
		if rec := serve(guarded, req); rec.Code != http.StatusUnauthorized {
			t.Fatalf("%s with a wrong password: status %d, want 401", path, rec.Code)
		}
	}
	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	req.SetBasicAuth("ops", "secret")
	if rec := serve(guarded, req); rec.Code != http.StatusOK {
		t.Fatalf("metrics with credentials: status %d", rec.Code)
	}
}
