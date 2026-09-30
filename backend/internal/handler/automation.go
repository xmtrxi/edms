package handler

import (
	"errors"
	"net/http"

	"github.com/labstack/echo/v5"

	"github.com/xmtrxi/edms/internal/dto"
	"github.com/xmtrxi/edms/internal/middleware"
	"github.com/xmtrxi/edms/internal/service"
)

// AutomationHandler lets administrators manage automation rules.
type AutomationHandler struct {
	auth       *service.AuthService
	automation *service.AutomationService
}

func NewAutomationHandler(deps Deps) *AutomationHandler {
	docs := service.NewDocumentService(deps.Cfg, deps.Client(), deps.Storage, deps.Queue, deps.Search, deps.Logger)
	email := service.NewOperationsService(deps.Client(), deps.Cfg.SMTP, deps.Queue)
	return &AutomationHandler{
		auth:       service.NewAuthService(deps.Cfg, deps.Client(), deps.Logger),
		automation: service.NewAutomationService(deps.Cfg, deps.Client(), docs, email, deps.Logger),
	}
}

func (h *AutomationHandler) RegisterRoutes(api *echo.Group) {
	g := api.Group("/admin", middleware.RequireAuth(h.auth), middleware.RequireOrgAdmin())
	g.GET("/automations", h.rules)
	g.POST("/automations", h.createRule)
	g.PATCH("/automations/:id", h.updateRule)
	g.DELETE("/automations/:id", h.deleteRule)
	g.GET("/automations/:id/runs", h.runs)
}

func (h *AutomationHandler) rules(c *echo.Context) error {
	items, err := h.automation.Rules(c.Request().Context())
	if err != nil {
		return automationError(c, err)
	}
	return respondJSON(c, http.StatusOK, dto.List(items, int64(len(items)), 1, len(items)))
}

func (h *AutomationHandler) createRule(c *echo.Context) error {
	var input service.RuleInput
	if err := bind(c, &input); err != nil {
		return err
	}
	rule, err := h.automation.CreateRule(c.Request().Context(), actor(c), input)
	if err != nil {
		return automationError(c, err)
	}
	return respondJSON(c, http.StatusCreated, dto.Created(rule))
}

func (h *AutomationHandler) updateRule(c *echo.Context) error {
	var input service.RuleInput
	if err := bind(c, &input); err != nil {
		return err
	}
	rule, err := h.automation.UpdateRule(c.Request().Context(), actor(c), c.Param("id"), input)
	if err != nil {
		return automationError(c, err)
	}
	return respondJSON(c, http.StatusOK, dto.OK(rule))
}

func (h *AutomationHandler) deleteRule(c *echo.Context) error {
	if err := h.automation.DeleteRule(c.Request().Context(), actor(c), c.Param("id")); err != nil {
		return automationError(c, err)
	}
	return c.NoContent(http.StatusNoContent)
}

func (h *AutomationHandler) runs(c *echo.Context) error {
	items, err := h.automation.Runs(c.Request().Context(), c.Param("id"))
	if err != nil {
		return automationError(c, err)
	}
	return respondJSON(c, http.StatusOK, dto.List(items, int64(len(items)), 1, len(items)))
}

func automationError(c *echo.Context, err error) error {
	switch {
	case errors.Is(err, service.ErrRuleNotFound):
		return respondError(c, http.StatusNotFound, err.Error())
	case errors.Is(err, service.ErrFolderNotFound):
		return respondError(c, http.StatusBadRequest, "choose a folder in this organization")
	default:
		return respondError(c, http.StatusBadRequest, err.Error())
	}
}
