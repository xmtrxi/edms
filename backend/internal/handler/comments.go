package handler

import (
	"errors"
	"net/http"

	"github.com/labstack/echo/v5"

	"github.com/xmtrxi/edms/internal/dto"
	"github.com/xmtrxi/edms/internal/middleware"
	"github.com/xmtrxi/edms/internal/service"
)

type CommentsHandler struct {
	auth     *service.AuthService
	comments *service.CommentService
}

func NewCommentsHandler(deps Deps) *CommentsHandler {
	docs := service.NewDocumentService(deps.Cfg, deps.Client(), deps.Storage, deps.Queue, deps.Search, deps.Logger)
	email := service.NewOperationsService(deps.Client(), deps.Cfg.SMTP, deps.Queue)
	return &CommentsHandler{
		auth:     service.NewAuthService(deps.Cfg, deps.Client(), deps.Logger),
		comments: service.NewCommentService(deps.Client(), docs, email, deps.Cfg.SMTP.AppURL),
	}
}

func (h *CommentsHandler) RegisterRoutes(api *echo.Group) {
	g := api.Group("", middleware.RequireAuth(h.auth))
	g.GET("/documents/:id/comments", h.list)
	g.POST("/documents/:id/comments", h.create)
	g.PATCH("/comments/:id", h.update)
	g.DELETE("/comments/:id", h.remove)
}

func (h *CommentsHandler) list(c *echo.Context) error {
	items, err := h.comments.List(c.Request().Context(), actor(c), c.Param("id"))
	if err != nil {
		return commentError(c, err)
	}
	return respondJSON(c, http.StatusOK, dto.List(items, int64(len(items)), 1, len(items)))
}

func (h *CommentsHandler) create(c *echo.Context) error {
	var input service.CommentInput
	if err := bind(c, &input); err != nil {
		return err
	}
	view, err := h.comments.Create(c.Request().Context(), actor(c), c.Param("id"), input)
	if err != nil {
		return commentError(c, err)
	}
	return respondJSON(c, http.StatusCreated, dto.Created(view))
}

// update edits the text, or resolves and reopens a thread.
func (h *CommentsHandler) update(c *echo.Context) error {
	var input struct {
		Body     *string `json:"body"`
		Resolved *bool   `json:"resolved"`
	}
	if err := bind(c, &input); err != nil {
		return err
	}
	ctx := c.Request().Context()
	var view *service.CommentView
	var err error
	switch {
	case input.Resolved != nil:
		view, err = h.comments.Resolve(ctx, actor(c), c.Param("id"), *input.Resolved)
	case input.Body != nil:
		view, err = h.comments.Update(ctx, actor(c), c.Param("id"), *input.Body)
	default:
		return respondError(c, http.StatusBadRequest, "nothing to change")
	}
	if err != nil {
		return commentError(c, err)
	}
	return respondJSON(c, http.StatusOK, dto.OK(view))
}

func (h *CommentsHandler) remove(c *echo.Context) error {
	if err := h.comments.Delete(c.Request().Context(), actor(c), c.Param("id")); err != nil {
		return commentError(c, err)
	}
	return c.NoContent(http.StatusNoContent)
}

func commentError(c *echo.Context, err error) error {
	switch {
	case errors.Is(err, service.ErrCommentNotFound), errors.Is(err, service.ErrDocumentNotFound):
		return respondError(c, http.StatusNotFound, "comment not found")
	case errors.Is(err, service.ErrForbidden):
		return respondError(c, http.StatusForbidden, "you can’t do that on this document")
	default:
		return respondError(c, http.StatusBadRequest, err.Error())
	}
}
