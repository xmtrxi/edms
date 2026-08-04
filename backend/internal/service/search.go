package service

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	surrealgoorm "github.com/kemboi22/surreal-go-orm"
	"github.com/surrealdb/surrealdb.go"

	"github.com/xmtrxi/edms/internal/database"
	"github.com/xmtrxi/edms/internal/search"
)

type SearchService struct {
	client *surrealdb.DB
	search *search.Client
	log    *slog.Logger
}

func NewSearchService(client *surrealdb.DB, s *search.Client, logger *slog.Logger) *SearchService {
	return &SearchService{client: client, search: s, log: logger}
}

type SearchInput struct {
	Query    string
	OwnerID  string
	Status   string
	Page     int
	PageSize int
}

type SearchResult struct {
	Items    []database.Document
	Total    int64
	Page     int
	PageSize int
}

func (s *SearchService) Search(ctx context.Context, in SearchInput) (*SearchResult, error) {
	if in.PageSize <= 0 {
		in.PageSize = 50
	}
	if in.Page <= 0 {
		in.Page = 1
	}

	if in.Query != "" && s.search != nil {
		return s.searchMeili(ctx, in)
	}

	offset := (in.Page - 1) * in.PageSize
	items, err := s.listDocuments(ctx, in.OwnerID, in.Query, in.Status, in.PageSize, offset)
	if err != nil {
		return nil, err
	}
	total, err := s.countDocuments(ctx, in.OwnerID, in.Status)
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

	items := make([]database.Document, 0, len(res.IDs))
	if len(res.IDs) > 0 {
		docs, err := s.documentsByIDs(ctx, res.IDs)
		if err != nil {
			return nil, err
		}
		items = docs
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
	if extraction == nil {
		return nil
	}

	return s.search.IndexDocument(ctx, search.Document{
		ID:           string(doc.ID),
		OwnerID:      doc.OwnerID,
		Name:         doc.Name,
		OriginalName: doc.OriginalName,
		MimeType:     doc.MimeType,
		Content:      extraction.Text,
		Status:       doc.Status,
		CreatedAt:    doc.CreatedAt,
		UpdatedAt:    time.Now().UTC(),
	})
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

func (s *SearchService) listDocuments(ctx context.Context, ownerID, query, status string, limit, offset int) ([]database.Document, error) {
	builder := surrealgoorm.Query[database.Document](s.client, "documents").WhereEq("owner_id", ownerID).OrderBy("created_at")
	if query != "" {
		builder = builder.Where("name", "CONTAINS", query)
	}
	if status != "" {
		builder = builder.WhereEq("status", status)
	}
	builder = builder.Limit(limit)

	sql, vars := builder.ToBuildSQL()
	if offset > 0 {
		sql += " START $offset"
		vars["offset"] = offset
	}

	results, err := surrealdb.Query[[]database.Document](ctx, s.client, sql, vars)
	if err != nil {
		return nil, err
	}
	if results == nil || len(*results) == 0 {
		return []database.Document{}, nil
	}
	return (*results)[0].Result, nil
}

func (s *SearchService) countDocuments(ctx context.Context, ownerID, status string) (int64, error) {
	sql := "SELECT count() AS total FROM documents WHERE owner_id = $owner"
	vars := map[string]any{"owner": ownerID}
	if status != "" {
		sql += " AND status = $status"
		vars["status"] = status
	}
	sql += " GROUP ALL"

	results, err := surrealdb.Query[[]map[string]any](ctx, s.client, sql, vars)
	if err != nil {
		return 0, err
	}
	if results == nil || len(*results) == 0 || len((*results)[0].Result) == 0 {
		return 0, nil
	}
	if total, ok := (*results)[0].Result[0]["total"].(float64); ok {
		return int64(total), nil
	}
	return 0, nil
}

func (s *SearchService) documentsByIDs(ctx context.Context, ids []string) ([]database.Document, error) {
	if len(ids) == 0 {
		return []database.Document{}, nil
	}

	results, err := surrealdb.Query[[]database.Document](ctx, s.client, "SELECT * FROM documents WHERE id IN $ids", map[string]any{"ids": ids})
	if err != nil {
		return nil, err
	}
	if results == nil || len(*results) == 0 {
		return []database.Document{}, nil
	}

	docs := (*results)[0].Result
	byID := make(map[string]database.Document, len(docs))
	for _, d := range docs {
		byID[string(d.ID)] = d
	}

	ordered := make([]database.Document, 0, len(ids))
	for _, id := range ids {
		if d, ok := byID[id]; ok {
			ordered = append(ordered, d)
		}
	}
	return ordered, nil
}
