package handler

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/labstack/echo/v5"

	"github.com/xmtrxi/edms/internal/dto"
	"github.com/xmtrxi/edms/internal/middleware"
	"github.com/xmtrxi/edms/internal/service"
)

type NotificationsHandler struct {
	auth          *service.AuthService
	notifications *service.NotificationService
}

func NewNotificationsHandler(deps Deps) *NotificationsHandler {
	email := service.NewOperationsService(deps.Client(), deps.Cfg.SMTP, deps.Queue)
	return &NotificationsHandler{
		auth:          service.NewAuthService(deps.Cfg, deps.Client(), deps.Logger),
		notifications: service.NewNotificationService(deps.Client(), email, deps.Cfg.SMTP, deps.Logger),
	}
}

func (h *NotificationsHandler) RegisterRoutes(api *echo.Group) {
	g := api.Group("/notifications", middleware.RequireAuth(h.auth))
	g.GET("", h.list)
	g.POST("/read-all", h.readAll)
	g.POST("/:id/read", h.read)
	g.DELETE("", h.clear)
}

func (h *NotificationsHandler) list(c *echo.Context) error {
	limit, _ := strconv.Atoi(c.QueryParam("limit"))
	page, err := h.notifications.List(c.Request().Context(), string(middleware.CurrentUser(c).ID), c.QueryParam("unread") == "true", limit)
	if err != nil {
		return respondError(c, http.StatusInternalServerError, "failed to load notifications")
	}
	return respondJSON(c, http.StatusOK, dto.OK(page))
}

func (h *NotificationsHandler) read(c *echo.Context) error {
	err := h.notifications.MarkRead(c.Request().Context(), string(middleware.CurrentUser(c).ID), c.Param("id"))
	if errors.Is(err, service.ErrNotificationNotFound) {
		return respondError(c, http.StatusNotFound, err.Error())
	}
	if err != nil {
		return respondError(c, http.StatusInternalServerError, "failed to update notification")
	}
	return c.NoContent(http.StatusNoContent)
}

func (h *NotificationsHandler) readAll(c *echo.Context) error {
	if err := h.notifications.MarkAllRead(c.Request().Context(), string(middleware.CurrentUser(c).ID)); err != nil {
		return respondError(c, http.StatusInternalServerError, "failed to update notifications")
	}
	return c.NoContent(http.StatusNoContent)
}

func (h *NotificationsHandler) clear(c *echo.Context) error {
	if err := h.notifications.Clear(c.Request().Context(), string(middleware.CurrentUser(c).ID)); err != nil {
		return respondError(c, http.StatusInternalServerError, "failed to clear notifications")
	}
	return c.NoContent(http.StatusNoContent)
}
