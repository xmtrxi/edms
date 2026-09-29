package handler

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/labstack/echo/v5"

	"github.com/xmtrxi/edms/internal/database"
	"github.com/xmtrxi/edms/internal/dto"
	"github.com/xmtrxi/edms/internal/middleware"
	"github.com/xmtrxi/edms/internal/service"
)

type FileRequestsHandler struct {
	auth     *service.AuthService
	requests *service.FileRequestService
}

func NewFileRequestsHandler(deps Deps) *FileRequestsHandler {
	docs := service.NewDocumentService(deps.Cfg, deps.Client(), deps.Storage, deps.Queue, deps.Search, deps.Logger)
	uploads := service.NewUploadService(deps.Cfg.Uploads, deps.Client(), docs)
	return &FileRequestsHandler{
		auth:     service.NewAuthService(deps.Cfg, deps.Client(), deps.Logger),
		requests: service.NewFileRequestService(deps.Client(), uploads, deps.Cfg.SMTP.AppURL),
	}
}

func (h *FileRequestsHandler) RegisterRoutes(api *echo.Group) {
	g := api.Group("", middleware.RequireAuth(h.auth))
	g.POST("/file-requests", h.create)
	g.GET("/folders/:id/file-requests", h.list)
	g.POST("/file-requests/:id/close", h.close)

	// Anyone holding the link may upload; there is no account involved.
	p := api.Group("/public/file-requests/:token")
	p.GET("", h.open)
	p.POST("/uploads", h.startUpload)
	p.PUT("/uploads/:id", h.uploadChunk)
	p.POST("/uploads/:id/complete", h.completeUpload)
}

type fileRequestResponse struct {
	ID            string    `json:"id"`
	FolderID      string    `json:"folder_id"`
	Title         string    `json:"title"`
	Instructions  string    `json:"instructions,omitempty"`
	MaxFiles      int       `json:"max_files"`
	ReceivedFiles int       `json:"received_files"`
	ExpiresAt     time.Time `json:"expires_at"`
	Closed        bool      `json:"closed"`
	CreatedAt     time.Time `json:"created_at"`
	URL           string    `json:"url,omitempty"`
}

func newFileRequestResponse(request *database.FileRequest, url string) fileRequestResponse {
	return fileRequestResponse{
		ID: string(request.ID), FolderID: request.FolderID, Title: request.Title, Instructions: request.Instructions,
		MaxFiles: request.MaxFiles, ReceivedFiles: request.ReceivedFiles, ExpiresAt: request.ExpiresAt,
		Closed: !request.ClosedAt.IsZero() || time.Now().UTC().After(request.ExpiresAt), CreatedAt: request.CreatedAt, URL: url,
	}
}

func (h *FileRequestsHandler) create(c *echo.Context) error {
	var r service.FileRequestInput
	if err := bind(c, &r); err != nil {
		return err
	}
	request, url, err := h.requests.Create(c.Request().Context(), actor(c), r)
	if err != nil {
		return fileRequestError(c, err)
	}
	return respondJSON(c, http.StatusCreated, dto.Created(newFileRequestResponse(request, url)))
}

func (h *FileRequestsHandler) list(c *echo.Context) error {
	items, err := h.requests.List(c.Request().Context(), actor(c), c.Param("id"))
	if err != nil {
		return fileRequestError(c, err)
	}
	result := make([]fileRequestResponse, 0, len(items))
	for i := range items {
		result = append(result, newFileRequestResponse(&items[i], ""))
	}
	return respondJSON(c, http.StatusOK, dto.List(result, int64(len(result)), 1, len(result)))
}

func (h *FileRequestsHandler) close(c *echo.Context) error {
	if err := h.requests.Close(c.Request().Context(), actor(c), c.Param("id")); err != nil {
		return fileRequestError(c, err)
	}
	return c.NoContent(http.StatusNoContent)
}

func (h *FileRequestsHandler) open(c *echo.Context) error {
	_, view, err := h.requests.Open(c.Request().Context(), c.Param("token"))
	if err != nil && !errors.Is(err, service.ErrFileRequestFull) {
		return fileRequestError(c, err)
	}
	return respondJSON(c, http.StatusOK, dto.OK(view))
}

func (h *FileRequestsHandler) startUpload(c *echo.Context) error {
	var r struct {
		Name     string `json:"name"`
		MimeType string `json:"mime_type"`
		Size     int64  `json:"size"`
	}
	if err := bind(c, &r); err != nil {
		return err
	}
	session, err := h.requests.StartUpload(c.Request().Context(), c.Param("token"), r.Name, r.MimeType, r.Size)
	if err != nil {
		return fileRequestError(c, err)
	}
	return respondJSON(c, http.StatusCreated, dto.Created(newUploadProgress(session)))
}

func (h *FileRequestsHandler) uploadChunk(c *echo.Context) error {
	offset, err := strconv.ParseInt(c.QueryParam("offset"), 10, 64)
	if err != nil || offset < 0 {
		return respondError(c, http.StatusBadRequest, "offset is required")
	}
	session, err := h.requests.AppendChunk(c.Request().Context(), c.Param("token"), c.Param("id"), offset, c.Request().Body)
	if errors.Is(err, service.ErrUploadOffset) && session != nil {
		return c.JSON(http.StatusConflict, map[string]any{"error": err.Error(), "received": session.Received})
	}
	if err != nil {
		return fileRequestError(c, err)
	}
	return respondJSON(c, http.StatusOK, dto.OK(newUploadProgress(session)))
}

func (h *FileRequestsHandler) completeUpload(c *echo.Context) error {
	var r struct {
		Name  string `json:"name"`
		Email string `json:"email"`
	}
	_ = c.Bind(&r)
	doc, err := h.requests.CompleteUpload(c.Request().Context(), c.Param("token"), c.Param("id"), r.Name, r.Email)
	if err != nil {
		return fileRequestError(c, err)
	}
	// Uploaders only learn that their file arrived, not where it is stored.
	return respondJSON(c, http.StatusCreated, dto.Created(map[string]string{"name": doc.Name}))
}

func fileRequestError(c *echo.Context, err error) error {
	switch {
	case errors.Is(err, service.ErrShareNotFound):
		return respondError(c, http.StatusNotFound, "this link isn’t available")
	case errors.Is(err, service.ErrFileRequestClosed), errors.Is(err, service.ErrFileRequestFull):
		return respondError(c, http.StatusGone, err.Error())
	case errors.Is(err, service.ErrForbidden):
		return respondError(c, http.StatusForbidden, "you do not have upload access to this folder")
	default:
		return uploadError(c, err)
	}
}
