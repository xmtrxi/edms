package service

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"maps"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"

	surrealgoorm "github.com/kemboi22/surreal-go-orm"
	"github.com/surrealdb/surrealdb.go"
	"github.com/surrealdb/surrealdb.go/pkg/models"

	"github.com/xmtrxi/edms/internal/database"
	markdownutil "github.com/xmtrxi/edms/internal/markdown"
	"github.com/xmtrxi/edms/internal/search"
	"github.com/xmtrxi/edms/internal/storage"
)

const maxSearchContentBytes = 150_000

type SearchService struct {
	client  *surrealdb.DB
	storage storage.Provider
	search  *search.Client
	log     *slog.Logger
}

func NewSearchService(client *surrealdb.DB, provider storage.Provider, s *search.Client, logger *slog.Logger) *SearchService {
	return &SearchService{client: client, storage: provider, search: s, log: logger}
}

type SearchInput struct {
	Query      string
	OwnerID    string
	Status     string
	FolderPath string
	FolderID   string
	// DocumentIDs, when set, restricts results to these ids (used for
	// category filtering). An empty slice means "no restriction".
	DocumentIDs []string
	Page        int
	PageSize    int
}

type SearchResult struct {
	Items    []SearchItem
	Total    int64
	Page     int
	PageSize int
}

type SearchItem struct {
	Document database.Document
	Snippet  string
}

func (s *SearchService) Search(ctx context.Context, in SearchInput) (*SearchResult, error) {
	if in.PageSize <= 0 {
		in.PageSize = 50
	}
	if in.Page <= 0 {
		in.Page = 1
	}

	// Meilisearch has no view of category assignments, so fall back to the
	// database whenever the results must be restricted to a set of ids.
	if in.Query != "" && in.FolderPath == "" && s.search != nil && len(in.DocumentIDs) == 0 {
		result, err := s.searchMeili(ctx, in)
		if err == nil {
			return result, nil
		}
		s.log.Warn("meilisearch query failed; using database fallback", "error", err, "query", in.Query)
	}

	offset := (in.Page - 1) * in.PageSize
	ocrIDs, err := s.matchingExtractionIDs(ctx, in.Query)
	if err != nil {
		return nil, err
	}
	items, err := s.listDocuments(ctx, in.OwnerID, in.Query, in.Status, in.FolderPath, in.FolderID, in.DocumentIDs, ocrIDs, in.PageSize, offset)
	if err != nil {
		return nil, err
	}
	total, err := s.countDocuments(ctx, in.OwnerID, in.Query, in.Status, in.FolderPath, in.FolderID, in.DocumentIDs, ocrIDs)
	if err != nil {
		return nil, err
	}
	return &SearchResult{Items: items, Total: total, Page: in.Page, PageSize: in.PageSize}, nil
}

func (s *SearchService) searchMeili(ctx context.Context, in SearchInput) (*SearchResult, error) {
	res, err := s.search.Search(ctx, search.SearchInput{
		Query:    in.Query,
		OwnerID:  in.OwnerID,
		Status:   in.Status,
		Page:     in.Page,
		PageSize: in.PageSize,
	})
	if err != nil {
		return nil, err
	}

	items := make([]SearchItem, 0, len(res.Hits))
	if len(res.Hits) > 0 {
		ids := make([]string, 0, len(res.Hits))
		snippets := make(map[string]string, len(res.Hits))
		for _, hit := range res.Hits {
			ids = append(ids, hit.RecordID)
			snippets[hit.RecordID] = hit.Snippet
		}
		docs, err := s.documentsByIDs(ctx, ids)
		if err != nil {
			return nil, err
		}
		for _, doc := range docs {
			items = append(items, SearchItem{Document: doc, Snippet: snippets[string(doc.ID)]})
		}
	}

	return &SearchResult{Items: items, Total: res.TotalHits, Page: res.Page, PageSize: res.PageSize}, nil
}

// IndexDocument indexes a single document (name + extracted content) into
// Meilisearch. No-op when search is disabled or the document has no OCR data.
func (s *SearchService) IndexDocument(ctx context.Context, documentID string) error {
	if s.search == nil {
		return nil
	}

	rid, err := database.ParseRecordID(documentID)
	if err != nil {
		return err
	}
	doc, err := surrealdb.Select[database.Document](ctx, s.client, rid)
	if err != nil {
		return err
	}
	if doc == nil {
		return ErrDocumentNotFound
	}

	extraction, err := surrealgoorm.Query[database.DocumentExtraction](s.client, "document_extractions").WhereEq("document_id", documentID).First(ctx)
	if err != nil {
		return err
	}
	content, summary, ocrEngine, err := s.indexContent(ctx, doc, extraction)
	if err != nil {
		return err
	}
	if strings.TrimSpace(content) == "" && strings.TrimSpace(summary) == "" {
		return nil
	}
	metadata := maps.Clone(doc.Metadata)
	if metadata == nil {
		metadata = map[string]any{}
	}

	return s.search.IndexDocument(ctx, search.Document{
		RecordID:     string(doc.ID),
		OwnerID:      doc.OwnerID,
		Name:         doc.Name,
		OriginalName: doc.OriginalName,
		MimeType:     doc.MimeType,
		Content:      capUTF8Bytes(content, maxSearchContentBytes),
		Summary:      summary,
		Extension:    metadataString(metadata, "extension", strings.TrimPrefix(strings.ToLower(filepath.Ext(doc.OriginalName)), ".")),
		OCREngine:    ocrEngine,
		Status:       doc.Status,
		CreatedAt:    doc.CreatedAt,
		UpdatedAt:    time.Now().UTC(),
	})
}

func (s *SearchService) indexContent(ctx context.Context, doc *database.Document, extraction *database.DocumentExtraction) (content, summary, engine string, err error) {
	if extraction != nil {
		content = markdownutil.WithoutEmbeddedDataImages(extraction.Text)
		summary = content
		engine = metadataString(extraction.Metadata, "engine", "")
		if path := metadataString(extraction.Metadata, "full_text_path", ""); path != "" && s.storage != nil {
			if stored, readErr := s.readStorageText(ctx, path, maxSearchContentBytes+1); readErr == nil && strings.TrimSpace(stored) != "" {
				content = markdownutil.WithoutEmbeddedDataImages(stored)
			} else if readErr != nil {
				s.log.Warn("read OCR text artifact for indexing", "error", readErr, "document_id", doc.ID, "path", path)
			}
		}
		return content, capUTF8Bytes(summary, 600), engine, nil
	}

	if s.storage == nil {
		return "", "", "", nil
	}
	metadata := doc.Metadata
	path := metadataString(metadata, "ocr_text_path", "")
	if path == "" {
		return "", "", metadataString(metadata, "ocr_engine", ""), nil
	}
	content, err = s.readStorageText(ctx, path, maxSearchContentBytes+1)
	if err != nil {
		return "", "", "", err
	}
	content = markdownutil.WithoutEmbeddedDataImages(content)
	return content, capUTF8Bytes(content, 600), metadataString(metadata, "ocr_engine", ""), nil
}

func (s *SearchService) readStorageText(ctx context.Context, path string, limit int) (string, error) {
	reader, err := s.storage.Download(ctx, path)
	if err != nil {
		return "", err
	}
	defer reader.Close()
	payload, err := io.ReadAll(io.LimitReader(reader, int64(limit)))
	if err != nil {
		return "", err
	}
	return string(payload), nil
}

func (s *SearchService) DeleteDocument(ctx context.Context, documentID string) error {
	if s.search == nil {
		return nil
	}
	return s.search.DeleteDocument(ctx, documentID)
}

func (s *SearchService) Enabled() bool {
	return s.search != nil
}

// ReindexAll rebuilds the search index for every document. Archived and failed
// documents are removed from the index (pruning stale entries). Returns the
// number of documents indexed.
func (s *SearchService) ReindexAll(ctx context.Context) (int, error) {
	if s.search == nil {
		return 0, fmt.Errorf("search is not configured")
	}

	docs, err := s.allDocuments(ctx)
	if err != nil {
		return 0, err
	}

	indexed := 0
	for _, doc := range docs {
		if doc.Status == database.DocumentStatusArchived || doc.Status == database.DocumentStatusFailed {
			if err := s.DeleteDocument(ctx, string(doc.ID)); err != nil {
				s.log.Warn("remove stale document from index", "error", err, "document_id", doc.ID)
			}
			continue
		}
		if err := s.IndexDocument(ctx, string(doc.ID)); err != nil {
			s.log.Warn("index document", "error", err, "document_id", doc.ID)
			continue
		}
		indexed++
	}
	return indexed, nil
}

func (s *SearchService) allDocuments(ctx context.Context) ([]database.Document, error) {
	results, err := surrealdb.Query[[]database.Document](ctx, s.client, "SELECT * FROM documents ORDER BY created_at", nil)
	if err != nil {
		return nil, err
	}
	if results == nil || len(*results) == 0 {
		return []database.Document{}, nil
	}
	return (*results)[0].Result, nil
}

func (s *SearchService) listDocuments(ctx context.Context, ownerID, query, status, folderPath, folderID string, documentIDs, ocrIDs []string, limit, offset int) ([]SearchItem, error) {
	sql := "SELECT * FROM documents WHERE owner_id = $owner"
	vars := map[string]any{
		"owner":  ownerID,
		"limit":  limit,
		"offset": offset,
	}
	if query != "" {
		sql += " AND (string::lowercase(name) CONTAINS $query OR string::lowercase(original_name) CONTAINS $query"
		if len(ocrIDs) > 0 {
			recordIDs, err := parseRecordIDs(ocrIDs)
			if err != nil {
				return nil, err
			}
			sql += " OR id IN $ocr_documents"
			vars["ocr_documents"] = recordIDs
		}
		sql += ")"
		vars["query"] = strings.ToLower(query)
	}
	if status != "" {
		sql += " AND status = $status"
		vars["status"] = status
	}
	if folderPath != "" {
		sql += " AND metadata.folder_path = $folder_path"
		vars["folder_path"] = cleanFolderPath(folderPath)
	}
	if folderID != "" {
		sql += " AND folder_id = $folder_id"
		vars["folder_id"] = folderID
	}
	if len(documentIDs) > 0 {
		recordIDs, err := parseRecordIDs(documentIDs)
		if err != nil {
			return nil, err
		}
		sql += " AND id IN $documents"
		vars["documents"] = recordIDs
	}
	sql += " ORDER BY created_at LIMIT $limit START $offset"

	results, err := surrealdb.Query[[]database.Document](ctx, s.client, sql, vars)
	if err != nil {
		return nil, err
	}
	if results == nil || len(*results) == 0 {
		return []SearchItem{}, nil
	}
	items := make([]SearchItem, 0, len((*results)[0].Result))
	for _, doc := range (*results)[0].Result {
		items = append(items, SearchItem{Document: doc})
	}
	return items, nil
}

func (s *SearchService) countDocuments(ctx context.Context, ownerID, query, status, folderPath, folderID string, documentIDs, ocrIDs []string) (int64, error) {
	sql := "SELECT count() AS total FROM documents WHERE owner_id = $owner"
	vars := map[string]any{"owner": ownerID}
	if query != "" {
		sql += " AND (string::lowercase(name) CONTAINS $query OR string::lowercase(original_name) CONTAINS $query"
		if len(ocrIDs) > 0 {
			recordIDs, err := parseRecordIDs(ocrIDs)
			if err != nil {
				return 0, err
			}
			sql += " OR id IN $ocr_documents"
			vars["ocr_documents"] = recordIDs
		}
		sql += ")"
		vars["query"] = strings.ToLower(query)
	}
	if status != "" {
		sql += " AND status = $status"
		vars["status"] = status
	}
	if folderPath != "" {
		sql += " AND metadata.folder_path = $folder_path"
		vars["folder_path"] = cleanFolderPath(folderPath)
	}
	if folderID != "" {
		sql += " AND folder_id = $folder_id"
		vars["folder_id"] = folderID
	}
	if len(documentIDs) > 0 {
		recordIDs, err := parseRecordIDs(documentIDs)
		if err != nil {
			return 0, err
		}
		sql += " AND id IN $documents"
		vars["documents"] = recordIDs
	}
	sql += " GROUP ALL"

	results, err := surrealdb.Query[[]map[string]any](ctx, s.client, sql, vars)
	if err != nil {
		return 0, err
	}
	if results == nil || len(*results) == 0 || len((*results)[0].Result) == 0 {
		return 0, nil
	}
	switch total := (*results)[0].Result[0]["total"].(type) {
	case int64:
		return total, nil
	case uint64:
		return int64(total), nil
	case float64:
		return int64(total), nil
	case int:
		return int64(total), nil
	}
	return 0, nil
}

func (s *SearchService) matchingExtractionIDs(ctx context.Context, query string) ([]string, error) {
	query = strings.TrimSpace(strings.ToLower(query))
	if query == "" {
		return nil, nil
	}
	results, err := surrealdb.Query[[]database.DocumentExtraction](ctx, s.client,
		"SELECT * FROM document_extractions WHERE string::lowercase(text) CONTAINS $query LIMIT 500",
		map[string]any{"query": query},
	)
	if err != nil {
		return nil, err
	}
	if results == nil || len(*results) == 0 {
		return nil, nil
	}
	ids := make([]string, 0, len((*results)[0].Result))
	for _, extraction := range (*results)[0].Result {
		if extraction.DocumentID != "" {
			ids = append(ids, extraction.DocumentID)
		}
	}
	return ids, nil
}

func parseRecordIDs(ids []string) ([]models.RecordID, error) {
	recordIDs := make([]models.RecordID, 0, len(ids))
	for _, id := range ids {
		rid, err := database.ParseRecordID(id)
		if err != nil {
			return nil, fmt.Errorf("parse document id %q: %w", id, err)
		}
		recordIDs = append(recordIDs, rid)
	}
	return recordIDs, nil
}

func metadataString(metadata map[string]any, key, fallback string) string {
	if value, ok := metadata[key].(string); ok && strings.TrimSpace(value) != "" {
		return value
	}
	return fallback
}

func capUTF8Bytes(value string, limit int) string {
	if limit <= 0 || len(value) <= limit {
		return value
	}
	capped := value[:limit]
	for !utf8.ValidString(capped) && limit > 0 {
		limit--
		capped = value[:limit]
	}
	return capped + "\n\n[truncated - full text is stored with the OCR artifacts]"
}

func (s *SearchService) documentsByIDs(ctx context.Context, ids []string) ([]database.Document, error) {
	if len(ids) == 0 {
		return []database.Document{}, nil
	}

	recordIDs, err := parseRecordIDs(ids)
	if err != nil {
		return nil, err
	}

	results, err := surrealdb.Query[[]database.Document](ctx, s.client, "SELECT * FROM documents WHERE id IN $ids", map[string]any{"ids": recordIDs})
	if err != nil {
		return nil, err
	}
	if results == nil || len(*results) == 0 {
		return []database.Document{}, nil
	}

	docs := (*results)[0].Result
	byID := make(map[string]database.Document, len(docs)*3)
	for _, d := range docs {
		for _, form := range documentIDForms(string(d.ID)) {
			byID[form] = d
		}
	}

	ordered := make([]database.Document, 0, len(ids))
	for _, id := range ids {
		for _, form := range documentIDForms(id) {
			if d, ok := byID[form]; ok {
				ordered = append(ordered, d)
				break
			}
		}
	}
	return ordered, nil
}

// documentIDForms accepts both values returned by SurrealDB and the canonical
// record id stored in Meilisearch: "doc_123" and "documents:doc_123".
func documentIDForms(id string) []string {
	id = strings.TrimSpace(id)
	if id == "" {
		return nil
	}
	forms := []string{id}
	if table, key, ok := strings.Cut(id, ":"); ok && table == "documents" && key != "" {
		return append(forms, key)
	}
	return append(forms, "documents:"+id)
}
