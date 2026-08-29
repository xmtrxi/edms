package handler

import (
	"net/http"

	"github.com/labstack/echo/v5"

	"github.com/xmtrxi/edms/internal/dto"
	"github.com/xmtrxi/edms/internal/middleware"
	"github.com/xmtrxi/edms/internal/service"
)

type AuthHandler struct {
	service *service.AuthService
}

func NewAuthHandler(deps Deps) *AuthHandler {
	return &AuthHandler{service: service.NewAuthService(deps.Cfg, deps.Client(), deps.Logger)}
}

func (h *AuthHandler) RegisterRoutes(api *echo.Group) {
	api.POST("/auth/register", h.register)
	api.POST("/auth/setup", h.setup)
	api.POST("/auth/login", h.login)
	api.GET("/auth/me", h.me, middleware.RequireAuth(h.service))
	api.GET("/users", h.users, middleware.RequireAuth(h.service))
}
func (h *AuthHandler) setup(c *echo.Context) error {
	var r struct {
		Token    string `json:"token"`
		Password string `json:"password"`
	}
	if err := bind(c, &r); err != nil {
		return err
	}
	user, err := h.service.AcceptInvitation(c.Request().Context(), r.Token, r.Password)
	if err != nil {
		return respondError(c, http.StatusBadRequest, err.Error())
	}
	return respondJSON(c, http.StatusCreated, dto.Created(dto.NewUserResponse(user)))
}

type registerRequest struct {
	Email    string `json:"email"`
	Name     string `json:"name"`
	Password string `json:"password"`
}

func (h *AuthHandler) register(c *echo.Context) error {
	var req registerRequest
	if err := bind(c, &req); err != nil {
		return err
	}

	user, err := h.service.Register(c.Request().Context(), service.RegisterInput{
		Email:    req.Email,
		Name:     req.Name,
		Password: req.Password,
	})
	if err != nil {
		return respondError(c, http.StatusBadRequest, err.Error())
	}

	return respondJSON(c, http.StatusCreated, dto.Created(dto.NewUserResponse(user)))
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (h *AuthHandler) login(c *echo.Context) error {
	var req loginRequest
	if err := bind(c, &req); err != nil {
		return err
	}

	user, token, err := h.service.Login(c.Request().Context(), service.LoginInput{
		Email:    req.Email,
		Password: req.Password,
	})
	if err != nil {
		return respondError(c, http.StatusUnauthorized, "invalid credentials")
	}

	return respondJSON(c, http.StatusOK, dto.OK(dto.AuthResponse{
		Token: token,
		User:  dto.NewUserResponse(user),
	}))
}

func (h *AuthHandler) me(c *echo.Context) error {
	user := middleware.CurrentUser(c)
	if user == nil {
		return respondError(c, http.StatusUnauthorized, "unauthorized")
	}
	return respondJSON(c, http.StatusOK, dto.OK(dto.NewUserResponse(user)))
}

func (h *AuthHandler) users(c *echo.Context) error {
	users, err := h.service.ListUsers(c.Request().Context())
	if err != nil {
		return respondError(c, http.StatusInternalServerError, "failed to list users")
	}
	items := make([]dto.UserResponse, 0, len(users))
	for i := range users {
		items = append(items, dto.NewUserResponse(&users[i]))
	}
	return respondJSON(c, http.StatusOK, dto.List(items, int64(len(items)), 1, len(items)))
}
