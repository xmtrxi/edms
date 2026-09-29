package handler

import (
	"errors"
	"net/http"
	"time"

	"github.com/labstack/echo/v5"
	"github.com/xmtrxi/edms/internal/dto"
	"github.com/xmtrxi/edms/internal/middleware"
	"github.com/xmtrxi/edms/internal/service"
)

type WorkspaceHandler struct {
	auth      *service.AuthService
	authz     *service.AuthorizationService
	workspace *service.WorkspaceService
}

func NewWorkspaceHandler(deps Deps) *WorkspaceHandler {
	authz, err := service.NewAuthorizationService(deps.Client())
	if err != nil {
		panic(err)
	}
	return &WorkspaceHandler{auth: service.NewAuthService(deps.Cfg, deps.Client(), deps.Logger), authz: authz, workspace: service.NewWorkspaceService(deps.Client(), deps.Cfg, deps.Storage)}
}
func (h *WorkspaceHandler) RegisterRoutes(api *echo.Group) {
	g := api.Group("", middleware.RequireAuth(h.auth))
	g.GET("/dashboard/overview", h.dashboard)
	g.GET("/favorites", h.favorites)
	g.PUT("/documents/:id/favorite", h.favorite)
	g.DELETE("/documents/:id/favorite", h.unfavorite)
	g.GET("/trash", h.trash)
	g.POST("/documents/:id/trash", h.moveToTrash)
	g.POST("/documents/:id/restore", h.restore)
	g.DELETE("/documents/:id/permanent", h.permanentlyDelete)
	g.GET("/shared", h.shared)
	g.GET("/documents/:id/shares", h.documentShares)
	g.POST("/documents/:id/shares", h.createShare)
	g.DELETE("/documents/:id/shares/:share_id", h.revokeShare)
	g.GET("/workflows", h.workflows)
	g.GET("/settings", h.settings)
	g.PUT("/settings", h.saveSettings)
	g.GET("/integrations/storage", h.storageStatus)
	g.GET("/privacy/export", h.privacyExport)
	g.POST("/privacy/deletion-requests", h.deletionRequest)
	g.GET("/privacy/legal-holds", h.legalHolds, middleware.RequirePermission(h.authz, "roles", "manage"))
	g.POST("/privacy/legal-holds", h.createLegalHold, middleware.RequirePermission(h.authz, "roles", "manage"))
	g.POST("/privacy/legal-holds/:id/release", h.releaseLegalHold, middleware.RequirePermission(h.authz, "roles", "manage"))
}
func (h *WorkspaceHandler) permanentlyDelete(c *echo.Context) error {
	err := h.workspace.PermanentlyDelete(c.Request().Context(), actor(c), c.Param("id"))
	if errors.Is(err, service.ErrLegalHold) {
		return respondError(c, http.StatusConflict, err.Error())
	}
	if err != nil {
		return respondError(c, http.StatusBadRequest, err.Error())
	}
	return c.NoContent(http.StatusNoContent)
}
func actor(c *echo.Context) string { return string(middleware.CurrentUser(c).ID) }
func (h *WorkspaceHandler) dashboard(c *echo.Context) error {
	item, err := h.workspace.Dashboard(c.Request().Context(), actor(c))
	if err != nil {
		return respondError(c, 500, "failed to load dashboard")
	}
	return respondJSON(c, 200, dto.OK(item))
}
func (h *WorkspaceHandler) favorites(c *echo.Context) error {
	items, err := h.workspace.Favorites(c.Request().Context(), actor(c))
	if err != nil {
		return respondError(c, 500, "failed to list favorites")
	}
	return respondJSON(c, 200, dto.List(items, int64(len(items)), 1, len(items)))
}
func (h *WorkspaceHandler) favorite(c *echo.Context) error {
	if err := h.workspace.SetFavorite(c.Request().Context(), actor(c), c.Param("id"), true); err != nil {
		return respondError(c, 400, err.Error())
	}
	return c.NoContent(204)
}
func (h *WorkspaceHandler) unfavorite(c *echo.Context) error {
	if err := h.workspace.SetFavorite(c.Request().Context(), actor(c), c.Param("id"), false); err != nil {
		return respondError(c, 400, err.Error())
	}
	return c.NoContent(204)
}
func (h *WorkspaceHandler) trash(c *echo.Context) error {
	items, err := h.workspace.Trash(c.Request().Context(), actor(c))
	if err != nil {
		return respondError(c, 500, "failed to list trash")
	}
	return respondJSON(c, 200, dto.List(items, int64(len(items)), 1, len(items)))
}
func (h *WorkspaceHandler) moveToTrash(c *echo.Context) error {
	if err := h.workspace.MoveToTrash(c.Request().Context(), actor(c), c.Param("id")); err != nil {
		return respondError(c, 400, err.Error())
	}
	return c.NoContent(204)
}
func (h *WorkspaceHandler) restore(c *echo.Context) error {
	if err := h.workspace.Restore(c.Request().Context(), actor(c), c.Param("id")); err != nil {
		return respondError(c, 400, err.Error())
	}
	return c.NoContent(204)
}
func (h *WorkspaceHandler) shared(c *echo.Context) error {
	items, err := h.workspace.Shared(c.Request().Context(), actor(c))
	if err != nil {
		return respondError(c, 500, "failed to list shared files")
	}
	return respondJSON(c, 200, dto.List(items, int64(len(items)), 1, len(items)))
}
func (h *WorkspaceHandler) documentShares(c *echo.Context) error {
	items, err := h.workspace.DocumentShares(c.Request().Context(), actor(c), c.Param("id"))
	if err != nil {
		return respondError(c, 400, err.Error())
	}
	return respondJSON(c, 200, dto.OK(items))
}
func (h *WorkspaceHandler) createShare(c *echo.Context) error {
	var input struct {
		Email      string    `json:"email"`
		Permission string    `json:"permission"`
		ExpiresAt  time.Time `json:"expires_at"`
	}
	if err := bind(c, &input); err != nil {
		return err
	}
	item, token, err := h.workspace.CreateShare(c.Request().Context(), actor(c), c.Param("id"), input.Email, input.Permission, input.ExpiresAt)
	if err != nil {
		return respondError(c, 400, err.Error())
	}
	return respondJSON(c, 201, dto.Created(map[string]any{"share": item, "token": token}))
}
func (h *WorkspaceHandler) revokeShare(c *echo.Context) error {
	if err := h.workspace.RevokeShare(c.Request().Context(), actor(c), c.Param("id"), c.Param("share_id")); err != nil {
		return respondError(c, 400, err.Error())
	}
	return c.NoContent(204)
}
func (h *WorkspaceHandler) workflows(c *echo.Context) error {
	items, err := h.workspace.Workflows(c.Request().Context(), actor(c))
	if err != nil {
		return respondError(c, 500, "failed to list workflows")
	}
	return respondJSON(c, 200, dto.List(items, int64(len(items)), 1, len(items)))
}
func (h *WorkspaceHandler) settings(c *echo.Context) error {
	values, err := h.workspace.Settings(c.Request().Context(), actor(c))
	if err != nil {
		return respondError(c, 500, "failed to load settings")
	}
	return respondJSON(c, 200, dto.OK(values))
}
func (h *WorkspaceHandler) saveSettings(c *echo.Context) error {
	values := map[string]any{}
	if err := bind(c, &values); err != nil {
		return err
	}
	saved, err := h.workspace.SaveSettings(c.Request().Context(), actor(c), values)
	if err != nil {
		return respondError(c, 500, "failed to save settings")
	}
	return respondJSON(c, 200, dto.OK(saved))
}
func (h *WorkspaceHandler) storageStatus(c *echo.Context) error {
	return respondJSON(c, 200, dto.OK(h.workspace.StorageStatus(c.Request().Context())))
}
func (h *WorkspaceHandler) privacyExport(c *echo.Context) error {
	data, err := h.workspace.PrivacyExport(c.Request().Context(), actor(c))
	if err != nil {
		return respondError(c, 500, "failed to export data")
	}
	c.Response().Header().Set(echo.HeaderContentDisposition, "attachment; filename=\"my-edms-data.json\"")
	return respondJSON(c, 200, data)
}
func (h *WorkspaceHandler) deletionRequest(c *echo.Context) error {
	item, err := h.workspace.RequestDeletion(c.Request().Context(), actor(c))
	if err != nil {
		return respondError(c, 500, "failed to create deletion request")
	}
	return respondJSON(c, 202, dto.Created(item))
}
func (h *WorkspaceHandler) legalHolds(c *echo.Context) error {
	items, err := h.workspace.LegalHolds(c.Request().Context())
	if err != nil {
		return respondError(c, 500, "failed to list legal holds")
	}
	return respondJSON(c, 200, dto.OK(items))
}
func (h *WorkspaceHandler) createLegalHold(c *echo.Context) error {
	var input struct {
		Name        string   `json:"name"`
		Reason      string   `json:"reason"`
		DocumentIDs []string `json:"document_ids"`
	}
	if err := bind(c, &input); err != nil {
		return err
	}
	item, err := h.workspace.CreateLegalHold(c.Request().Context(), actor(c), input.Name, input.Reason, input.DocumentIDs)
	if err != nil {
		return respondError(c, 400, err.Error())
	}
	return respondJSON(c, http.StatusCreated, dto.Created(item))
}
func (h *WorkspaceHandler) releaseLegalHold(c *echo.Context) error {
	item, err := h.workspace.ReleaseLegalHold(c.Request().Context(), actor(c), c.Param("id"))
	if err != nil {
		return respondError(c, http.StatusNotFound, err.Error())
	}
	return respondJSON(c, http.StatusOK, dto.OK(item))
}
