package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
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
	access  *AccessService
	log     *slog.Logger
}

func NewSearchService(client *surrealdb.DB, provider storage.Provider, s *search.Client, logger *slog.Logger) *SearchService {
	return &SearchService{client: client, storage: provider, search: s, access: NewAccessService(client), log: logger}
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
	DocumentID  string
	Mode        string
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
	Passage  SearchPassage
}

type SearchPassage struct {
	VersionID   string
	ChunkIndex  int
	Headings    []string
	PageNumbers []int
	PageStart   int
	PageEnd     int
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
	folderIDs, err := s.access.ViewableFolderIDs(ctx, in.OwnerID, "view")
	if err != nil {
		return nil, err
	}
	if in.DocumentID != "" {
		docs, err := s.documentsByIDs(ctx, []string{in.DocumentID})
		if err != nil {
			return nil, err
		}
		if len(docs) == 0 {
			return nil, ErrDocumentNotFound
		}
		allowed, err := s.access.CanDocument(ctx, in.OwnerID, "view", &docs[0])
		if err != nil || !allowed {
			return nil, ErrAccessDenied
		}
	}
	res, err := s.search.Search(ctx, search.SearchInput{
		Query:      in.Query,
		UserID:     in.OwnerID,
		FolderIDs:  folderIDs,
		DocumentID: in.DocumentID,
		Status:     in.Status,
		Mode:       in.Mode,
		Page:       in.Page,
		PageSize:   in.PageSize,
	})
	if err != nil {
		return nil, err
	}

	items := make([]SearchItem, 0, len(res.Hits))
	if len(res.Hits) > 0 {
		ids := make([]string, 0, len(res.Hits))
		for _, hit := range res.Hits {
			ids = append(ids, hit.DocumentID)
		}
		docs, err := s.documentsByIDs(ctx, ids)
		if err != nil {
			return nil, err
		}
		byID := make(map[string]database.Document, len(docs)*2)
		for _, doc := range docs {
			for _, form := range documentIDForms(string(doc.ID)) {
				byID[form] = doc
			}
		}
		for _, hit := range res.Hits {
			doc, ok := byID[hit.DocumentID]
			if !ok {
				continue
			}
			if doc.CurrentVersionID != "" && hit.VersionID != doc.CurrentVersionID {
				continue
			}
			allowed, accessErr := s.access.CanDocument(ctx, in.OwnerID, "view", &doc)
			if accessErr != nil || !allowed {
				continue
			}
			items = append(items, SearchItem{Document: doc, Snippet: hit.Snippet, Passage: SearchPassage{
				VersionID: hit.VersionID, ChunkIndex: hit.ChunkIndex, Headings: hit.Headings,
				PageNumbers: hit.PageNumbers, PageStart: hit.PageStart, PageEnd: hit.PageEnd,
			}})
		}
	}

	return &SearchResult{Items: items, Total: res.TotalHits, Page: res.Page, PageSize: res.PageSize}, nil
}

// IndexDocument rebuilds one document from its versioned chunk artifact.
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
	chunks, err := s.chunksForIndex(ctx, doc, extraction)
	if err != nil {
		return err
	}
	if len(chunks) == 0 {
		return s.search.DeleteDocument(ctx, documentID)
	}
	generationID := chunks[0].GenerationID
	if err := s.search.IndexChunks(ctx, chunks); err != nil {
		return err
	}
	return s.search.DeleteOlderGenerations(ctx, string(doc.ID), generationID)
}

func (s *SearchService) chunksForIndex(ctx context.Context, doc *database.Document, extraction *database.DocumentExtraction) ([]search.DocumentChunk, error) {
	if extraction != nil {
		if extractedVersion := metadataString(extraction.Metadata, "version_id", ""); extractedVersion != "" && doc.CurrentVersionID != "" && extractedVersion != doc.CurrentVersionID {
			return nil, nil
		}
	}
	path := metadataString(doc.Metadata, "ocr_chunks_path", "")
	if path == "" && extraction != nil {
		path = metadataString(extraction.Metadata, "chunks_path", "")
	}
	if path != "" && s.storage != nil {
		reader, err := s.storage.Download(ctx, path)
		if err == nil {
			defer reader.Close()
			decoder := json.NewDecoder(reader)
			chunks := []search.DocumentChunk{}
			for {
				var chunk search.DocumentChunk
				if err := decoder.Decode(&chunk); err == io.EOF {
					break
				} else if err != nil {
					return nil, fmt.Errorf("decode chunk artifact: %w", err)
				}
				chunk.DocumentID = string(doc.ID)
				chunk.OwnerID, chunk.FolderID, chunk.Status = doc.OwnerID, doc.FolderID, doc.Status
				chunks = append(chunks, chunk)
			}
			if len(chunks) > 0 && chunks[0].VersionID == doc.CurrentVersionID {
				return chunks, nil
			}
		} else {
			s.log.Warn("read chunk artifact for indexing", "error", err, "document_id", doc.ID, "path", path)
		}
	}
	content, _, _, err := s.indexContent(ctx, doc, extraction)
	if err != nil || strings.TrimSpace(content) == "" {
		return nil, err
	}
	versionID := doc.CurrentVersionID
	if versionID == "" {
		versionID = "legacy"
	}
	generationID := metadataString(doc.Metadata, "ocr_generation_id", doc.Checksum)
	if generationID == "" {
		generationID = "legacy"
	}
	return buildSearchChunks(doc, versionID, generationID, nil, content, time.Now().UTC()), nil
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

// ReindexAll builds all current-version chunks before atomically replacing the
// live index. Search remains available from the previous generation until the
// replacement is fully populated.
func (s *SearchService) ReindexAll(ctx context.Context) (int, error) {
	if s.search == nil {
		return 0, fmt.Errorf("search is not configured")
	}

	docs, err := s.allDocuments(ctx)
	if err != nil {
		return 0, err
	}

	indexed := 0
	allChunks := []search.DocumentChunk{}
	for _, doc := range docs {
		if doc.Status == database.DocumentStatusArchived || doc.Status == database.DocumentStatusFailed {
			continue
		}
		extraction, err := surrealgoorm.Query[database.DocumentExtraction](s.client, "document_extractions").WhereEq("document_id", string(doc.ID)).First(ctx)
		if err != nil {
			s.log.Warn("load extraction for reindex", "error", err, "document_id", doc.ID)
			continue
		}
		chunks, err := s.chunksForIndex(ctx, &doc, extraction)
		if err != nil {
			s.log.Warn("index document", "error", err, "document_id", doc.ID)
			continue
		}
		if len(chunks) == 0 {
			continue
		}
		allChunks = append(allChunks, chunks...)
		indexed++
	}
	if err := s.search.ReplaceAll(ctx, allChunks); err != nil {
		return 0, err
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
