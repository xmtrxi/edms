package handler

import (
	"errors"
	"net/http"
	"strings"

	"github.com/labstack/echo/v5"
	"github.com/xmtrxi/edms/internal/dto"
	"github.com/xmtrxi/edms/internal/middleware"
	"github.com/xmtrxi/edms/internal/service"
)

type VersionsHandler struct {
	versions *service.VersionService
	auth     *service.AuthService
}

func NewVersionsHandler(deps Deps) *VersionsHandler {
	docs := service.NewDocumentService(deps.Cfg, deps.Client(), deps.Storage, deps.Queue, deps.Search, deps.Logger)
	return &VersionsHandler{versions: service.NewVersionService(deps.Client(), deps.Storage, docs), auth: service.NewAuthService(deps.Cfg, deps.Client(), deps.Logger)}
}

func (h *VersionsHandler) RegisterRoutes(api *echo.Group) {
	g := api.Group("/documents", middleware.RequireAuth(h.auth))
	g.GET("/:id/versions", h.list)
	g.GET("/:id/versions/:versionId/download", h.download)
	g.POST("/:id/versions/:versionId/restore", h.restore)
	g.GET("/:id/contributors", h.contributors)
	g.GET("/:id/history", h.history)
}

func (h *VersionsHandler) list(c *echo.Context) error {
	user := middleware.CurrentUser(c)
	items, err := h.versions.List(c.Request().Context(), string(user.ID), c.Param("id"))
	if err != nil {
		return h.handleError(c, err)
	}
	return respondJSON(c, http.StatusOK, dto.OK(items))
}
func (h *VersionsHandler) contributors(c *echo.Context) error {
	user := middleware.CurrentUser(c)
	result, err := h.versions.Contributors(c.Request().Context(), string(user.ID), c.Param("id"))
	if err != nil {
		return h.handleError(c, err)
	}
	return respondJSON(c, http.StatusOK, dto.OK(result))
}
func (h *VersionsHandler) history(c *echo.Context) error {
	user := middleware.CurrentUser(c)
	result, err := h.versions.History(c.Request().Context(), string(user.ID), c.Param("id"))
	if err != nil {
		return h.handleError(c, err)
	}
	return respondJSON(c, http.StatusOK, dto.OK(result))
}
func (h *VersionsHandler) download(c *echo.Context) error {
	user := middleware.CurrentUser(c)
	version, reader, err := h.versions.Download(c.Request().Context(), string(user.ID), c.Param("id"), c.Param("versionId"))
	if err != nil {
		return h.handleError(c, err)
	}
	defer reader.Close()
	c.Response().Header().Set(echo.HeaderContentDisposition, "attachment; filename=\""+strings.ReplaceAll(version.FileName, "\"", "")+"\"")
	return c.Stream(http.StatusOK, version.MimeType, reader)
}
func (h *VersionsHandler) restore(c *echo.Context) error {
	user := middleware.CurrentUser(c)
	var input struct {
		ChangeSummary string `json:"change_summary"`
	}
	if err := bind(c, &input); err != nil {
		return err
	}
	version, err := h.versions.Restore(c.Request().Context(), string(user.ID), c.Param("id"), c.Param("versionId"), input.ChangeSummary)
	if err != nil {
		return h.handleError(c, err)
	}
	return respondJSON(c, http.StatusCreated, dto.Created(dto.NewDocumentVersionResponse(version)))
}
func (h *VersionsHandler) handleError(c *echo.Context, err error) error {
	switch {
	case errors.Is(err, service.ErrDocumentNotFound), errors.Is(err, service.ErrVersionNotFound):
		return respondError(c, http.StatusNotFound, err.Error())
	case errors.Is(err, service.ErrForbidden):
		return respondError(c, http.StatusForbidden, "forbidden")
	default:
		return respondError(c, http.StatusInternalServerError, "version request failed")
	}
}
