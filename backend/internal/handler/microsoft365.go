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

// Microsoft365Handler opens documents stored in SharePoint in Office for the
// web and checks the edits back in as versions.
type Microsoft365Handler struct {
	auth *service.AuthService
	m365 *service.Microsoft365Service
}

func NewMicrosoft365Handler(deps Deps) *Microsoft365Handler {
	docs := service.NewDocumentService(deps.Cfg, deps.Client(), deps.Storage, deps.Queue, deps.Search, deps.Logger)
	return &Microsoft365Handler{
		auth: service.NewAuthService(deps.Cfg, deps.Client(), deps.Logger),
		m365: service.NewMicrosoft365Service(deps.Client(), deps.Storage, docs, service.NewVersionService(deps.Client(), deps.Storage, docs), deps.Logger),
	}
}

func (h *Microsoft365Handler) RegisterRoutes(api *echo.Group) {
	g := api.Group("/documents/:id/microsoft365", middleware.RequireAuth(h.auth))
	g.GET("", h.status)
	g.POST("/open", h.open)
	g.POST("/checkin", h.checkIn)
	g.DELETE("", h.discard)
}

type m365Response struct {
	Open          bool   `json:"open"`
	WebURL        string `json:"web_url,omitempty"`
	BaseVersionID string `json:"base_version_id,omitempty"`
	OpenedAt      string `json:"opened_at,omitempty"`
}

func newM365Response(session *database.M365Session) m365Response {
	if session == nil {
		return m365Response{}
	}
	return m365Response{Open: true, WebURL: session.WebURL, BaseVersionID: session.BaseVersionID, OpenedAt: session.CreatedAt.Format("2006-01-02T15:04:05Z07:00")}
}

func (h *Microsoft365Handler) status(c *echo.Context) error {
	session, err := h.m365.Session(c.Request().Context(), actor(c), c.Param("id"))
	if err != nil {
		return respondError(c, http.StatusInternalServerError, "failed to load Microsoft 365 status")
	}
	return respondJSON(c, http.StatusOK, dto.OK(newM365Response(session)))
}

func (h *Microsoft365Handler) open(c *echo.Context) error {
	session, err := h.m365.Open(c.Request().Context(), actor(c), c.Param("id"))
	if err != nil {
		return h.error(c, err)
	}
	return respondJSON(c, http.StatusOK, dto.OK(newM365Response(session)))
}

func (h *Microsoft365Handler) checkIn(c *echo.Context) error {
	var r struct {
		Done bool `json:"done"`
	}
	_ = c.Bind(&r)
	version, err := h.m365.CheckIn(c.Request().Context(), actor(c), c.Param("id"), r.Done)
	if err != nil {
		return h.error(c, err)
	}
	if version == nil {
		return respondJSON(c, http.StatusOK, dto.WithMessage(map[string]any{"changed": false}, "no changes to save"))
	}
	return respondJSON(c, http.StatusOK, dto.OK(map[string]any{"changed": true, "version": dto.NewDocumentVersionResponse(version)}))
}

func (h *Microsoft365Handler) discard(c *echo.Context) error {
	if err := h.m365.Discard(c.Request().Context(), actor(c), c.Param("id")); err != nil {
		return h.error(c, err)
	}
	return c.NoContent(http.StatusNoContent)
}

func (h *Microsoft365Handler) error(c *echo.Context, err error) error {
	if response, ok := scanRefusal(c, err); ok {
		return response
	}
	switch {
	case errors.Is(err, service.ErrM365Unavailable):
		return respondError(c, http.StatusServiceUnavailable, err.Error())
	case errors.Is(err, service.ErrM365NoSession), errors.Is(err, service.ErrDocumentNotFound), errors.Is(err, service.ErrVersionNotFound):
		return respondError(c, http.StatusNotFound, err.Error())
	case errors.Is(err, service.ErrForbidden), errors.Is(err, service.ErrAccessDenied):
		return respondError(c, http.StatusForbidden, "you can't edit this document")
	case errors.Is(err, service.ErrM365Format):
		return respondError(c, http.StatusBadRequest, err.Error())
	}
	return respondError(c, http.StatusBadGateway, "Microsoft 365 request failed: "+err.Error())
}
