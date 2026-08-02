package search

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/meilisearch/meilisearch-go"

	"github.com/xmtrxi/edms/internal/config"
)

type Client struct {
	inner meilisearch.ServiceManager
	index string
}

func New(cfg config.SearchConfig) *Client {
	return &Client{
		inner: meilisearch.New(cfg.URL, meilisearch.WithAPIKey(cfg.APIKey)),
		index: cfg.Index,
	}
}

func (c *Client) Ping(ctx context.Context) error {
	if c == nil || c.inner == nil {
		return fmt.Errorf("search is not configured")
	}
	_, err := c.inner.HealthWithContext(ctx)
	return err
}

type Document struct {
	ID           string    `json:"id"`
	OwnerID      string    `json:"owner_id"`
	Name         string    `json:"name"`
	OriginalName string    `json:"original_name"`
	MimeType     string    `json:"mime_type"`
	Content      string    `json:"content"`
	Status       string    `json:"status"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

func (c *Client) IndexDocument(ctx context.Context, doc Document) error {
	if c == nil || c.inner == nil {
		return nil
	}
	pk := "id"
	_, err := c.inner.Index(c.index).AddDocumentsWithContext(ctx, []Document{doc}, &meilisearch.DocumentOptions{PrimaryKey: &pk})
	return err
}

func (c *Client) DeleteDocument(ctx context.Context, id string) error {
	if c == nil || c.inner == nil {
		return nil
	}
	_, err := c.inner.Index(c.index).DeleteDocumentWithContext(ctx, id, nil)
	return err
}

type SearchInput struct {
	Query    string
	OwnerID  string
	Status   string
	Page     int
	PageSize int
}

type SearchResult struct {
	IDs       []string
	TotalHits int64
	Page      int
	PageSize  int
}

func (c *Client) Search(ctx context.Context, in SearchInput) (*SearchResult, error) {
	if c == nil || c.inner == nil {
		return nil, fmt.Errorf("search is not configured")
	}
	if in.PageSize <= 0 {
		in.PageSize = 50
	}
	if in.Page <= 0 {
		in.Page = 1
	}

	filters := make([]string, 0, 3)
	if in.OwnerID != "" {
		filters = append(filters, fmt.Sprintf("owner_id = %q", in.OwnerID))
	}
	if in.Status != "" {
		filters = append(filters, fmt.Sprintf("status = %q", in.Status))
	}

	req := &meilisearch.SearchRequest{
		Offset: int64((in.Page - 1) * in.PageSize),
		Limit:  int64(in.PageSize),
	}
	if len(filters) > 0 {
		req.Filter = strings.Join(filters, " AND ")
	}

	res, err := c.inner.Index(c.index).SearchWithContext(ctx, in.Query, req)
	if err != nil {
		return nil, err
	}

	ids := make([]string, 0, len(res.Hits))
	for _, hit := range res.Hits {
		var doc Document
		if err := hit.DecodeInto(&doc); err != nil {
			continue
		}
		ids = append(ids, doc.ID)
	}

	return &SearchResult{
		IDs:       ids,
		TotalHits: res.TotalHits,
		Page:      in.Page,
		PageSize:  in.PageSize,
	}, nil
}
