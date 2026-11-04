package handler

import (
	"errors"
	"net/http"

	"github.com/labstack/echo/v5"

	"github.com/xmtrxi/edms/internal/dto"
	"github.com/xmtrxi/edms/internal/middleware"
	"github.com/xmtrxi/edms/internal/pdftools"
	"github.com/xmtrxi/edms/internal/queue"
	"github.com/xmtrxi/edms/internal/service"
)

// IntegrationsHandler shows how background work and outside services are
// doing, and lets an administrator retry what got stuck. The queue is shared
// by every organization, so on SaaS installs it is not exposed.
type IntegrationsHandler struct {
	auth      *service.AuthService
	reconcile *service.ReconcileService
	inspector *queue.Inspector
}

func NewIntegrationsHandler(deps Deps) *IntegrationsHandler {
	docs := service.NewDocumentService(deps.Cfg, deps.Client(), deps.Storage, deps.Queue, deps.Search, deps.Logger)
	var inspector *queue.Inspector
	if deps.Cfg.Queue.RedisURL != "" {
		if i, err := queue.NewInspector(deps.Cfg.Queue.RedisURL); err == nil {
			inspector = i
		}
	}
	var client *pdftools.Client
	if deps.Cfg.PDFTools.URL != "" {
		client = pdftools.New(deps.Cfg.PDFTools.URL, deps.Cfg.PDFTools.APIKey, deps.Cfg.PDFTools.Timeout)
	}
	pdf := service.NewPDFService(deps.Client(), client, docs, service.NewVersionService(deps.Client(), deps.Storage, docs),
		service.NewPreviewService(docs, deps.Storage, deps.Office), deps.Queue, deps.Logger)
	email := service.NewOperationsService(deps.Client(), deps.Cfg.SMTP, deps.Queue)
	return &IntegrationsHandler{
		auth:      service.NewAuthService(deps.Cfg, deps.Client(), deps.Logger),
		reconcile: service.NewReconcileService(deps.Client(), docs, email, pdf, deps.Storage, deps.Scanner, inspector, deps.Logger),
		inspector: inspector,
	}
}

func (h *IntegrationsHandler) RegisterRoutes(api *echo.Group) {
	g := api.Group("/admin/integrations", middleware.RequireAuth(h.auth), h.requireInstallAdmin)
	g.GET("", h.report)
	g.POST("/reconcile", h.reconcileNow)
	g.GET("/queues", h.queues)
	g.GET("/dead-tasks", h.deadTasks)
	g.POST("/dead-tasks/:queue/:id/retry", h.retryDead)
	g.DELETE("/dead-tasks/:queue/:id", h.discardDead)
}

func (h *IntegrationsHandler) requireInstallAdmin(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c *echo.Context) error {
		if h.auth.Orgs.Capabilities().IsSaaS() || !middleware.IsOrgAdmin(c) {
			return respondError(c, http.StatusForbidden, "only an administrator of a self-hosted installation can manage integrations")
		}
		return next(c)
	}
}

func (h *IntegrationsHandler) report(c *echo.Context) error {
	return respondJSON(c, http.StatusOK, dto.OK(h.reconcile.Report(c.Request().Context())))
}

func (h *IntegrationsHandler) reconcileNow(c *echo.Context) error {
	var r struct {
		RequeueDead bool `json:"requeue_dead"`
	}
	_ = c.Bind(&r)
	result, err := h.reconcile.Reconcile(c.Request().Context(), r.RequeueDead)
	if err != nil {
		return respondError(c, http.StatusInternalServerError, "failed to retry stuck work: "+err.Error())
	}
	return respondJSON(c, http.StatusOK, dto.OK(result))
}

func (h *IntegrationsHandler) queues(c *echo.Context) error {
	if h.inspector == nil {
		return respondJSON(c, http.StatusOK, dto.OK([]queue.QueueStats{}))
	}
	stats, err := h.inspector.Stats()
	if err != nil {
		return respondError(c, http.StatusServiceUnavailable, "task queue can't be reached")
	}
	return respondJSON(c, http.StatusOK, dto.OK(stats))
}

func (h *IntegrationsHandler) deadTasks(c *echo.Context) error {
	if h.inspector == nil {
		return respondJSON(c, http.StatusOK, dto.OK([]queue.DeadTask{}))
	}
	tasks, err := h.inspector.Dead(100)
	if err != nil {
		return respondError(c, http.StatusServiceUnavailable, "task queue can't be reached")
	}
	if tasks == nil {
		tasks = []queue.DeadTask{}
	}
	return respondJSON(c, http.StatusOK, dto.OK(tasks))
}

func (h *IntegrationsHandler) retryDead(c *echo.Context) error {
	return h.deadTaskAction(c, h.inspector.Requeue, "integrations.dead_task_requeued")
}

func (h *IntegrationsHandler) discardDead(c *echo.Context) error {
	return h.deadTaskAction(c, h.inspector.Discard, "integrations.dead_task_discarded")
}

func (h *IntegrationsHandler) deadTaskAction(c *echo.Context, action func(queue, id string) error, audit string) error {
	if h.inspector == nil {
		return respondError(c, http.StatusServiceUnavailable, "no task queue is configured")
	}
	err := action(c.Param("queue"), c.Param("id"))
	if errors.Is(err, queue.ErrTaskNotFound) {
		return respondError(c, http.StatusNotFound, err.Error())
	}
	if err != nil {
		return respondError(c, http.StatusInternalServerError, err.Error())
	}
	_ = service.WriteAudit(c.Request().Context(), h.auth.DB, actor(c), audit, "task", c.Param("id"), map[string]any{"queue": c.Param("queue")})
	return c.NoContent(http.StatusNoContent)
}
