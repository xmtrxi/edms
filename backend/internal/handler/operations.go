package handler

import (
	"net/http"
	"time"

	"github.com/labstack/echo/v5"
	"github.com/xmtrxi/edms/internal/dto"
	"github.com/xmtrxi/edms/internal/middleware"
	"github.com/xmtrxi/edms/internal/service"
)

type OperationsHandler struct {
	auth       *service.AuthService
	authz      *service.AuthorizationService
	operations *service.OperationsService
}

func NewOperationsHandler(deps Deps) *OperationsHandler {
	authz, err := service.NewAuthorizationService(deps.Client())
	if err != nil {
		panic(err)
	}
	return &OperationsHandler{auth: service.NewAuthService(deps.Cfg, deps.Client(), deps.Logger), authz: authz, operations: service.NewOperationsService(deps.Client(), deps.Cfg.SMTP, deps.Queue)}
}
func (h *OperationsHandler) RegisterRoutes(api *echo.Group) {
	g := api.Group("/admin", middleware.RequireAuth(h.auth), middleware.RequirePermission(h.authz, "roles", "manage"))
	g.GET("/audit-events", h.audit)
	g.GET("/audit-events/export", h.auditExport)
	g.DELETE("/audit-events", h.purgeAudit)
	g.GET("/email-deliveries", h.emails)
	g.POST("/email-deliveries", h.queueEmail)
	g.POST("/email-deliveries/:id/retry", h.retryEmail)
	g.PATCH("/email-deliveries/:id/status", h.emailStatus)
}
func parseSince(value string) time.Time {
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return time.Now().UTC().AddDate(0, -3, 0)
	}
	return parsed
}
func (h *OperationsHandler) audit(c *echo.Context) error {
	items, err := h.operations.Audit(c.Request().Context(), c.QueryParam("action"), c.QueryParam("resource"), parseSince(c.QueryParam("since")))
	if err != nil {
		return respondError(c, 500, "failed to list audit events")
	}
	return respondJSON(c, 200, dto.List(items, int64(len(items)), 1, len(items)))
}
func (h *OperationsHandler) auditExport(c *echo.Context) error {
	c.Response().Header().Set(echo.HeaderContentDisposition, "attachment; filename=\"audit-events.json\"")
	return h.audit(c)
}
func (h *OperationsHandler) purgeAudit(c *echo.Context) error {
	before, err := time.Parse(time.RFC3339, c.QueryParam("before"))
	if err != nil {
		return respondError(c, 400, "valid before timestamp is required")
	}
	if err = h.operations.PurgeAudit(c.Request().Context(), before); err != nil {
		return respondError(c, 400, err.Error())
	}
	return c.NoContent(204)
}
func (h *OperationsHandler) emails(c *echo.Context) error {
	items, err := h.operations.Emails(c.Request().Context())
	if err != nil {
		return respondError(c, 500, "failed to list email deliveries")
	}
	return respondJSON(c, 200, dto.List(items, int64(len(items)), 1, len(items)))
}
func (h *OperationsHandler) queueEmail(c *echo.Context) error {
	var input struct {
		Recipient string `json:"recipient"`
		Subject   string `json:"subject"`
		Body      string `json:"body"`
	}
	if err := bind(c, &input); err != nil {
		return err
	}
	item, err := h.operations.QueueEmail(c.Request().Context(), actor(c), input.Recipient, input.Subject, input.Body)
	if err != nil {
		return respondError(c, 400, err.Error())
	}
	return respondJSON(c, http.StatusCreated, dto.Created(item))
}
func (h *OperationsHandler) retryEmail(c *echo.Context) error {
	item, err := h.operations.RetryEmail(c.Request().Context(), c.Param("id"))
	if err != nil {
		return respondError(c, 400, err.Error())
	}
	return respondJSON(c, 200, dto.OK(item))
}
func (h *OperationsHandler) emailStatus(c *echo.Context) error {
	var input struct {
		Status string `json:"status"`
	}
	if err := bind(c, &input); err != nil {
		return err
	}
	item, err := h.operations.SetEmailStatus(c.Request().Context(), c.Param("id"), input.Status)
	if err != nil {
		return respondError(c, 400, err.Error())
	}
	return respondJSON(c, 200, dto.OK(item))
}
