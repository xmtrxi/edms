package handler

import (
	"net/http"
	"strings"

	"github.com/labstack/echo/v5"

	"github.com/xmtrxi/edms/internal/dto"
	"github.com/xmtrxi/edms/internal/middleware"
	"github.com/xmtrxi/edms/internal/queue"
	"github.com/xmtrxi/edms/internal/service"
)

type SearchHandler struct {
	service *service.SearchService
	auth    *service.AuthService
	queue   *queue.Client
}

func NewSearchHandler(deps Deps) *SearchHandler {
	return &SearchHandler{
		service: service.NewSearchService(deps.Client(), deps.Search, deps.Logger),
		auth:    service.NewAuthService(deps.Cfg, deps.Client(), deps.Logger),
		queue:   deps.Queue,
	}
}

func (h *SearchHandler) RegisterRoutes(api *echo.Group) {
	g := api.Group("/search", middleware.RequireAuth(h.auth))
	g.GET("", h.search)
	g.POST("/reindex", h.reindex)
}

func (h *SearchHandler) search(c *echo.Context) error {
	user := middleware.CurrentUser(c)

	query := strings.TrimSpace(c.QueryParam("q"))
	if query == "" {
		return respondError(c, http.StatusBadRequest, "search query is required")
	}

	page, pageSize := parsePageParams(c)
	result, err := h.service.Search(c.Request().Context(), service.SearchInput{
		Query:    query,
		OwnerID:  user.ID,
		Status:   c.QueryParam("status"),
		Page:     page,
		PageSize: pageSize,
	})
	if err != nil {
		return respondError(c, http.StatusInternalServerError, "search failed")
	}

	items := make([]dto.DocumentResponse, 0, len(result.Items))
	for i := range result.Items {
		doc := result.Items[i]
		items = append(items, dto.NewDocumentResponse(&doc).WithOwner(user))
	}

	return respondJSON(c, http.StatusOK, dto.List(items, result.Total, result.Page, result.PageSize))
}

func (h *SearchHandler) reindex(c *echo.Context) error {
	if !h.service.Enabled() {
		return respondError(c, http.StatusBadRequest, "search is not configured")
	}
	if h.queue == nil {
		return respondError(c, http.StatusServiceUnavailable, "task queue is not configured")
	}
	if err := h.queue.Enqueue(c.Request().Context(), queue.TaskReindex, queue.ReindexPayload{}); err != nil {
		return respondError(c, http.StatusInternalServerError, "failed to enqueue reindex")
	}
	return respondJSON(c, http.StatusAccepted, dto.OK(map[string]string{"status": "queued"}))
}
