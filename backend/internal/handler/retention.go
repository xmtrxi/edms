package handler

import (
	"errors"
	"net/http"

	"github.com/labstack/echo/v5"

	"github.com/xmtrxi/edms/internal/database"
	"github.com/xmtrxi/edms/internal/dto"
	"github.com/xmtrxi/edms/internal/middleware"
	"github.com/xmtrxi/edms/internal/service"
)

// RetentionHandler lets administrators manage retention policies.
type RetentionHandler struct {
	auth      *service.AuthService
	retention *service.RetentionService
}

func NewRetentionHandler(deps Deps) *RetentionHandler {
	return &RetentionHandler{
		auth:      service.NewAuthService(deps.Cfg, deps.Client(), deps.Logger),
		retention: service.NewRetentionService(deps.Cfg, deps.Client(), deps.Storage, deps.Logger),
	}
}

func (h *RetentionHandler) RegisterRoutes(api *echo.Group) {
	g := api.Group("/admin/retention", middleware.RequireAuth(h.auth), middleware.RequireOrgAdmin())
	g.GET("", h.policies)
	g.POST("", h.createPolicy)
	g.PATCH("/:id", h.updatePolicy)
	g.DELETE("/:id", h.deletePolicy)
	g.GET("/:id/preview", h.preview)
	g.POST("/:id/run", h.runPolicy)
}

func (h *RetentionHandler) policies(c *echo.Context) error {
	items, err := h.retention.Policies(c.Request().Context())
	if err != nil {
		return retentionError(c, err)
	}
	return respondJSON(c, http.StatusOK, dto.List(items, int64(len(items)), 1, len(items)))
}

func (h *RetentionHandler) createPolicy(c *echo.Context) error {
	var input service.PolicyInput
	if err := bind(c, &input); err != nil {
		return err
	}
	policy, err := h.retention.Create(c.Request().Context(), actor(c), input)
	if err != nil {
		return retentionError(c, err)
	}
	return respondJSON(c, http.StatusCreated, dto.Created(policy))
}

func (h *RetentionHandler) updatePolicy(c *echo.Context) error {
	var input service.PolicyInput
	if err := bind(c, &input); err != nil {
		return err
	}
	policy, err := h.retention.Update(c.Request().Context(), actor(c), c.Param("id"), input)
	if err != nil {
		return retentionError(c, err)
	}
	return respondJSON(c, http.StatusOK, dto.OK(policy))
}

func (h *RetentionHandler) deletePolicy(c *echo.Context) error {
	if err := h.retention.Delete(c.Request().Context(), actor(c), c.Param("id")); err != nil {
		return retentionError(c, err)
	}
	return c.NoContent(http.StatusNoContent)
}

func (h *RetentionHandler) preview(c *echo.Context) error {
	preview, err := h.retention.Preview(c.Request().Context(), c.Param("id"))
	if err != nil {
		return retentionError(c, err)
	}
	return respondJSON(c, http.StatusOK, dto.OK(map[string]any{
		"count": preview.Count, "held": preview.Held, "sample": documentSummaries(preview.Sample),
	}))
}

func (h *RetentionHandler) runPolicy(c *echo.Context) error {
	count, err := h.retention.RunNow(c.Request().Context(), c.Param("id"))
	if err != nil {
		return retentionError(c, err)
	}
	return respondJSON(c, http.StatusOK, dto.OK(map[string]int{"applied": count}))
}

func retentionError(c *echo.Context, err error) error {
	switch {
	case errors.Is(err, service.ErrPolicyNotFound):
		return respondError(c, http.StatusNotFound, err.Error())
	case errors.Is(err, service.ErrFolderNotFound):
		return respondError(c, http.StatusBadRequest, "choose a folder in this organization")
	default:
		return respondError(c, http.StatusBadRequest, err.Error())
	}
}

func documentSummaries(docs []database.Document) []dto.DocumentResponse {
	items := make([]dto.DocumentResponse, 0, len(docs))
	for i := range docs {
		items = append(items, dto.NewDocumentResponse(&docs[i]))
	}
	return items
}
