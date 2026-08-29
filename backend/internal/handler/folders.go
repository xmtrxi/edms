package handler

import (
	"errors"
	"net/http"

	"github.com/labstack/echo/v5"

	"github.com/xmtrxi/edms/internal/dto"
	"github.com/xmtrxi/edms/internal/middleware"
	"github.com/xmtrxi/edms/internal/service"
)

type FoldersHandler struct {
	folders *service.FolderService
	auth    *service.AuthService
}

func NewFoldersHandler(deps Deps) *FoldersHandler {
	return &FoldersHandler{folders: service.NewFolderService(deps.Client()), auth: service.NewAuthService(deps.Cfg, deps.Client(), deps.Logger)}
}

func (h *FoldersHandler) RegisterRoutes(api *echo.Group) {
	g := api.Group("/folders", middleware.RequireAuth(h.auth))
	g.GET("", h.list)
	g.POST("", h.create)
	g.GET("/:id", h.get)
	g.PUT("/:id", h.update)
	g.DELETE("/:id", h.delete)
	g.GET("/:id/grants", h.listGrants)
	g.PUT("/:id/grants", h.upsertGrant)
	g.DELETE("/:id/grants/:grantId", h.deleteGrant)
}

type folderRequest struct {
	Name               string `json:"name"`
	ParentID           string `json:"parent_id"`
	Description        string `json:"description"`
	Color              string `json:"color"`
	InheritPermissions *bool  `json:"inherit_permissions"`
}

type folderGrantRequest struct {
	PrincipalType string   `json:"principal_type"`
	PrincipalID   string   `json:"principal_id"`
	Role          string   `json:"role"`
	Allow         []string `json:"allow"`
	Deny          []string `json:"deny"`
}

func folderInput(request folderRequest) service.FolderInput {
	inherit := true
	if request.InheritPermissions != nil {
		inherit = *request.InheritPermissions
	}
	return service.FolderInput{Name: request.Name, ParentID: request.ParentID, Description: request.Description, Color: request.Color, InheritPermissions: inherit}
}
func (h *FoldersHandler) list(c *echo.Context) error {
	user := middleware.CurrentUser(c)
	folders, err := h.folders.List(c.Request().Context(), string(user.ID))
	if err != nil {
		return respondError(c, 500, "failed to list folders")
	}
	return respondJSON(c, 200, dto.List(folders, int64(len(folders)), 1, len(folders)))
}
func (h *FoldersHandler) get(c *echo.Context) error {
	user := middleware.CurrentUser(c)
	folder, err := h.folders.Get(c.Request().Context(), string(user.ID), c.Param("id"))
	if err != nil {
		return h.handleError(c, err)
	}
	return respondJSON(c, 200, dto.OK(dto.NewManagedFolderResponse(folder, 0)))
}
func (h *FoldersHandler) create(c *echo.Context) error {
	user := middleware.CurrentUser(c)
	var request folderRequest
	if err := bind(c, &request); err != nil {
		return err
	}
	folder, err := h.folders.Create(c.Request().Context(), string(user.ID), folderInput(request))
	if err != nil {
		return h.handleError(c, err)
	}
	return respondJSON(c, http.StatusCreated, dto.Created(folder))
}
func (h *FoldersHandler) update(c *echo.Context) error {
	user := middleware.CurrentUser(c)
	var request folderRequest
	if err := bind(c, &request); err != nil {
		return err
	}
	folder, err := h.folders.Update(c.Request().Context(), string(user.ID), c.Param("id"), folderInput(request))
	if err != nil {
		return h.handleError(c, err)
	}
	return respondJSON(c, 200, dto.OK(folder))
}
func (h *FoldersHandler) delete(c *echo.Context) error {
	user := middleware.CurrentUser(c)
	if err := h.folders.Delete(c.Request().Context(), string(user.ID), c.Param("id")); err != nil {
		return h.handleError(c, err)
	}
	return c.NoContent(http.StatusNoContent)
}
func (h *FoldersHandler) listGrants(c *echo.Context) error {
	user := middleware.CurrentUser(c)
	grants, err := h.folders.ListGrants(c.Request().Context(), string(user.ID), c.Param("id"))
	if err != nil {
		return h.handleError(c, err)
	}
	return respondJSON(c, 200, dto.OK(grants))
}
func (h *FoldersHandler) upsertGrant(c *echo.Context) error {
	user := middleware.CurrentUser(c)
	var request folderGrantRequest
	if err := bind(c, &request); err != nil {
		return err
	}
	grant, err := h.folders.UpsertGrant(c.Request().Context(), string(user.ID), c.Param("id"), service.FolderGrantInput{PrincipalType: request.PrincipalType, PrincipalID: request.PrincipalID, Role: request.Role, Allow: request.Allow, Deny: request.Deny})
	if err != nil {
		return h.handleError(c, err)
	}
	return respondJSON(c, 200, dto.OK(grant))
}
func (h *FoldersHandler) deleteGrant(c *echo.Context) error {
	user := middleware.CurrentUser(c)
	if err := h.folders.DeleteGrant(c.Request().Context(), string(user.ID), c.Param("id"), c.Param("grantId")); err != nil {
		return h.handleError(c, err)
	}
	return c.NoContent(http.StatusNoContent)
}
func (h *FoldersHandler) handleError(c *echo.Context, err error) error {
	switch {
	case errors.Is(err, service.ErrFolderNotFound):
		return respondError(c, 404, "folder not found")
	case errors.Is(err, service.ErrFolderNotEmpty):
		return respondError(c, 409, "folder contains documents or subfolders")
	case errors.Is(err, service.ErrInvalidFolderParent):
		return respondError(c, 400, "select a valid parent folder")
	default:
		return respondError(c, 400, err.Error())
	}
}
