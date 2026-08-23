package handler

import (
	"errors"
	"net/http"

	"github.com/labstack/echo/v5"

	"github.com/xmtrxi/edms/internal/dto"
	"github.com/xmtrxi/edms/internal/middleware"
	"github.com/xmtrxi/edms/internal/service"
)

type CategoriesHandler struct {
	categories *service.CategoryService
	auth       *service.AuthService
}

func NewCategoriesHandler(deps Deps) *CategoriesHandler {
	return &CategoriesHandler{
		categories: service.NewCategoryService(deps.Client()),
		auth:       service.NewAuthService(deps.Cfg, deps.Client(), deps.Logger),
	}
}

func (h *CategoriesHandler) RegisterRoutes(api *echo.Group) {
	group := api.Group("/categories", middleware.RequireAuth(h.auth))
	group.GET("", h.list)
	group.POST("", h.create)
	group.PUT("/:id", h.update)
	group.DELETE("/:id", h.delete)
}

type categoryRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	ParentID    string `json:"parent_id"`
	Color       string `json:"color"`
}

func (h *CategoriesHandler) list(c *echo.Context) error {
	user := middleware.CurrentUser(c)
	categories, err := h.categories.List(c.Request().Context(), string(user.ID))
	if err != nil {
		return respondError(c, http.StatusInternalServerError, "failed to list categories")
	}
	return respondJSON(c, http.StatusOK, dto.List(categories, int64(len(categories)), 1, len(categories)))
}

func (h *CategoriesHandler) create(c *echo.Context) error {
	user := middleware.CurrentUser(c)
	var request categoryRequest
	if err := bind(c, &request); err != nil {
		return err
	}
	category, err := h.categories.Create(c.Request().Context(), string(user.ID), categoryInput(request))
	if err != nil {
		return h.handleError(c, err)
	}
	return respondJSON(c, http.StatusCreated, dto.Created(category))
}

func (h *CategoriesHandler) update(c *echo.Context) error {
	user := middleware.CurrentUser(c)
	var request categoryRequest
	if err := bind(c, &request); err != nil {
		return err
	}
	category, err := h.categories.Update(c.Request().Context(), string(user.ID), c.Param("id"), categoryInput(request))
	if err != nil {
		return h.handleError(c, err)
	}
	return respondJSON(c, http.StatusOK, dto.OK(category))
}

func (h *CategoriesHandler) delete(c *echo.Context) error {
	user := middleware.CurrentUser(c)
	if err := h.categories.Delete(c.Request().Context(), string(user.ID), c.Param("id")); err != nil {
		return h.handleError(c, err)
	}
	return c.NoContent(http.StatusNoContent)
}

func categoryInput(request categoryRequest) service.CategoryInput {
	return service.CategoryInput{
		Name:        request.Name,
		Description: request.Description,
		ParentID:    request.ParentID,
		Color:       request.Color,
	}
}

func (h *CategoriesHandler) handleError(c *echo.Context, err error) error {
	switch {
	case errors.Is(err, service.ErrCategoryNotFound):
		return respondError(c, http.StatusNotFound, "category not found")
	case errors.Is(err, service.ErrCategoryHasItems):
		return respondError(c, http.StatusConflict, "remove subcategories and document assignments before deleting this category")
	case errors.Is(err, service.ErrInvalidParent):
		return respondError(c, http.StatusBadRequest, "select a valid top-level parent category")
	default:
		return respondError(c, http.StatusBadRequest, err.Error())
	}
}
