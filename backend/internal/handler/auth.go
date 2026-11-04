package handler

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/labstack/echo/v5"

	"github.com/xmtrxi/edms/internal/config"
	"github.com/xmtrxi/edms/internal/database"
	"github.com/xmtrxi/edms/internal/deployment"
	"github.com/xmtrxi/edms/internal/dto"
	"github.com/xmtrxi/edms/internal/middleware"
	"github.com/xmtrxi/edms/internal/observability"
	"github.com/xmtrxi/edms/internal/service"
)

type AuthHandler struct {
	service *service.AuthService
	limiter *service.Limiter
	cfg     config.Config
	sso     *service.SSOService
}

func NewAuthHandler(deps Deps) *AuthHandler {
	auth := service.NewAuthService(deps.Cfg, deps.Client(), deps.Logger)
	auth.Mail = service.NewOperationsService(deps.Client(), deps.Cfg.SMTP, deps.Queue)
	return &AuthHandler{service: auth, limiter: service.NewLimiter(deps.Cache), cfg: deps.Cfg, sso: service.NewSSOService(deps.Cfg, auth)}
}

func (h *AuthHandler) RegisterRoutes(api *echo.Group) {
	api.GET("/meta", h.meta)
	api.POST("/auth/register", h.register)
	api.POST("/auth/setup", h.setup)
	api.POST("/auth/login", h.login)
	api.POST("/auth/login/mfa", h.loginMFA)
	api.POST("/auth/password/forgot", h.forgotPassword)
	api.POST("/auth/password/reset", h.resetPassword)
	api.POST("/auth/email/verify", h.verifyEmail)
	api.POST("/auth/email/resend", h.resendVerification)
	api.GET("/auth/sso/start", h.ssoStart)
	api.GET("/auth/sso/callback", h.ssoCallback)
	api.GET("/invitations/:token", h.previewInvitation)

	authed := api.Group("", middleware.RequireAuth(h.service))
	authed.GET("/auth/me", h.me)
	authed.PATCH("/auth/me", h.updateProfile)
	authed.GET("/auth/session", h.session)
	authed.POST("/auth/logout", h.logout)
	authed.GET("/auth/sessions", h.sessions)
	authed.DELETE("/auth/sessions/:id", h.revokeSession)
	authed.POST("/auth/sessions/revoke-others", h.revokeOtherSessions)
	authed.POST("/auth/password", h.changePassword)
	authed.POST("/auth/email/send-verification", h.sendVerification)
	authed.POST("/auth/mfa/setup", h.mfaSetup)
	authed.POST("/auth/mfa/enable", h.mfaEnable)
	authed.POST("/auth/mfa/disable", h.mfaDisable)
	authed.POST("/auth/mfa/recovery-codes", h.mfaRecoveryCodes)
	authed.POST("/auth/switch-org", h.switchOrganization)
	authed.POST("/invitations/accept", h.acceptInvitation)
	authed.GET("/users", h.users)
	authed.GET("/tokens", h.listTokens)
	authed.POST("/tokens", h.createToken)
	authed.DELETE("/tokens/:id", h.revokeToken)
}

// meta describes the deployment so the frontend can show or hide signup,
// organization creation and the workspace switcher. It is public.
func (h *AuthHandler) meta(c *echo.Context) error {
	return respondJSON(c, http.StatusOK, dto.OK(metaResponse{
		Meta:                 h.service.Orgs.Capabilities().Meta(),
		AppName:              h.cfg.App.Name,
		Version:              h.cfg.App.Version,
		SSO:                  h.sso.Enabled(),
		SSOLabel:             h.sso.Label(),
		PasswordLogin:        !h.sso.Enforced(),
		RequireVerifiedEmail: h.cfg.Deployment.RequireVerifiedEmail && h.cfg.SMTP.Host != "",
	}))
}

type metaResponse struct {
	deployment.Meta
	AppName              string `json:"app_name"`
	Version              string `json:"version"`
	SSO                  bool   `json:"sso"`
	SSOLabel             string `json:"sso_label,omitempty"`
	PasswordLogin        bool   `json:"password_login"`
	RequireVerifiedEmail bool   `json:"require_verified_email"`
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
	user, token, err := h.service.AcceptInvitation(ctx, r.Token, r.Name, r.Password, sessionMeta(c))
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
	if err := h.throttle(c, "register:ip:"+c.RealIP(), 10, time.Hour); err != nil {
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
	if h.service.VerificationRequired(user) {
		return respondJSON(c, http.StatusCreated, dto.Created(dto.AuthResponse{User: dto.NewUserResponse(user), VerificationRequired: true}))
	}
	meta := sessionMeta(c)
	meta.Method = database.SessionMethodSignup
	token, err := h.service.StartSession(ctx, user, user.LastOrgID, meta)
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
	if h.sso.Enforced() {
		return respondError(c, http.StatusForbidden, "sign in with "+h.sso.Label())
	}
	email := strings.ToLower(strings.TrimSpace(req.Email))
	ctx := c.Request().Context()
	emailKey, ipKey := "login:email:"+email, "login:ip:"+c.RealIP()
	if err := h.checkLimit(c, emailKey, h.cfg.Security.LoginAttempts); err != nil {
		return err
	}
	// Offices often share one address, so the IP allowance is wider.
	if err := h.checkLimit(c, ipKey, h.cfg.Security.LoginAttempts*5); err != nil {
		return err
	}

	result, err := h.service.Login(ctx, service.LoginInput{Email: email, Password: req.Password, Meta: sessionMeta(c)})
	if err != nil {
		switch {
		case errors.Is(err, service.ErrNoOrganization):
			return respondError(c, http.StatusForbidden, err.Error())
		case errors.Is(err, service.ErrEmailNotVerified):
			return respondJSON(c, http.StatusForbidden, map[string]any{"error": err.Error(), "code": "email_not_verified"})
		case errors.Is(err, service.ErrInvalidCredentials):
			observability.Logins.WithLabelValues("password", "invalid").Inc()
			h.limiter.Hit(ctx, emailKey, h.cfg.Security.LoginWindow)
			h.limiter.Hit(ctx, ipKey, h.cfg.Security.LoginWindow)
			return respondError(c, http.StatusUnauthorized, "invalid credentials")
		}
		return respondError(c, http.StatusInternalServerError, "failed to sign in")
	}
	h.limiter.Reset(ctx, emailKey)
	if result.MFAToken != "" {
		observability.Logins.WithLabelValues("password", "mfa_required").Inc()
		return respondJSON(c, http.StatusOK, dto.OK(dto.AuthResponse{MFARequired: true, MFAToken: result.MFAToken}))
	}
	observability.Logins.WithLabelValues("password", "success").Inc()
	return h.respondAuth(c, http.StatusOK, result.User, result.Token)
}

func (h *AuthHandler) loginMFA(c *echo.Context) error {
	var r struct {
		MFAToken string `json:"mfa_token"`
		Code     string `json:"code"`
	}
	if err := bind(c, &r); err != nil {
		return err
	}
	ctx := c.Request().Context()
	// Each challenge allows a few guesses; the IP allowance covers retries.
	tokenKey, ipKey := "mfa:token:"+shortHash(r.MFAToken), "mfa:ip:"+c.RealIP()
	if err := h.checkLimit(c, tokenKey, 5); err != nil {
		return err
	}
	if err := h.checkLimit(c, ipKey, h.cfg.Security.LoginAttempts*5); err != nil {
		return err
	}
	user, token, err := h.service.CompleteMFA(ctx, r.MFAToken, r.Code, sessionMeta(c))
	if err != nil {
		h.limiter.Hit(ctx, tokenKey, 10*time.Minute)
		h.limiter.Hit(ctx, ipKey, h.cfg.Security.LoginWindow)
		if errors.Is(err, service.ErrInvalidLink) {
			return respondError(c, http.StatusUnauthorized, "sign-in expired; enter your password again")
		}
		if errors.Is(err, service.ErrNoOrganization) {
			return respondError(c, http.StatusForbidden, err.Error())
		}
		observability.Logins.WithLabelValues("mfa", "invalid").Inc()
		return respondError(c, http.StatusUnauthorized, service.ErrInvalidMFACode.Error())
	}
	observability.Logins.WithLabelValues("mfa", "success").Inc()
	return h.respondAuth(c, http.StatusOK, user, token)
}

func (h *AuthHandler) forgotPassword(c *echo.Context) error {
	var r struct {
		Email string `json:"email"`
	}
	if err := bind(c, &r); err != nil {
		return err
	}
	email := strings.ToLower(strings.TrimSpace(r.Email))
	if err := h.throttle(c, "reset:email:"+email, 3, time.Hour); err != nil {
		return err
	}
	if err := h.throttle(c, "reset:ip:"+c.RealIP(), 20, time.Hour); err != nil {
		return err
	}
	if err := h.service.RequestPasswordReset(c.Request().Context(), email); err != nil {
		h.log(c).Warn("password reset request", "error", err)
	}
	// The same answer whether or not the address has an account.
	return respondJSON(c, http.StatusAccepted, dto.OK(map[string]string{"status": "If an account uses that address, a reset link is on its way."}))
}

func (h *AuthHandler) resetPassword(c *echo.Context) error {
	var r struct {
		Token    string `json:"token"`
		Password string `json:"password"`
	}
	if err := bind(c, &r); err != nil {
		return err
	}
	if err := h.throttle(c, "reset-confirm:ip:"+c.RealIP(), 20, time.Hour); err != nil {
		return err
	}
	if err := h.service.ResetPassword(c.Request().Context(), r.Token, r.Password); err != nil {
		if errors.Is(err, service.ErrWeakPassword) || errors.Is(err, service.ErrInvalidLink) {
			return respondError(c, http.StatusBadRequest, err.Error())
		}
		return respondError(c, http.StatusInternalServerError, "failed to reset password")
	}
	return c.NoContent(http.StatusNoContent)
}

func (h *AuthHandler) verifyEmail(c *echo.Context) error {
	var r struct {
		Token string `json:"token"`
	}
	if err := bind(c, &r); err != nil {
		return err
	}
	if err := h.throttle(c, "verify:ip:"+c.RealIP(), 30, time.Hour); err != nil {
		return err
	}
	user, err := h.service.VerifyEmail(c.Request().Context(), r.Token)
	if err != nil {
		if errors.Is(err, service.ErrInvalidLink) {
			return respondError(c, http.StatusBadRequest, err.Error())
		}
		return respondError(c, http.StatusInternalServerError, "failed to confirm email")
	}
	return respondJSON(c, http.StatusOK, dto.OK(dto.NewUserResponse(user)))
}

func (h *AuthHandler) resendVerification(c *echo.Context) error {
	var r struct {
		Email string `json:"email"`
	}
	if err := bind(c, &r); err != nil {
		return err
	}
	email := strings.ToLower(strings.TrimSpace(r.Email))
	if err := h.throttle(c, "verify-send:email:"+email, 3, time.Hour); err != nil {
		return err
	}
	if err := h.throttle(c, "verify-send:ip:"+c.RealIP(), 20, time.Hour); err != nil {
		return err
	}
	if err := h.service.ResendVerification(c.Request().Context(), email); err != nil {
		h.log(c).Warn("resend verification", "error", err)
	}
	return respondJSON(c, http.StatusAccepted, dto.OK(map[string]string{"status": "If that address needs confirming, a new link is on its way."}))
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
	token, org, err := h.service.SwitchOrganization(ctx, user, middleware.CurrentSessionID(c), r.OrgID)
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
	if middleware.CurrentSessionID(c) == "" {
		return respondJSON(c, http.StatusOK, dto.OK(dto.SwitchOrganizationResponse{Organization: dto.NewOrganizationResponse(org, "")}))
	}
	token, _, err := h.service.SwitchOrganization(ctx, user, middleware.CurrentSessionID(c), string(org.ID))
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

type tokenResponse struct {
	ID         string    `json:"id"`
	Name       string    `json:"name"`
	Prefix     string    `json:"prefix"`
	Scopes     []string  `json:"scopes"`
	LastUsedAt time.Time `json:"last_used_at,omitempty"`
	ExpiresAt  time.Time `json:"expires_at,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
	// Secret is only returned when the token is created.
	Secret string `json:"secret,omitempty"`
}

func newTokenResponse(token *database.APIToken, secret string) tokenResponse {
	return tokenResponse{ID: string(token.ID), Name: token.Name, Prefix: token.Prefix, Scopes: token.Scopes,
		LastUsedAt: token.LastUsedAt, ExpiresAt: token.ExpiresAt, CreatedAt: token.CreatedAt, Secret: secret}
}

func (h *AuthHandler) listTokens(c *echo.Context) error {
	tokens, err := h.service.Tokens.List(c.Request().Context(), string(middleware.CurrentUser(c).ID), middleware.CurrentOrgID(c))
	if err != nil {
		return respondError(c, http.StatusInternalServerError, "failed to list tokens")
	}
	items := make([]tokenResponse, 0, len(tokens))
	for i := range tokens {
		items = append(items, newTokenResponse(&tokens[i], ""))
	}
	return respondJSON(c, http.StatusOK, dto.List(items, int64(len(items)), 1, len(items)))
}

// createToken needs a signed-in session: a token cannot mint more tokens.
func (h *AuthHandler) createToken(c *echo.Context) error {
	if middleware.CurrentAPIToken(c) != nil {
		return respondError(c, http.StatusForbidden, "sign in to create tokens")
	}
	var r service.TokenInput
	if err := bind(c, &r); err != nil {
		return err
	}
	token, secret, err := h.service.Tokens.Create(c.Request().Context(), string(middleware.CurrentUser(c).ID), middleware.CurrentOrgID(c), r)
	if err != nil {
		return respondError(c, http.StatusBadRequest, err.Error())
	}
	return respondJSON(c, http.StatusCreated, dto.Created(newTokenResponse(token, secret)))
}

func (h *AuthHandler) revokeToken(c *echo.Context) error {
	if err := h.service.Tokens.Revoke(c.Request().Context(), string(middleware.CurrentUser(c).ID), c.Param("id")); err != nil {
		return respondError(c, http.StatusNotFound, "token not found")
	}
	return c.NoContent(http.StatusNoContent)
}
