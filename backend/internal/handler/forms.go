package handler

import (
	"github.com/labstack/echo/v5"
	"github.com/xmtrxi/edms/internal/dto"
	"github.com/xmtrxi/edms/internal/middleware"
	"github.com/xmtrxi/edms/internal/service"
	"net/http"
)

type FormsHandler struct {
	auth  *service.AuthService
	authz *service.AuthorizationService
	forms *service.FormsService
}

func NewFormsHandler(deps Deps) *FormsHandler {
	authz, e := service.NewAuthorizationService(deps.Client())
	if e != nil {
		panic(e)
	}
	return &FormsHandler{auth: service.NewAuthService(deps.Cfg, deps.Client(), deps.Logger), authz: authz, forms: service.NewFormsService(deps.Client())}
}
func (h *FormsHandler) RegisterRoutes(api *echo.Group) {
	g := api.Group("/forms", middleware.RequireAuth(h.auth))
	g.GET("", h.templates)
	g.POST("", h.create, middleware.RequirePermission(h.authz, "forms", "create"))
	g.POST("/:id/submissions", h.submit)
	g.GET("/submissions/mine", h.submissions)
	g.POST("/submissions/:id/comments", h.comment)
	g.POST("/submissions/:id/decision", h.decide)
}
func (h *FormsHandler) templates(c *echo.Context) error {
	items, e := h.forms.Templates(c.Request().Context())
	if e != nil {
		return respondError(c, 500, "failed to list forms")
	}
	return respondJSON(c, 200, dto.List(items, int64(len(items)), 1, len(items)))
}
func (h *FormsHandler) create(c *echo.Context) error {
	var r service.FormTemplateInput
	if e := bind(c, &r); e != nil {
		return e
	}
	item, e := h.forms.CreateTemplate(c.Request().Context(), string(middleware.CurrentUser(c).ID), r)
	if e != nil {
		return respondError(c, 400, e.Error())
	}
	return respondJSON(c, http.StatusCreated, dto.Created(item))
}
func (h *FormsHandler) submit(c *echo.Context) error {
	var r service.FormSubmissionInput
	if e := bind(c, &r); e != nil {
		return e
	}
	item, e := h.forms.Submit(c.Request().Context(), string(middleware.CurrentUser(c).ID), c.Param("id"), r)
	if e != nil {
		return respondError(c, 400, e.Error())
	}
	return respondJSON(c, http.StatusCreated, dto.Created(item))
}
func (h *FormsHandler) submissions(c *echo.Context) error {
	items, e := h.forms.Submissions(c.Request().Context(), string(middleware.CurrentUser(c).ID))
	if e != nil {
		return respondError(c, 500, "failed to list submissions")
	}
	return respondJSON(c, 200, dto.List(items, int64(len(items)), 1, len(items)))
}
func (h *FormsHandler) comment(c *echo.Context) error {
	var r struct {
		Body string `json:"body"`
	}
	if e := bind(c, &r); e != nil {
		return e
	}
	item, e := h.forms.AddComment(c.Request().Context(), string(middleware.CurrentUser(c).ID), c.Param("id"), r.Body)
	if e != nil {
		return respondError(c, 400, e.Error())
	}
	return respondJSON(c, http.StatusCreated, dto.Created(item))
}
func (h *FormsHandler) decide(c *echo.Context) error {
	var r service.ApprovalDecisionInput
	if e := bind(c, &r); e != nil {
		return e
	}
	if e := h.forms.Decide(c.Request().Context(), string(middleware.CurrentUser(c).ID), c.Param("id"), r); e != nil {
		return respondError(c, 400, e.Error())
	}
	return c.NoContent(http.StatusNoContent)
}
