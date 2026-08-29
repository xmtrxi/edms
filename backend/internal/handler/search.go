package handler

import (
	"log/slog"
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
	log     *slog.Logger
}

func NewSearchHandler(deps Deps) *SearchHandler {
	return &SearchHandler{
		service: service.NewSearchService(deps.Client(), deps.Storage, deps.Search, deps.Logger),
		auth:    service.NewAuthService(deps.Cfg, deps.Client(), deps.Logger),
		queue:   deps.Queue,
		log:     deps.Logger,
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
		OwnerID:  string(user.ID),
		Status:   c.QueryParam("status"),
		Page:     page,
		PageSize: pageSize,
	})
	if err != nil {
		h.log.Error("search failed", "error", err, "query", query, "user_id", user.ID)
		return respondError(c, http.StatusInternalServerError, "search failed")
	}

	items := make([]dto.SearchResultResponse, 0, len(result.Items))
	for i := range result.Items {
		item := result.Items[i]
		response := dto.NewSearchResultResponse(&item.Document, item.Snippet)
		response.DocumentResponse = response.DocumentResponse.WithOwner(user)
		items = append(items, response)
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
