package handler

import (
	"errors"
	"net/http"

	"github.com/labstack/echo/v5"

	"github.com/xmtrxi/edms/internal/database"
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
	api.GET("/meta", h.meta)
	api.POST("/auth/register", h.register)
	api.POST("/auth/setup", h.setup)
	api.POST("/auth/login", h.login)
	api.GET("/invitations/:token", h.previewInvitation)

	authed := api.Group("", middleware.RequireAuth(h.service))
	authed.GET("/auth/me", h.me)
	authed.GET("/auth/session", h.session)
	authed.POST("/auth/switch-org", h.switchOrganization)
	authed.POST("/invitations/accept", h.acceptInvitation)
	authed.GET("/users", h.users)
}

// meta describes the deployment so the frontend can show or hide signup,
// organization creation and the workspace switcher. It is public.
func (h *AuthHandler) meta(c *echo.Context) error {
	return respondJSON(c, http.StatusOK, dto.OK(h.service.Orgs.Capabilities().Meta()))
}

// setup creates the account for a person invited by email.
func (h *AuthHandler) setup(c *echo.Context) error {
	var r struct {
		Token    string `json:"token"`
		Name     string `json:"name"`
		Password string `json:"password"`
	}
	if err := bind(c, &r); err != nil {
		return err
	}
	ctx := c.Request().Context()
	user, token, err := h.service.AcceptInvitation(ctx, r.Token, r.Name, r.Password)
	if err != nil {
		return respondError(c, http.StatusBadRequest, err.Error())
	}
	return h.respondAuth(c, http.StatusCreated, user, token)
}

type registerRequest struct {
	Email            string `json:"email"`
	Name             string `json:"name"`
	Password         string `json:"password"`
	OrganizationName string `json:"organization_name"`
}

func (h *AuthHandler) register(c *echo.Context) error {
	var req registerRequest
	if err := bind(c, &req); err != nil {
		return err
	}

	ctx := c.Request().Context()
	user, err := h.service.Register(ctx, service.RegisterInput{
		Email:            req.Email,
		Name:             req.Name,
		Password:         req.Password,
		OrganizationName: req.OrganizationName,
	})
	if err != nil {
		status := http.StatusBadRequest
		if errors.Is(err, service.ErrRegistrationClosed) {
			status = http.StatusForbidden
		}
		return respondError(c, status, err.Error())
	}
	token, err := h.service.SignToken(user, user.LastOrgID)
	if err != nil {
		return respondError(c, http.StatusInternalServerError, "failed to sign in")
	}
	return h.respondAuth(c, http.StatusCreated, user, token)
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
		if errors.Is(err, service.ErrNoOrganization) {
			return respondError(c, http.StatusForbidden, err.Error())
		}
		return respondError(c, http.StatusUnauthorized, "invalid credentials")
	}
	return h.respondAuth(c, http.StatusOK, user, token)
}

func (h *AuthHandler) respondAuth(c *echo.Context, status int, user *database.User, token string) error {
	ctx := c.Request().Context()
	claims, err := h.service.ParseToken(token)
	if err != nil {
		return respondError(c, http.StatusInternalServerError, "failed to sign in")
	}
	response := dto.AuthResponse{Token: token, User: dto.NewUserResponse(user)}
	if org, err := h.service.Orgs.Get(ctx, claims.OrgID); err == nil {
		role := ""
		if membership, err := h.service.Orgs.Membership(ctx, claims.OrgID, string(user.ID)); err == nil {
			role = membership.Role
		}
		response.Organization = dto.NewOrganizationResponse(org, role)
	}
	return respondJSON(c, status, dto.OK(response))
}

func (h *AuthHandler) me(c *echo.Context) error {
	user := middleware.CurrentUser(c)
	if user == nil {
		return respondError(c, http.StatusUnauthorized, "unauthorized")
	}
	return respondJSON(c, http.StatusOK, dto.OK(dto.NewUserResponse(user)))
}

func (h *AuthHandler) session(c *echo.Context) error {
	user := middleware.CurrentUser(c)
	orgs, err := h.service.Orgs.ForUser(c.Request().Context(), string(user.ID))
	if err != nil {
		return respondError(c, http.StatusInternalServerError, "failed to load organizations")
	}
	items := make([]dto.OrganizationResponse, 0, len(orgs))
	for i := range orgs {
		items = append(items, dto.NewOrganizationResponse(&orgs[i].Organization, orgs[i].Role))
	}
	role := ""
	if membership := middleware.CurrentMembership(c); membership != nil {
		role = membership.Role
	}
	return respondJSON(c, http.StatusOK, dto.OK(dto.SessionResponse{
		User:          dto.NewUserResponse(user),
		Organization:  dto.NewOrganizationResponse(middleware.CurrentOrg(c), role),
		Organizations: items,
	}))
}

func (h *AuthHandler) switchOrganization(c *echo.Context) error {
	var r struct {
		OrgID string `json:"org_id"`
	}
	if err := bind(c, &r); err != nil {
		return err
	}
	ctx := c.Request().Context()
	user := middleware.CurrentUser(c)
	token, org, err := h.service.SwitchOrganization(ctx, user, r.OrgID)
	if err != nil {
		return respondError(c, http.StatusForbidden, service.ErrNotMember.Error())
	}
	membership, _ := h.service.Orgs.Membership(ctx, r.OrgID, string(user.ID))
	role := ""
	if membership != nil {
		role = membership.Role
	}
	return respondJSON(c, http.StatusOK, dto.OK(dto.SwitchOrganizationResponse{Token: token, Organization: dto.NewOrganizationResponse(org, role)}))
}

func (h *AuthHandler) previewInvitation(c *echo.Context) error {
	preview, err := h.service.Orgs.PreviewInvitation(c.Request().Context(), c.Param("token"))
	if err != nil {
		return respondError(c, http.StatusNotFound, service.ErrInvitationInvalid.Error())
	}
	return respondJSON(c, http.StatusOK, dto.OK(preview))
}

// acceptInvitation lets someone who already has an account join an organization.
func (h *AuthHandler) acceptInvitation(c *echo.Context) error {
	var r struct {
		Token string `json:"token"`
	}
	if err := bind(c, &r); err != nil {
		return err
	}
	ctx := c.Request().Context()
	user := middleware.CurrentUser(c)
	org, err := h.service.Orgs.AcceptInvitation(ctx, r.Token, user)
	if err != nil {
		return respondError(c, http.StatusBadRequest, err.Error())
	}
	token, _, err := h.service.SwitchOrganization(ctx, user, string(org.ID))
	if err != nil {
		return respondError(c, http.StatusInternalServerError, "failed to switch organization")
	}
	membership, _ := h.service.Orgs.Membership(ctx, string(org.ID), string(user.ID))
	role := ""
	if membership != nil {
		role = membership.Role
	}
	return respondJSON(c, http.StatusOK, dto.OK(dto.SwitchOrganizationResponse{Token: token, Organization: dto.NewOrganizationResponse(org, role)}))
}

func (h *AuthHandler) users(c *echo.Context) error {
	users, err := h.service.ListUsers(c.Request().Context(), middleware.CurrentOrgID(c))
	if err != nil {
		return respondError(c, http.StatusInternalServerError, "failed to list users")
	}
	items := make([]dto.UserResponse, 0, len(users))
	for i := range users {
		items = append(items, dto.NewUserResponse(&users[i]))
	}
	return respondJSON(c, http.StatusOK, dto.List(items, int64(len(items)), 1, len(items)))
}
