package handler

import (
	"errors"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/labstack/echo/v5"

	"github.com/xmtrxi/edms/internal/cache"
	"github.com/xmtrxi/edms/internal/dto"
	"github.com/xmtrxi/edms/internal/middleware"
	"github.com/xmtrxi/edms/internal/service"
)

// ESignHandler serves the built-in signing flow: senders prepare and track
// requests; recipients sign through a personal link without an account.
type ESignHandler struct {
	auth  *service.AuthService
	esign *service.ESignService
	cache *cache.Cache
}

func NewESignHandler(deps Deps) *ESignHandler {
	docs := service.NewDocumentService(deps.Cfg, deps.Client(), deps.Storage, deps.Queue, deps.Search, deps.Logger)
	previews := service.NewPreviewService(docs, deps.Storage, deps.Office)
	email := service.NewOperationsService(deps.Client(), deps.Cfg.SMTP, deps.Queue)
	return &ESignHandler{
		auth:  service.NewAuthService(deps.Cfg, deps.Client(), deps.Logger),
		esign: service.NewESignService(deps.Cfg, deps.Client(), deps.Storage, docs, previews, email, deps.Logger),
		cache: deps.Cache,
	}
}

func (h *ESignHandler) RegisterRoutes(api *echo.Group) {
	g := api.Group("", middleware.RequireAuth(h.auth))
	g.POST("/documents/:id/signatures", h.create)
	g.GET("/documents/:id/signatures", h.forDocument)
	g.GET("/signatures", h.list)
	g.GET("/signatures/:id", h.get)
	g.POST("/signatures/:id/void", h.void)
	g.POST("/signatures/:id/remind", h.remind)
	g.POST("/signatures/:id/retry", h.retry)

	p := api.Group("/public/sign/:token")
	p.GET("", h.open)
	p.GET("/document", h.document)
	p.POST("/code", h.sendCode)
	p.POST("/verify", h.verifyCode)
	p.POST("/complete", h.sign)
	p.POST("/decline", h.decline)
}

func (h *ESignHandler) create(c *echo.Context) error {
	var input service.EnvelopeInput
	if err := bind(c, &input); err != nil {
		return err
	}
	view, err := h.esign.Create(c.Request().Context(), actor(c), c.Param("id"), input)
	if err != nil {
		return esignError(c, err)
	}
	return respondJSON(c, http.StatusCreated, dto.Created(view))
}

func (h *ESignHandler) forDocument(c *echo.Context) error {
	items, err := h.esign.ForDocument(c.Request().Context(), actor(c), c.Param("id"))
	if err != nil {
		return esignError(c, err)
	}
	return respondJSON(c, http.StatusOK, dto.List(items, int64(len(items)), 1, len(items)))
}

func (h *ESignHandler) list(c *echo.Context) error {
	ctx := c.Request().Context()
	var items []service.EnvelopeView
	var err error
	if c.QueryParam("box") == "sent" {
		items, err = h.esign.Sent(ctx, actor(c))
	} else {
		items, err = h.esign.Inbox(ctx, actor(c))
	}
	if err != nil {
		return esignError(c, err)
	}
	if items == nil {
		items = []service.EnvelopeView{}
	}
	return respondJSON(c, http.StatusOK, dto.List(items, int64(len(items)), 1, len(items)))
}

func (h *ESignHandler) get(c *echo.Context) error {
	view, err := h.esign.Get(c.Request().Context(), actor(c), c.Param("id"))
	if err != nil {
		return esignError(c, err)
	}
	return respondJSON(c, http.StatusOK, dto.OK(view))
}

func (h *ESignHandler) void(c *echo.Context) error {
	var input struct {
		Reason string `json:"reason"`
	}
	if err := bind(c, &input); err != nil {
		return err
	}
	if err := h.esign.Void(c.Request().Context(), actor(c), c.Param("id"), input.Reason); err != nil {
		return esignError(c, err)
	}
	return c.NoContent(http.StatusNoContent)
}

func (h *ESignHandler) remind(c *echo.Context) error {
	count, err := h.esign.Remind(c.Request().Context(), actor(c), c.Param("id"))
	if err != nil {
		return esignError(c, err)
	}
	return respondJSON(c, http.StatusOK, dto.OK(map[string]int{"reminded": count}))
}

func (h *ESignHandler) retry(c *echo.Context) error {
	if err := h.esign.Retry(c.Request().Context(), actor(c), c.Param("id")); err != nil {
		return esignError(c, err)
	}
	return h.get(c)
}

// ── Public, by link ─────────────────────────────────────────────────────────

func (h *ESignHandler) open(c *echo.Context) error {
	_, session, err := h.esign.Open(c.Request().Context(), c.Param("token"), c.RealIP(), c.Request().UserAgent())
	if err != nil {
		return esignError(c, err)
	}
	return respondJSON(c, http.StatusOK, dto.OK(session))
}

func (h *ESignHandler) document(c *echo.Context) error {
	body, name, err := h.esign.SourcePDF(c.Request().Context(), c.Param("token"))
	if err != nil {
		return esignError(c, err)
	}
	defer body.Close()
	disposition := "inline"
	if c.QueryParam("download") != "" {
		disposition = "attachment"
	}
	c.Response().Header().Set(echo.HeaderContentDisposition, disposition+"; filename*=UTF-8''"+url.PathEscape(name))
	c.Response().Header().Set("Cache-Control", "private, no-store")
	c.Response().Header().Set(echo.HeaderContentType, "application/pdf")
	c.Response().WriteHeader(http.StatusOK)
	_, err = io.Copy(c.Response(), body)
	return err
}

func (h *ESignHandler) sendCode(c *echo.Context) error {
	if !h.allow(c, "esign:code:"+c.RealIP(), 20) {
		return respondError(c, http.StatusTooManyRequests, "too many code requests; try again in 10 minutes")
	}
	if err := h.esign.SendCode(c.Request().Context(), c.Param("token"), c.RealIP(), c.Request().UserAgent()); err != nil {
		return esignError(c, err)
	}
	return c.NoContent(http.StatusNoContent)
}

func (h *ESignHandler) verifyCode(c *echo.Context) error {
	var input struct {
		Code string `json:"code"`
	}
	if err := bind(c, &input); err != nil {
		return err
	}
	if !h.allow(c, "esign:verify:"+c.RealIP(), 30) {
		return respondError(c, http.StatusTooManyRequests, "too many attempts; try again in 10 minutes")
	}
	ctx := c.Request().Context()
	if err := h.esign.VerifyCode(ctx, c.Param("token"), input.Code, c.RealIP(), c.Request().UserAgent()); err != nil {
		return esignError(c, err)
	}
	return h.open(c)
}

func (h *ESignHandler) sign(c *echo.Context) error {
	c.Request().Body = http.MaxBytesReader(c.Response(), c.Request().Body, 12<<20)
	var input service.SignInput
	if err := bind(c, &input); err != nil {
		return err
	}
	session, err := h.esign.Sign(c.Request().Context(), c.Param("token"), input, c.RealIP(), c.Request().UserAgent())
	if err != nil {
		return esignError(c, err)
	}
	return respondJSON(c, http.StatusOK, dto.OK(session))
}

func (h *ESignHandler) decline(c *echo.Context) error {
	var input struct {
		Reason string `json:"reason"`
	}
	if err := bind(c, &input); err != nil {
		return err
	}
	if err := h.esign.Decline(c.Request().Context(), c.Param("token"), input.Reason, c.RealIP(), c.Request().UserAgent()); err != nil {
		return esignError(c, err)
	}
	return c.NoContent(http.StatusNoContent)
}

func (h *ESignHandler) allow(c *echo.Context, key string, limit int64) bool {
	if h.cache == nil {
		return true
	}
	count, err := h.cache.IncrementWindow(c.Request().Context(), key, 10*time.Minute)
	return err == nil && count <= limit
}

func esignError(c *echo.Context, err error) error {
	switch {
	case errors.Is(err, service.ErrEnvelopeNotFound), errors.Is(err, service.ErrDocumentNotFound):
		return respondError(c, http.StatusNotFound, "this signature request isn’t available")
	case errors.Is(err, service.ErrEnvelopeClosed):
		return respondError(c, http.StatusGone, err.Error())
	case errors.Is(err, service.ErrForbidden):
		return respondError(c, http.StatusForbidden, "you can’t send this document for signature")
	case errors.Is(err, service.ErrNotYourTurn), errors.Is(err, service.ErrCodeRequired):
		return respondError(c, http.StatusConflict, err.Error())
	case errors.Is(err, service.ErrCodeInvalid):
		return respondError(c, http.StatusUnprocessableEntity, err.Error())
	case errors.Is(err, service.ErrCodeTooSoon):
		return respondError(c, http.StatusTooManyRequests, err.Error())
	default:
		return respondError(c, http.StatusBadRequest, err.Error())
	}
}
