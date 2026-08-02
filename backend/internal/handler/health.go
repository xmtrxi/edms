package handler

import (
	"net/http"

	"github.com/labstack/echo/v5"
)

type HealthHandler struct {
	deps Deps
}

func NewHealthHandler(deps Deps) *HealthHandler {
	return &HealthHandler{deps: deps}
}

func (h *HealthHandler) RegisterRoutes(e *echo.Echo, api *echo.Group) {
	e.GET("/health", h.health)
	api.GET("/health", h.health)
}

func (h *HealthHandler) health(c *echo.Context) error {
	checks := map[string]bool{}
	if h.deps.DB != nil {
		checks["database"] = h.deps.DB.Health(c.Request().Context()) == nil
	}
	if h.deps.Cache != nil {
		checks["cache"] = h.deps.Cache.Ping(c.Request().Context()) == nil
	}
	if h.deps.Storage != nil {
		checks["storage"] = h.deps.Storage.HealthCheck(c.Request().Context()) == nil
	}
	if h.deps.Search != nil {
		checks["search"] = h.deps.Search.Ping(c.Request().Context()) == nil
	}

	return c.JSON(http.StatusOK, map[string]any{
		"status": "ok",
		"env":    h.deps.Cfg.App.Env,
		"features": map[string]bool{
			"signing": h.deps.Cfg.Signing.Enabled,
		},
		"checks": checks,
	})
}
