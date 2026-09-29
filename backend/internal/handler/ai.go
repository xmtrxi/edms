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
	auth *service.AuthService
	ai   *service.AIService
}

func NewAIHandler(deps Deps) *AIHandler {
	return &AIHandler{auth: service.NewAuthService(deps.Cfg, deps.Client(), deps.Logger), ai: service.NewAIService(deps.Client(), deps.Cfg.AI)}
}
func (h *AIHandler) RegisterRoutes(api *echo.Group) {
	g := api.Group("/ai", middleware.RequireAuth(h.auth))
	g.POST("/classify", h.run("classify"))
	g.POST("/ask", h.run("ask"))
	g.POST("/summarize", h.run("summarize"))
	g.POST("/extract", h.run("extract"))
	g.GET("/jobs/:id", h.job)
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
