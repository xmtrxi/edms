package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/xmtrxi/edms/internal/config"
	"github.com/xmtrxi/edms/internal/storage"
)

// ---------- in-memory mock storage provider ----------

type mockStorage struct {
	mu    sync.RWMutex
	files map[string][]byte
}

var _ storage.Provider = (*mockStorage)(nil)

func newMockStorage() *mockStorage {
	return &mockStorage{files: make(map[string][]byte)}
}

func (m *mockStorage) Name() string { return "mock" }
func (m *mockStorage) Upload(_ context.Context, path string, file io.Reader) error {
	data, err := io.ReadAll(file)
	if err != nil {
		return err
	}
	m.mu.Lock()
	m.files[path] = data
	m.mu.Unlock()
	return nil
}
func (m *mockStorage) Download(_ context.Context, path string) (io.ReadCloser, error) {
	m.mu.RLock()
	data, ok := m.files[path]
	m.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("not found: %s", path)
	}
	return io.NopCloser(bytes.NewReader(data)), nil
}
func (m *mockStorage) Move(_ context.Context, prev, next string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	data, ok := m.files[prev]
	if !ok {
		return fmt.Errorf("not found: %s", prev)
	}
	m.files[next] = data
	delete(m.files, prev)
	return nil
}
func (m *mockStorage) Delete(_ context.Context, path string) error {
	m.mu.Lock()
	delete(m.files, path)
	m.mu.Unlock()
	return nil
}
func (m *mockStorage) HealthCheck(context.Context) error                       { return nil }
func (m *mockStorage) Exists(_ context.Context, path string) (bool, error) {
	m.mu.RLock()
	_, ok := m.files[path]
	m.mu.RUnlock()
	return ok, nil
}
func (m *mockStorage) GetUrl(_ context.Context, path string) (string, error) {
	return "https://mock.s3/" + path + "?signed=1", nil
}
func (m *mockStorage) List(_ context.Context, _ string, _ *string, _ int) ([]string, *string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	keys := make([]string, 0, len(m.files))
	for k := range m.files {
		keys = append(keys, k)
	}
	return keys, nil, nil
}

// ---------- tests ----------

func TestHealthReportsSigningDisabledByDefault(t *testing.T) {
	server := New(config.Config{
		App: config.AppConfig{
			Env:  "test",
			Port: "0",
		},
	}, newMockStorage())

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	server.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var body struct {
		Status          string          `json:"status"`
		Env             string          `json:"env"`
		StorageProvider string          `json:"storage_provider"`
		Features        map[string]bool `json:"features"`
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
	if body.StorageProvider != "mock" {
		t.Fatalf("expected storage_provider mock, got %q", body.StorageProvider)
	}
}

func TestSigningRouteIsOptIn(t *testing.T) {
	disabled := New(config.Config{}, newMockStorage())
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
	}, newMockStorage())
	enabledReq := httptest.NewRequest(http.MethodPost, "/api/documents/documents:1/sign", nil)
	enabledRec := httptest.NewRecorder()
	enabled.Handler().ServeHTTP(enabledRec, enabledReq)

	if enabledRec.Code != http.StatusNotImplemented {
		t.Fatalf("expected enabled signing route status %d, got %d", http.StatusNotImplemented, enabledRec.Code)
	}
}

func TestOpenAPIDocsAreServed(t *testing.T) {
	server := New(config.Config{}, newMockStorage())

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
