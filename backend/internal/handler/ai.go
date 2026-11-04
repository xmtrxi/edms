package handler

import (
	"errors"
	"net/http"

	"github.com/labstack/echo/v5"
	applicationai "github.com/xmtrxi/edms/internal/ai"
	"github.com/xmtrxi/edms/internal/dto"
	"github.com/xmtrxi/edms/internal/middleware"
	"github.com/xmtrxi/edms/internal/service"
)

type AIHandler struct {
	auth   *service.AuthService
	ai     *service.AIService
	docs   *service.DocumentService
	labels *service.LabelService
}

func NewAIHandler(deps Deps) *AIHandler {
	search := service.NewSearchService(deps.Client(), deps.Storage, deps.Search, deps.Logger)
	return &AIHandler{
		auth:   service.NewAuthService(deps.Cfg, deps.Client(), deps.Logger),
		ai:     service.NewAIService(deps.Client(), deps.Cfg.AI).WithSearch(search),
		docs:   service.NewDocumentService(deps.Cfg, deps.Client(), deps.Storage, deps.Queue, deps.Search, deps.Logger),
		labels: service.NewLabelService(deps.Client(), deps.Storage, service.NewAIProvider(deps.Cfg.AI), deps.Logger),
	}
}
func (h *AIHandler) RegisterRoutes(api *echo.Group) {
	g := api.Group("/ai", middleware.RequireAuth(h.auth))
	g.POST("/classify", h.run("classify"))
	g.POST("/ask", h.run("ask"))
	g.POST("/summarize", h.run("summarize"))
	g.POST("/extract", h.run("extract"))
	g.GET("/jobs/:id", h.job)
	g.POST("/answer", h.answer)
	g.POST("/documents/:id/label", h.label)
}
func (h *AIHandler) run(kind string) echo.HandlerFunc {
	return func(c *echo.Context) error {
		var input service.AIInput
		if err := bind(c, &input); err != nil {
			return err
		}
		job, err := h.ai.Run(c.Request().Context(), actor(c), kind, input)
		if errors.Is(err, applicationai.ErrDisabled) {
			return respondError(c, http.StatusServiceUnavailable, err.Error())
		}
		if err != nil {
			return respondError(c, http.StatusBadRequest, err.Error())
		}
		return respondJSON(c, http.StatusOK, dto.OK(job))
	}
}
func (h *AIHandler) job(c *echo.Context) error {
	job, err := h.ai.Get(c.Request().Context(), actor(c), c.Param("id"))
	if err != nil {
		return respondError(c, 404, err.Error())
	}
	return respondJSON(c, 200, dto.OK(job))
}

// answer replies to a question from the documents the user can open, with
// citations to the passages used.
func (h *AIHandler) answer(c *echo.Context) error {
	var input service.AnswerInput
	if err := bind(c, &input); err != nil {
		return err
	}
	result, err := h.ai.Answer(c.Request().Context(), actor(c), input)
	switch {
	case errors.Is(err, applicationai.ErrDisabled):
		return respondError(c, http.StatusServiceUnavailable, "AI is not configured on this installation")
	case errors.Is(err, service.ErrForbidden):
		return respondError(c, http.StatusForbidden, "you cannot open that folder")
	case err != nil:
		return respondError(c, http.StatusBadRequest, err.Error())
	}
	return respondJSON(c, http.StatusOK, dto.OK(result))
}

// label re-runs sensitive-data detection (and AI labelling when configured)
// for one document the user can edit.
func (h *AIHandler) label(c *echo.Context) error {
	ctx := c.Request().Context()
	if _, err := h.docs.GetFor(ctx, actor(c), "edit", c.Param("id")); err != nil {
		return respondError(c, http.StatusForbidden, "you cannot edit this document")
	}
	if err := h.labels.Process(ctx, c.Param("id")); err != nil {
		return respondError(c, http.StatusBadRequest, err.Error())
	}
	doc, err := h.docs.Get(ctx, actor(c), c.Param("id"))
	if err != nil {
		return respondError(c, http.StatusInternalServerError, "labelled, but the document could not be reloaded")
	}
	return respondJSON(c, http.StatusOK, dto.OK(dto.NewDocumentResponse(doc)))
}
