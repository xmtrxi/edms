package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/xmtrxi/edms/internal/config"
)

func TestHealthReportsSigningDisabledByDefault(t *testing.T) {
	server := New(config.Config{
		App: config.AppConfig{
			Env:  "test",
			Port: "0",
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	server.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var body struct {
		Status   string          `json:"status"`
		Env      string          `json:"env"`
		Features map[string]bool `json:"features"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode health response: %v", err)
	}

	if body.Status != "ok" {
		t.Fatalf("expected health status ok, got %q", body.Status)
	}
	if body.Features["signing"] {
		t.Fatal("expected signing to be disabled")
	}
}

func TestSigningRouteIsOptIn(t *testing.T) {
	disabled := New(config.Config{})
	disabledReq := httptest.NewRequest(http.MethodPost, "/api/documents/documents:1/sign", nil)
	disabledRec := httptest.NewRecorder()
	disabled.Handler().ServeHTTP(disabledRec, disabledReq)

	if disabledRec.Code != http.StatusNotFound {
		t.Fatalf("expected disabled signing route status %d, got %d", http.StatusNotFound, disabledRec.Code)
	}

	enabled := New(config.Config{
		Signing: config.SigningConfig{
			Enabled: true,
		},
	})
	enabledReq := httptest.NewRequest(http.MethodPost, "/api/documents/documents:1/sign", nil)
	enabledRec := httptest.NewRecorder()
	enabled.Handler().ServeHTTP(enabledRec, enabledReq)

	if enabledRec.Code != http.StatusNotImplemented {
		t.Fatalf("expected enabled signing route status %d, got %d", http.StatusNotImplemented, enabledRec.Code)
	}
}

func TestOpenAPIDocsAreServed(t *testing.T) {
	server := New(config.Config{})

	specReq := httptest.NewRequest(http.MethodGet, "/openapi.yaml", nil)
	specRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(specRec, specReq)

	if specRec.Code != http.StatusOK {
		t.Fatalf("expected openapi status %d, got %d", http.StatusOK, specRec.Code)
	}
	if !strings.Contains(specRec.Body.String(), "openapi: 3.0.3") {
		t.Fatalf("expected openapi yaml, got %q", specRec.Body.String())
	}

	docsReq := httptest.NewRequest(http.MethodGet, "/docs", nil)
	docsRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(docsRec, docsReq)

	if docsRec.Code != http.StatusOK {
		t.Fatalf("expected docs status %d, got %d", http.StatusOK, docsRec.Code)
	}
	if !strings.Contains(docsRec.Body.String(), "SwaggerUIBundle") {
		t.Fatalf("expected swagger ui html, got %q", docsRec.Body.String())
	}
}
