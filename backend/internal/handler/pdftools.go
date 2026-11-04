package handler

import (
	"errors"
	"net/http"

	"github.com/labstack/echo/v5"

	"github.com/xmtrxi/edms/internal/dto"
	"github.com/xmtrxi/edms/internal/middleware"
	"github.com/xmtrxi/edms/internal/pdftools"
	"github.com/xmtrxi/edms/internal/service"
)

type PDFToolsHandler struct {
	auth *service.AuthService
	pdf  *service.PDFService
}

func NewPDFToolsHandler(deps Deps) *PDFToolsHandler {
	docs := service.NewDocumentService(deps.Cfg, deps.Client(), deps.Storage, deps.Queue, deps.Search, deps.Logger)
	var client *pdftools.Client
	if deps.Cfg.PDFTools.URL != "" {
		client = pdftools.New(deps.Cfg.PDFTools.URL, deps.Cfg.PDFTools.APIKey, deps.Cfg.PDFTools.Timeout)
	}
	return &PDFToolsHandler{
		auth: service.NewAuthService(deps.Cfg, deps.Client(), deps.Logger),
		pdf: service.NewPDFService(deps.Client(), client, docs, service.NewVersionService(deps.Client(), deps.Storage, docs),
			service.NewPreviewService(docs, deps.Storage, deps.Office), deps.Queue, deps.Logger),
	}
}

func (h *PDFToolsHandler) RegisterRoutes(api *echo.Group) {
	g := api.Group("/pdf-tools", middleware.RequireAuth(h.auth))
	g.GET("", h.status)
	g.POST("/jobs", h.create)
	g.GET("/jobs", h.list)
	g.GET("/jobs/:id", h.get)
	g.POST("/jobs/:id/retry", h.retry)
}

func (h *PDFToolsHandler) status(c *echo.Context) error {
	return respondJSON(c, http.StatusOK, dto.OK(map[string]any{
		"enabled":    h.pdf.Enabled(),
		"operations": []string{pdftools.OpMerge, pdftools.OpCompress, pdftools.OpRotate, pdftools.OpWatermark, pdftools.OpPageNumbers},
	}))
}

func (h *PDFToolsHandler) create(c *echo.Context) error {
	var in service.PDFJobInput
	if err := bind(c, &in); err != nil {
		return err
	}
	if key := c.Request().Header.Get("Idempotency-Key"); key != "" && in.IdempotencyKey == "" {
		in.IdempotencyKey = key
	}
	job, err := h.pdf.Create(c.Request().Context(), actor(c), in)
	if err != nil {
		return h.error(c, err)
	}
	return respondJSON(c, http.StatusAccepted, dto.OK(job))
}

func (h *PDFToolsHandler) list(c *echo.Context) error {
	jobs, err := h.pdf.List(c.Request().Context(), actor(c), c.QueryParam("document_id"))
	if err != nil {
		return respondError(c, http.StatusInternalServerError, "failed to list PDF jobs")
	}
	return respondJSON(c, http.StatusOK, dto.List(jobs, int64(len(jobs)), 1, len(jobs)))
}

func (h *PDFToolsHandler) get(c *echo.Context) error {
	job, err := h.pdf.Get(c.Request().Context(), actor(c), c.Param("id"))
	if err != nil {
		return h.error(c, err)
	}
	return respondJSON(c, http.StatusOK, dto.OK(job))
}

func (h *PDFToolsHandler) retry(c *echo.Context) error {
	job, err := h.pdf.Retry(c.Request().Context(), actor(c), c.Param("id"))
	if err != nil {
		return h.error(c, err)
	}
	return respondJSON(c, http.StatusAccepted, dto.OK(job))
}

func (h *PDFToolsHandler) error(c *echo.Context, err error) error {
	switch {
	case errors.Is(err, service.ErrPDFToolsDisabled):
		return respondError(c, http.StatusServiceUnavailable, err.Error())
	case errors.Is(err, service.ErrPDFJobNotFound), errors.Is(err, service.ErrDocumentNotFound):
		return respondError(c, http.StatusNotFound, err.Error())
	case errors.Is(err, service.ErrForbidden), errors.Is(err, service.ErrAccessDenied):
		return respondError(c, http.StatusForbidden, "you don't have permission to change these documents")
	}
	return respondError(c, http.StatusBadRequest, err.Error())
}
