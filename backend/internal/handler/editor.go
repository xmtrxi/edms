package handler

import (
	"errors"
	"io"
	"net/http"
	"net/url"

	"github.com/labstack/echo/v5"

	"github.com/xmtrxi/edms/internal/dto"
	"github.com/xmtrxi/edms/internal/middleware"
	"github.com/xmtrxi/edms/internal/service"
)

// EditorHandler opens documents in ONLYOFFICE and receives its saves.
type EditorHandler struct {
	auth   *service.AuthService
	editor *service.EditorService
}

func NewEditorHandler(deps Deps) *EditorHandler {
	docs := service.NewDocumentService(deps.Cfg, deps.Client(), deps.Storage, deps.Queue, deps.Search, deps.Logger)
	return &EditorHandler{
		auth:   service.NewAuthService(deps.Cfg, deps.Client(), deps.Logger),
		editor: service.NewEditorService(deps.Cfg.Editor, deps.Client(), deps.Storage, docs, deps.Logger),
	}
}

func (h *EditorHandler) RegisterRoutes(api *echo.Group) {
	api.GET("/documents/:id/editor", h.open, middleware.RequireAuth(h.auth))
	// ONLYOFFICE calls these with signed, expiring tokens instead of a session.
	api.GET("/editor/files/:token", h.file)
	api.POST("/editor/callback/:token", h.callback)
}

func (h *EditorHandler) open(c *echo.Context) error {
	session, err := h.editor.Open(c.Request().Context(), actor(c), c.Param("id"), c.QueryParam("mode") == "view")
	switch {
	case errors.Is(err, service.ErrEditorDisabled):
		return respondError(c, http.StatusServiceUnavailable, err.Error())
	case errors.Is(err, service.ErrEditorUnsupported):
		return respondError(c, http.StatusUnsupportedMediaType, err.Error())
	case errors.Is(err, service.ErrForbidden):
		return respondError(c, http.StatusForbidden, "you can’t open this document")
	case err != nil:
		return respondError(c, http.StatusNotFound, "document not found")
	}
	return respondJSON(c, http.StatusOK, dto.OK(session))
}

func (h *EditorHandler) file(c *echo.Context) error {
	body, name, err := h.editor.File(c.Request().Context(), c.Param("token"))
	if err != nil {
		return respondError(c, http.StatusForbidden, "invalid or expired link")
	}
	defer body.Close()
	c.Response().Header().Set(echo.HeaderContentDisposition, "attachment; filename*=UTF-8''"+url.PathEscape(name))
	c.Response().Header().Set(echo.HeaderContentType, "application/octet-stream")
	c.Response().WriteHeader(http.StatusOK)
	_, err = io.Copy(c.Response(), body)
	return err
}

// callback answers in ONLYOFFICE's format: {"error": 0} means handled. Any
// other value makes ONLYOFFICE keep the changes and retry.
func (h *EditorHandler) callback(c *echo.Context) error {
	body, err := io.ReadAll(io.LimitReader(c.Request().Body, 1<<20))
	if err != nil {
		return c.JSON(http.StatusOK, map[string]int{"error": 1})
	}
	if err := h.editor.Callback(c.Request().Context(), c.Param("token"), body, c.Request().Header.Get(echo.HeaderAuthorization)); err != nil {
		c.Logger().Warn("editor callback failed", "error", err)
		return c.JSON(http.StatusOK, map[string]int{"error": 1})
	}
	return c.JSON(http.StatusOK, map[string]int{"error": 0})
}
