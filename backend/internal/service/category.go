package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	surrealgoorm "github.com/kemboi22/surreal-go-orm"
	"github.com/surrealdb/surrealdb.go"

	"github.com/xmtrxi/edms/internal/database"
	"github.com/xmtrxi/edms/internal/dto"
)

var (
	ErrCategoryNotFound = errors.New("category not found")
	ErrCategoryHasItems = errors.New("category has subcategories or documents")
	ErrInvalidParent    = errors.New("invalid parent category")
)

type CategoryService struct {
	DB *surrealdb.DB
}

func NewCategoryService(db *surrealdb.DB) *CategoryService {
	return &CategoryService{DB: db}
}

type CategoryInput struct {
	Name        string
	Description string
	ParentID    string
	Color       string
}

func (s *CategoryService) List(ctx context.Context, ownerID string) ([]dto.CategoryResponse, error) {
	results, err := surrealdb.Query[[]database.Category](ctx, s.DB,
		"SELECT * FROM categories WHERE org_id = $org ORDER BY name",
		map[string]any{"org": orgOf(ctx)},
	)
	if err != nil {
		return nil, err
	}
	categories := []database.Category{}
	if results != nil && len(*results) > 0 {
		categories = (*results)[0].Result
	}
	counts, err := s.documentCounts(ctx, categories)
	if err != nil {
		return nil, err
	}
	response := make([]dto.CategoryResponse, 0, len(categories))
	for i := range categories {
		response = append(response, dto.NewCategoryResponse(&categories[i], counts[string(categories[i].ID)]))
	}
	return response, nil
}

func (s *CategoryService) Create(ctx context.Context, ownerID string, in CategoryInput) (*dto.CategoryResponse, error) {
	in.Name = strings.TrimSpace(in.Name)
	if in.Name == "" {
		return nil, fmt.Errorf("category name is required")
	}
	if err := s.validateParent(ctx, ownerID, "", in.ParentID); err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	category, err := surrealdb.Create[database.Category](ctx, s.DB, "categories", &database.Category{
		OrgID:       orgOf(ctx),
		OwnerID:     ownerID,
		Name:        in.Name,
		Description: strings.TrimSpace(in.Description),
		ParentID:    in.ParentID,
		Color:       strings.TrimSpace(in.Color),
		CreatedAt:   now,
		UpdatedAt:   now,
	})
	if err != nil {
		return nil, err
	}
	response := dto.NewCategoryResponse(category, 0)
	return &response, nil
}

func (s *CategoryService) Update(ctx context.Context, ownerID, id string, in CategoryInput) (*dto.CategoryResponse, error) {
	category, err := s.get(ctx, ownerID, id)
	if err != nil {
		return nil, err
	}
	in.Name = strings.TrimSpace(in.Name)
	if in.Name == "" {
		return nil, fmt.Errorf("category name is required")
	}
	if in.ParentID != "" {
		children, err := surrealgoorm.Query[database.Category](s.DB, "categories").WhereEq("parent_id", id).First(ctx)
		if err != nil {
			return nil, err
		}
		if children != nil {
			return nil, ErrInvalidParent
		}
	}
	if err := s.validateParent(ctx, ownerID, id, in.ParentID); err != nil {
		return nil, err
	}
	rid, err := database.ParseRecordID(id)
	if err != nil {
		return nil, err
	}
	updated, err := surrealdb.Merge[database.Category](ctx, s.DB, rid, map[string]any{
		"name":        in.Name,
		"description": strings.TrimSpace(in.Description),
		"parent_id":   in.ParentID,
		"color":       strings.TrimSpace(in.Color),
		"updated_at":  time.Now().UTC(),
	})
	if err != nil {
		return nil, err
	}
	if updated == nil {
		return nil, ErrCategoryNotFound
	}
	count, err := s.categoryDocumentCount(ctx, string(category.ID))
	if err != nil {
		return nil, err
	}
	response := dto.NewCategoryResponse(updated, count)
	return &response, nil
}

func (s *CategoryService) Delete(ctx context.Context, ownerID, id string) error {
	if _, err := s.get(ctx, ownerID, id); err != nil {
		return err
	}
	children, err := surrealgoorm.Query[database.Category](s.DB, "categories").WhereEq("parent_id", id).First(ctx)
	if err != nil {
		return err
	}
	count, err := s.categoryDocumentCount(ctx, id)
	if err != nil {
		return err
	}
	if children != nil || count > 0 {
		return ErrCategoryHasItems
	}
	rid, err := database.ParseRecordID(id)
	if err != nil {
		return err
	}
	_, err = surrealdb.Delete[database.Category](ctx, s.DB, rid)
	return err
}

func (s *CategoryService) Get(ctx context.Context, ownerID, id string) (*database.Category, error) {
	return s.get(ctx, ownerID, id)
}

// SetDocumentCategory replaces any existing category assignment for a document.
// An empty categoryID clears the assignment.
func (s *CategoryService) SetDocumentCategory(ctx context.Context, ownerID, documentID, categoryID string) error {
	if categoryID != "" {
		if _, err := s.get(ctx, ownerID, categoryID); err != nil {
			return err
		}
	}
	if err := s.RemoveDocument(ctx, documentID); err != nil {
		return err
	}
	if categoryID == "" {
		return nil
	}
	_, err := surrealdb.Create[database.DocumentCategory](ctx, s.DB, "document_categories", &database.DocumentCategory{
		DocumentID: documentID,
		CategoryID: categoryID,
		CreatedAt:  time.Now().UTC(),
	})
	return err
}

// RemoveDocument clears every category assignment for a document. Used when a
// document is deleted or reassigned.
func (s *CategoryService) RemoveDocument(ctx context.Context, documentID string) error {
	_, err := surrealdb.Query[[]any](ctx, s.DB,
		"DELETE FROM document_categories WHERE document_id = $document",
		map[string]any{"document": documentID},
	)
	return err
}

// CategoriesForDocuments returns the category assigned to each of the given
// documents, keyed by document id. Documents without a category are absent.
func (s *CategoryService) CategoriesForDocuments(ctx context.Context, documentIDs []string) (map[string]dto.CategoryResponse, error) {
	result := map[string]dto.CategoryResponse{}
	if len(documentIDs) == 0 {
		return result, nil
	}
	links, err := surrealdb.Query[[]database.DocumentCategory](ctx, s.DB,
		"SELECT * FROM document_categories WHERE document_id IN $documents",
		map[string]any{"documents": documentIDs},
	)
	if err != nil {
		return nil, err
	}
	if links == nil || len(*links) == 0 {
		return result, nil
	}

	cache := map[string]*database.Category{}
	for _, link := range (*links)[0].Result {
		if _, seen := result[link.DocumentID]; seen {
			continue
		}
		category, ok := cache[link.CategoryID]
		if !ok {
			rid, err := database.ParseRecordID(link.CategoryID)
			if err != nil {
				continue
			}
			category, err = surrealdb.Select[database.Category](ctx, s.DB, rid)
			if err != nil {
				return nil, err
			}
			cache[link.CategoryID] = category
		}
		if category == nil {
			continue
		}
		result[link.DocumentID] = dto.NewCategoryResponse(category, 0)
	}
	return result, nil
}

// DocumentIDs returns the ids of documents assigned to a category or to any of
// its subcategories.
func (s *CategoryService) DocumentIDs(ctx context.Context, ownerID, categoryID string) ([]string, error) {
	if _, err := s.get(ctx, ownerID, categoryID); err != nil {
		return nil, err
	}
	ids := []string{categoryID}

	children, err := surrealdb.Query[[]database.Category](ctx, s.DB,
		"SELECT * FROM categories WHERE org_id = $org AND parent_id = $parent",
		map[string]any{"org": orgOf(ctx), "parent": categoryID},
	)
	if err != nil {
		return nil, err
	}
	if children != nil && len(*children) > 0 {
		for _, child := range (*children)[0].Result {
			ids = append(ids, string(child.ID))
		}
	}

	links, err := surrealdb.Query[[]database.DocumentCategory](ctx, s.DB,
		"SELECT * FROM document_categories WHERE category_id IN $categories",
		map[string]any{"categories": ids},
	)
	if err != nil {
		return nil, err
	}
	documents := []string{}
	if links == nil || len(*links) == 0 {
		return documents, nil
	}
	seen := map[string]bool{}
	for _, link := range (*links)[0].Result {
		if seen[link.DocumentID] {
			continue
		}
		seen[link.DocumentID] = true
		documents = append(documents, link.DocumentID)
	}
	return documents, nil
}

func (s *CategoryService) get(ctx context.Context, ownerID, id string) (*database.Category, error) {
	rid, err := database.ParseRecordID(id)
	if err != nil {
		return nil, ErrCategoryNotFound
	}
	category, err := surrealdb.Select[database.Category](ctx, s.DB, rid)
	if err != nil {
		return nil, err
	}
	// Categories are the organization's shared taxonomy.
	if category == nil || category.ID == "" || category.OrgID != orgOf(ctx) {
		return nil, ErrCategoryNotFound
	}
	return category, nil
}

func (s *CategoryService) validateParent(ctx context.Context, ownerID, categoryID, parentID string) error {
	if parentID == "" {
		return nil
	}
	if parentID == categoryID {
		return ErrInvalidParent
	}
	parent, err := s.get(ctx, ownerID, parentID)
	if err != nil {
		return ErrInvalidParent
	}
	// Keep the hierarchy at two levels: categories and subcategories.
	if parent.ParentID != "" {
		return ErrInvalidParent
	}
	return nil
}

func (s *CategoryService) documentCounts(ctx context.Context, categories []database.Category) (map[string]int64, error) {
	counts := make(map[string]int64, len(categories))
	for _, category := range categories {
		count, err := s.categoryDocumentCount(ctx, string(category.ID))
		if err != nil {
			return nil, err
		}
		counts[string(category.ID)] = count
	}
	return counts, nil
}

func (s *CategoryService) categoryDocumentCount(ctx context.Context, categoryID string) (int64, error) {
	results, err := surrealdb.Query[[]map[string]any](ctx, s.DB,
		"SELECT count() AS total FROM document_categories WHERE category_id = $category GROUP ALL",
		map[string]any{"category": categoryID},
	)
	if err != nil || results == nil || len(*results) == 0 || len((*results)[0].Result) == 0 {
		return 0, err
	}
	return numericCount((*results)[0].Result[0]["total"]), nil
}

func numericCount(value any) int64 {
	switch count := value.(type) {
	case int:
		return int64(count)
	case int64:
		return count
	case uint64:
		return int64(count)
	case float64:
		return int64(count)
	}
	return 0
}
