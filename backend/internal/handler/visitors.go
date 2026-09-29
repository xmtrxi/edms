package handler

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/labstack/echo/v5"
	"github.com/xmtrxi/edms/internal/cache"
	"github.com/xmtrxi/edms/internal/dto"
	"github.com/xmtrxi/edms/internal/middleware"
	"github.com/xmtrxi/edms/internal/service"
)

type VisitorsHandler struct {
	auth     *service.AuthService
	authz    *service.AuthorizationService
	visitors *service.VisitorService
	cache    *cache.Cache
}

func NewVisitorsHandler(deps Deps) *VisitorsHandler {
	authz, err := service.NewAuthorizationService(deps.Client())
	if err != nil {
		panic(err)
	}
	return &VisitorsHandler{auth: service.NewAuthService(deps.Cfg, deps.Client(), deps.Logger), authz: authz, visitors: service.NewVisitorService(deps.Cfg, deps.Client(), deps.Storage, deps.Queue), cache: deps.Cache}
}

func (h *VisitorsHandler) RegisterRoutes(api *echo.Group) {
	staff := api.Group("/visitor-requests", middleware.RequireAuth(h.auth))
	staff.POST("", h.create, middleware.RequirePermission(h.authz, "forms", "create"))
	staff.GET("", h.list)
	staff.GET("/:id", h.get)
	staff.POST("/:id/revoke", h.revoke)
	staff.POST("/:id/resend", h.resend)
	staff.GET("/:id/attachments/:attachment_id/download", h.downloadAttachment)

	public := api.Group("/visitor")
	public.POST("/access/request", h.requestOTP)
	public.POST("/access/verify", h.verify)
	public.GET("/portal", h.portal)
	public.POST("/submit", h.submit)
	public.GET("/attachments", h.attachments)
	public.POST("/attachments", h.uploadAttachment)
	public.DELETE("/attachments/:id", h.deleteAttachment)
}

func (h *VisitorsHandler) list(c *echo.Context) error {
	items, err := h.visitors.List(c.Request().Context(), string(middleware.CurrentUser(c).ID))
	if err != nil {
		return respondError(c, http.StatusInternalServerError, "failed to list visitor requests")
	}
	return respondJSON(c, http.StatusOK, dto.List(items, int64(len(items)), 1, len(items)))
}

func (h *VisitorsHandler) get(c *echo.Context) error {
	item, err := h.visitors.Get(c.Request().Context(), string(middleware.CurrentUser(c).ID), c.Param("id"))
	if err != nil {
		return respondError(c, http.StatusNotFound, err.Error())
	}
	return respondJSON(c, http.StatusOK, dto.OK(item))
}

func (h *VisitorsHandler) revoke(c *echo.Context) error {
	if err := h.visitors.Revoke(c.Request().Context(), string(middleware.CurrentUser(c).ID), c.Param("id")); err != nil {
		return respondError(c, http.StatusBadRequest, err.Error())
	}
	return c.NoContent(http.StatusNoContent)
}

func (h *VisitorsHandler) resend(c *echo.Context) error {
	item, err := h.visitors.Resend(c.Request().Context(), string(middleware.CurrentUser(c).ID), c.Param("id"))
	if err != nil {
		return respondError(c, http.StatusBadRequest, err.Error())
	}
	return respondJSON(c, http.StatusOK, dto.OK(item))
}

func (h *VisitorsHandler) create(c *echo.Context) error {
	var input service.CreateVisitorRequestInput
	if err := bind(c, &input); err != nil {
		return err
	}
	created, err := h.visitors.Create(c.Request().Context(), string(middleware.CurrentUser(c).ID), input)
	if err != nil {
		return respondError(c, http.StatusBadRequest, err.Error())
	}
	return respondJSON(c, http.StatusCreated, dto.Created(created))
}

func (h *VisitorsHandler) requestOTP(c *echo.Context) error {
	var input service.VisitorAccessInput
	if err := bind(c, &input); err != nil {
		return err
	}
	if !h.allowOTP(c, input.Email) {
		return respondError(c, http.StatusTooManyRequests, "too many verification requests; try again in 10 minutes")
	}
	delivery, err := h.visitors.RequestOTP(c.Request().Context(), input)
	if err != nil {
		return h.visitorError(c, err)
	}
	return respondJSON(c, http.StatusOK, dto.WithMessage(delivery, "Verification code sent"))
}

func (h *VisitorsHandler) allowOTP(c *echo.Context, email string) bool {
	if h.cache == nil {
		return true
	}
	digest := sha256.Sum256([]byte(strings.ToLower(strings.TrimSpace(email))))
	keys := []struct {
		key   string
		limit int64
	}{
		{key: "visitor:otp:ip:" + c.RealIP(), limit: 20},
		{key: "visitor:otp:email:" + hex.EncodeToString(digest[:]), limit: 5},
	}
	for _, item := range keys {
		count, err := h.cache.IncrementWindow(c.Request().Context(), item.key, 10*time.Minute)
		if err != nil || count > item.limit {
			return false
		}
	}
	return true
}

func (h *VisitorsHandler) verify(c *echo.Context) error {
	var input service.VisitorVerifyInput
	if err := bind(c, &input); err != nil {
		return err
	}
	session, err := h.visitors.Verify(c.Request().Context(), input)
	if err != nil {
		return h.visitorError(c, err)
	}
	return respondJSON(c, http.StatusOK, dto.OK(session))
}

func (h *VisitorsHandler) portal(c *echo.Context) error {
	portal, err := h.visitors.Portal(c.Request().Context(), visitorToken(c))
	if err != nil {
		return h.visitorError(c, err)
	}
	return respondJSON(c, http.StatusOK, dto.OK(portal))
}

func (h *VisitorsHandler) submit(c *echo.Context) error {
	var input struct {
		Values map[string]any `json:"values"`
	}
	if err := bind(c, &input); err != nil {
		return err
	}
	submission, err := h.visitors.Submit(c.Request().Context(), visitorToken(c), input.Values)
	if err != nil {
		return h.visitorError(c, err)
	}
	return respondJSON(c, http.StatusCreated, dto.Created(submission))
}

func (h *VisitorsHandler) attachments(c *echo.Context) error {
	items, err := h.visitors.Attachments(c.Request().Context(), visitorToken(c))
	if err != nil {
		return h.visitorError(c, err)
	}
	return respondJSON(c, http.StatusOK, dto.OK(items))
}

func (h *VisitorsHandler) uploadAttachment(c *echo.Context) error {
	file, err := c.FormFile("file")
	if err != nil {
		return respondError(c, http.StatusBadRequest, "a file is required")
	}
	src, err := file.Open()
	if err != nil {
		return respondError(c, http.StatusBadRequest, "could not read attachment")
	}
	defer src.Close()
	name := filepath.Base(file.Filename)
	item, err := h.visitors.AddAttachment(c.Request().Context(), visitorToken(c), name, file.Header.Get("Content-Type"), file.Size, src)
	if err != nil {
		return h.visitorError(c, err)
	}
	return respondJSON(c, http.StatusCreated, dto.Created(item))
}

func (h *VisitorsHandler) deleteAttachment(c *echo.Context) error {
	if err := h.visitors.DeleteAttachment(c.Request().Context(), visitorToken(c), c.Param("id")); err != nil {
		return h.visitorError(c, err)
	}
	return c.NoContent(http.StatusNoContent)
}

func (h *VisitorsHandler) downloadAttachment(c *echo.Context) error {
	item, reader, err := h.visitors.DownloadAttachment(c.Request().Context(), string(middleware.CurrentUser(c).ID), c.Param("id"), c.Param("attachment_id"))
	if err != nil {
		return respondError(c, http.StatusNotFound, err.Error())
	}
	defer reader.Close()
	c.Response().Header().Set(echo.HeaderContentDisposition, "attachment; filename=\""+strings.ReplaceAll(item.Name, "\"", "")+"\"")
	return c.Stream(http.StatusOK, item.MimeType, reader)
}

func (h *VisitorsHandler) visitorError(c *echo.Context, err error) error {
	if errors.Is(err, service.ErrVisitorAccess) {
		return respondError(c, http.StatusUnauthorized, service.ErrVisitorAccess.Error())
	}
	return respondError(c, http.StatusBadRequest, err.Error())
}

func visitorToken(c *echo.Context) string {
	if token := strings.TrimSpace(c.Request().Header.Get("X-Visitor-Token")); token != "" {
		return token
	}
	return strings.TrimPrefix(c.Request().Header.Get(echo.HeaderAuthorization), "Bearer ")
}
