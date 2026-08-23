package handler

import (
	"errors"
	"net/http"

	"github.com/labstack/echo/v5"
	"github.com/xmtrxi/edms/internal/dto"
	"github.com/xmtrxi/edms/internal/middleware"
	"github.com/xmtrxi/edms/internal/service"
)

type RoutingHandler struct {
	routes *service.RoutingService
	auth   *service.AuthService
}

func NewRoutingHandler(deps Deps) *RoutingHandler {
	docs := service.NewDocumentService(deps.Cfg, deps.Client(), deps.Storage, deps.Queue, deps.Search, deps.Logger)
	return &RoutingHandler{routes: service.NewRoutingService(deps.Client(), docs), auth: service.NewAuthService(deps.Cfg, deps.Client(), deps.Logger)}
}
func (h *RoutingHandler) RegisterRoutes(api *echo.Group) {
	g := api.Group("/documents", middleware.RequireAuth(h.auth))
	g.GET("/:id/routes", h.list)
	g.POST("/:id/routes", h.create)
	g.POST("/:id/routes/:routeId/decisions", h.decide)
}
func (h *RoutingHandler) list(c *echo.Context) error {
	user := middleware.CurrentUser(c)
	routes, err := h.routes.List(c.Request().Context(), string(user.ID), c.Param("id"))
	if err != nil {
		return h.handleError(c, err)
	}
	return respondJSON(c, http.StatusOK, dto.OK(routes))
}
func (h *RoutingHandler) create(c *echo.Context) error {
	user := middleware.CurrentUser(c)
	var input service.CreateRouteInput
	if err := bind(c, &input); err != nil {
		return err
	}
	route, err := h.routes.Create(c.Request().Context(), string(user.ID), c.Param("id"), input)
	if err != nil {
		return h.handleError(c, err)
	}
	return respondJSON(c, http.StatusCreated, dto.Created(route))
}
func (h *RoutingHandler) decide(c *echo.Context) error {
	user := middleware.CurrentUser(c)
	var input service.RouteDecisionInput
	if err := bind(c, &input); err != nil {
		return err
	}
	route, err := h.routes.Decide(c.Request().Context(), string(user.ID), c.Param("id"), c.Param("routeId"), input)
	if err != nil {
		return h.handleError(c, err)
	}
	return respondJSON(c, http.StatusOK, dto.OK(route))
}
func (h *RoutingHandler) handleError(c *echo.Context, err error) error {
	switch {
	case errors.Is(err, service.ErrDocumentNotFound), errors.Is(err, service.ErrWorkflowNotFound):
		return respondError(c, http.StatusNotFound, err.Error())
	case errors.Is(err, service.ErrForbidden):
		return respondError(c, http.StatusForbidden, "forbidden")
	case errors.Is(err, service.ErrInvalidTransition), errors.Is(err, service.ErrNewVersionRequired):
		return respondError(c, http.StatusConflict, err.Error())
	default:
		return respondError(c, http.StatusBadRequest, err.Error())
	}
}
