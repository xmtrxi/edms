package api

import (
	"crypto/rand"
	"encoding/hex"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/labstack/echo/v5"
)

// documentStore keeps the first document API slice usable before the database
// repository is connected. Its interface is deliberately isolated in this file
// so it can be swapped for SurrealDB without changing the HTTP contract.
type documentStore struct {
	mu    sync.RWMutex
	items map[string]storedDocument
}

type storedDocument struct {
	DocumentResponse
	content []byte
}

type DocumentResponse struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	MimeType  string    `json:"mime_type"`
	Size      int64     `json:"size"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type documentListResponse struct {
	Items []DocumentResponse `json:"items"`
}

func newDocumentStore() *documentStore {
	return &documentStore{items: make(map[string]storedDocument)}
}

func (s *Server) listDocuments(c *echo.Context) error {
	query := strings.ToLower(strings.TrimSpace(c.QueryParam("q")))
	status := strings.TrimSpace(c.QueryParam("status"))

	s.documents.mu.RLock()
	items := make([]DocumentResponse, 0, len(s.documents.items))
	for _, document := range s.documents.items {
		if query != "" && !strings.Contains(strings.ToLower(document.Name), query) {
			continue
		}
		if status != "" && document.Status != status {
			continue
		}
		items = append(items, document.DocumentResponse)
	}
	s.documents.mu.RUnlock()

	// Most recently added documents are the useful default in a workspace.
	for i := 0; i < len(items)/2; i++ {
		items[i], items[len(items)-1-i] = items[len(items)-1-i], items[i]
	}
	return c.JSON(http.StatusOK, documentListResponse{Items: items})
}

func (s *Server) uploadDocument(c *echo.Context) error {
	file, err := c.FormFile("file")
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "a file is required"})
	}

	src, err := file.Open()
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "could not read uploaded file"})
	}
	defer src.Close()
	content, err := io.ReadAll(src)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "could not store uploaded file"})
	}

	name := strings.TrimSpace(c.FormValue("name"))
	if name == "" {
		name = file.Filename
	}
	name = filepath.Base(name)
	if name == "." || name == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "a valid filename is required"})
	}

	id, err := newDocumentID()
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "could not create document"})
	}
	now := time.Now().UTC()
	document := storedDocument{DocumentResponse: DocumentResponse{
		ID: id, Name: name, MimeType: file.Header.Get("Content-Type"), Size: int64(len(content)),
		Status: "ready", CreatedAt: now, UpdatedAt: now,
	}, content: content}
	if document.MimeType == "" {
		document.MimeType = "application/octet-stream"
	}

	s.documents.mu.Lock()
	s.documents.items[id] = document
	s.documents.mu.Unlock()
	return c.JSON(http.StatusCreated, document.DocumentResponse)
}

func (s *Server) getDocument(c *echo.Context) error {
	document, ok := s.documentByID(c.Param("id"))
	if !ok {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "document not found"})
	}
	return c.JSON(http.StatusOK, document.DocumentResponse)
}

func (s *Server) downloadDocument(c *echo.Context) error {
	document, ok := s.documentByID(c.Param("id"))
	if !ok {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "document not found"})
	}
	c.Response().Header().Set(echo.HeaderContentType, document.MimeType)
	c.Response().Header().Set(echo.HeaderContentDisposition, "attachment; filename=\""+strings.ReplaceAll(document.Name, "\"", "")+"\"")
	return c.Blob(http.StatusOK, document.MimeType, document.content)
}

func (s *Server) deleteDocument(c *echo.Context) error {
	id := c.Param("id")
	s.documents.mu.Lock()
	_, ok := s.documents.items[id]
	delete(s.documents.items, id)
	s.documents.mu.Unlock()
	if !ok {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "document not found"})
	}
	return c.NoContent(http.StatusNoContent)
}

func (s *Server) documentByID(id string) (storedDocument, bool) {
	s.documents.mu.RLock()
	document, ok := s.documents.items[id]
	s.documents.mu.RUnlock()
	return document, ok
}

func newDocumentID() (string, error) {
	bytes := make([]byte, 10)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return "doc_" + hex.EncodeToString(bytes), nil
}
