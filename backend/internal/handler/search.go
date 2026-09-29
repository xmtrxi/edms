package handler

import (
	"errors"
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
	mode := strings.ToLower(strings.TrimSpace(c.QueryParam("mode")))
	if mode == "" {
		mode = "documents"
	}
	if mode != "documents" && mode != "passages" {
		return respondError(c, http.StatusBadRequest, "search mode must be documents or passages")
	}
	result, err := h.service.Search(c.Request().Context(), service.SearchInput{
		Query:      query,
		OwnerID:    string(user.ID),
		Status:     c.QueryParam("status"),
		Mode:       mode,
		DocumentID: strings.TrimSpace(c.QueryParam("document_id")),
		Page:       page,
		PageSize:   pageSize,
	})
	if err != nil {
		if errors.Is(err, service.ErrAccessDenied) {
			return respondError(c, http.StatusForbidden, "forbidden")
		}
		if errors.Is(err, service.ErrDocumentNotFound) {
			return respondError(c, http.StatusNotFound, "document not found")
		}
		h.log.Error("search failed", "error", err, "query", query, "user_id", user.ID)
		return respondError(c, http.StatusInternalServerError, "search failed")
	}

	ownerIDs := make([]string, 0, len(result.Items))
	for i := range result.Items {
		ownerIDs = append(ownerIDs, result.Items[i].Document.OwnerID)
	}
	owners := h.auth.Orgs.UsersByID(c.Request().Context(), ownerIDs)
	items := make([]dto.SearchResultResponse, 0, len(result.Items))
	for i := range result.Items {
		item := result.Items[i]
		response := dto.NewSearchResultResponse(&item.Document, item.Snippet, item.Passage.VersionID, item.Passage.ChunkIndex, item.Passage.Headings, item.Passage.PageNumbers, item.Passage.PageStart, item.Passage.PageEnd)
		if item.Document.OwnerID == string(user.ID) {
			response.DocumentResponse = response.DocumentResponse.WithOwner(user)
		} else if owner := owners[item.Document.OwnerID]; owner != nil {
			response.DocumentResponse = response.DocumentResponse.WithOwner(owner)
		}
		items = append(items, response)
	}

	return respondJSON(c, http.StatusOK, dto.List(items, result.Total, result.Page, result.PageSize))
}

// reindex rebuilds the index for every organization, so on SaaS it is an
// operator task and on self-hosted installs it needs an organization admin.
func (h *SearchHandler) reindex(c *echo.Context) error {
	if h.auth.Orgs.Capabilities().IsSaaS() || !middleware.IsOrgAdmin(c) {
		return respondError(c, http.StatusForbidden, "only an administrator of a self-hosted installation can rebuild the index")
	}
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
