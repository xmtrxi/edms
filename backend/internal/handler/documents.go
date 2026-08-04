package handler

import (
	"errors"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/labstack/echo/v5"

	"github.com/xmtrxi/edms/internal/config"
	"github.com/xmtrxi/edms/internal/dto"
	"github.com/xmtrxi/edms/internal/middleware"
	"github.com/xmtrxi/edms/internal/service"
)

type DocumentsHandler struct {
	cfg    config.Config
	docs   *service.DocumentService
	auth   *service.AuthService
	search *service.SearchService
}

func NewDocumentsHandler(deps Deps) *DocumentsHandler {
	return &DocumentsHandler{
		cfg:    deps.Cfg,
		docs:   service.NewDocumentService(deps.Cfg, deps.Client(), deps.Storage, deps.Queue, deps.Search, deps.Logger),
		auth:   service.NewAuthService(deps.Cfg, deps.Client(), deps.Logger),
		search: service.NewSearchService(deps.Client(), deps.Search, deps.Logger),
	}
}

func (h *DocumentsHandler) RegisterRoutes(api *echo.Group) {
	g := api.Group("/documents", middleware.RequireAuth(h.auth))
	g.GET("", h.list)
	g.POST("", h.upload)
	g.GET("/:id", h.get)
	g.GET("/:id/ocr", h.ocr)
	g.POST("/:id/redo-ocr", h.redoOCR)
	g.GET("/:id/download", h.download)
	g.DELETE("/:id", h.delete)
	if h.cfg.Signing.Enabled {
		g.POST("/:id/sign", h.sign)
	}
}

func (h *DocumentsHandler) list(c *echo.Context) error {
	user := middleware.CurrentUser(c)
	page, pageSize := parsePageParams(c)

	result, err := h.search.Search(c.Request().Context(), service.SearchInput{
		Query:    c.QueryParam("q"),
		OwnerID:  string(user.ID),
		Status:   c.QueryParam("status"),
		Page:     page,
		PageSize: pageSize,
	})
	if err != nil {
		return respondError(c, http.StatusInternalServerError, "failed to list documents")
	}

	items := make([]dto.DocumentResponse, 0, len(result.Items))
	for i := range result.Items {
		doc := result.Items[i]
		items = append(items, dto.NewDocumentResponse(&doc).WithOwner(user))
	}

	return respondJSON(c, http.StatusOK, dto.List(items, result.Total, result.Page, result.PageSize))
}

func (h *DocumentsHandler) upload(c *echo.Context) error {
	user := middleware.CurrentUser(c)

	file, err := c.FormFile("file")
	if err != nil {
		return respondError(c, http.StatusBadRequest, "a file is required")
	}

	src, err := file.Open()
	if err != nil {
		return respondError(c, http.StatusBadRequest, "could not read uploaded file")
	}
	defer src.Close()

	name := strings.TrimSpace(c.FormValue("name"))
	if name == "" {
		name = file.Filename
	}
	name = filepath.Base(name)
	if name == "." || name == "" {
		return respondError(c, http.StatusBadRequest, "a valid filename is required")
	}

	mime := file.Header.Get("Content-Type")
	if mime == "" {
		mime = "application/octet-stream"
	}

	doc, err := h.docs.Upload(c.Request().Context(), service.UploadInput{
		OwnerID:  string(user.ID),
		Name:     name,
		MimeType: mime,
		File:     src,
	})
	if err != nil {
		return respondError(c, http.StatusInternalServerError, "failed to upload document")
	}

	return respondJSON(c, http.StatusCreated, dto.Created(dto.NewDocumentResponse(doc).WithOwner(user)))
}

func (h *DocumentsHandler) get(c *echo.Context) error {
	user := middleware.CurrentUser(c)
	doc, extraction, err := h.docs.GetOCR(c.Request().Context(), string(user.ID), c.Param("id"))
	if err != nil {
		return h.handleError(c, err)
	}
	return respondJSON(c, http.StatusOK, dto.OK(dto.NewDocumentResponse(doc).WithOwner(user).WithOCR(extraction)))
}

func (h *DocumentsHandler) ocr(c *echo.Context) error {
	user := middleware.CurrentUser(c)
	_, extraction, err := h.docs.GetOCR(c.Request().Context(), string(user.ID), c.Param("id"))
	if err != nil {
		return h.handleError(c, err)
	}
	if extraction == nil {
		return respondError(c, http.StatusNotFound, "ocr not available for this document")
	}
	return respondJSON(c, http.StatusOK, dto.OK(dto.NewOCRResponse(extraction)))
}

func (h *DocumentsHandler) redoOCR(c *echo.Context) error {
	user := middleware.CurrentUser(c)
	doc, err := h.docs.ReOCR(c.Request().Context(), string(user.ID), c.Param("id"))
	if err != nil {
		return h.handleError(c, err)
	}
	return respondJSON(c, http.StatusAccepted, dto.OK(dto.NewDocumentResponse(doc).WithOwner(user)))
}

func (h *DocumentsHandler) download(c *echo.Context) error {
	user := middleware.CurrentUser(c)
	doc, reader, err := h.docs.Download(c.Request().Context(), string(user.ID), c.Param("id"))
	if err != nil {
		return h.handleError(c, err)
	}
	defer reader.Close()

	c.Response().Header().Set(echo.HeaderContentType, doc.MimeType)
	c.Response().Header().Set(echo.HeaderContentDisposition, "attachment; filename=\""+strings.ReplaceAll(doc.Name, "\"", "")+"\"")
	return c.Stream(http.StatusOK, doc.MimeType, reader)
}

func (h *DocumentsHandler) delete(c *echo.Context) error {
	user := middleware.CurrentUser(c)
	if err := h.docs.Delete(c.Request().Context(), string(user.ID), c.Param("id")); err != nil {
		return h.handleError(c, err)
	}
	return c.NoContent(http.StatusNoContent)
}

func (h *DocumentsHandler) sign(c *echo.Context) error {
	return respondJSON(c, http.StatusNotImplemented, dto.WithMessage(map[string]string{}, "document signing is enabled but not implemented yet"))
}

func (h *DocumentsHandler) handleError(c *echo.Context, err error) error {
	switch {
	case errors.Is(err, service.ErrDocumentNotFound):
		return respondError(c, http.StatusNotFound, "document not found")
	case errors.Is(err, service.ErrForbidden):
		return respondError(c, http.StatusForbidden, "forbidden")
	default:
		return respondError(c, http.StatusInternalServerError, "request failed")
	}
}
