package dto

import (
	"time"

	"github.com/xmtrxi/edms/internal/database"
)

type CategoryResponse struct {
	ID            string    `json:"id"`
	Name          string    `json:"name"`
	Description   string    `json:"description,omitempty"`
	ParentID      string    `json:"parent_id,omitempty"`
	Color         string    `json:"color,omitempty"`
	DocumentCount int64     `json:"document_count"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

func NewCategoryResponse(category *database.Category, documentCount int64) CategoryResponse {
	if category == nil {
		return CategoryResponse{}
	}
	return CategoryResponse{
		ID:            string(category.ID),
		Name:          category.Name,
		Description:   category.Description,
		ParentID:      category.ParentID,
		Color:         category.Color,
		DocumentCount: documentCount,
		CreatedAt:     category.CreatedAt,
		UpdatedAt:     category.UpdatedAt,
	}
}
