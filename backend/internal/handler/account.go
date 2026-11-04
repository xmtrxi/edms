package handler

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/labstack/echo/v5"

	"github.com/xmtrxi/edms/internal/dto"
	"github.com/xmtrxi/edms/internal/middleware"
	"github.com/xmtrxi/edms/internal/observability"
	"github.com/xmtrxi/edms/internal/service"
)

// Endpoints for the signed-in person's own account: profile, password,
// sessions and two-step sign-in.

func (h *AuthHandler) updateProfile(c *echo.Context) error {
	var r struct {
		Name string `json:"name"`
	}
	if err := bind(c, &r); err != nil {
		return err
	}
	user, err := h.service.UpdateProfile(c.Request().Context(), middleware.CurrentUser(c), r.Name)
	if err != nil {
		return respondError(c, http.StatusBadRequest, err.Error())
	}
	return respondJSON(c, http.StatusOK, dto.OK(dto.NewUserResponse(user)))
}

func (h *AuthHandler) logout(c *echo.Context) error {
	sid := middleware.CurrentSessionID(c)
	if sid == "" {
		return respondError(c, http.StatusBadRequest, "personal access tokens are revoked from settings")
	}
	if err := h.service.Sessions.Revoke(c.Request().Context(), string(middleware.CurrentUser(c).ID), sid); err != nil {
		return respondError(c, http.StatusInternalServerError, "failed to sign out")
	}
	return c.NoContent(http.StatusNoContent)
}

type sessionResponse struct {
	ID         string    `json:"id"`
	Method     string    `json:"method"`
	IP         string    `json:"ip,omitempty"`
	UserAgent  string    `json:"user_agent,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
	LastSeenAt time.Time `json:"last_seen_at"`
	ExpiresAt  time.Time `json:"expires_at"`
	Current    bool      `json:"current"`
}

func (h *AuthHandler) sessions(c *echo.Context) error {
	items, err := h.service.Sessions.List(c.Request().Context(), string(middleware.CurrentUser(c).ID))
	if err != nil {
		return respondError(c, http.StatusInternalServerError, "failed to list sessions")
	}
	current := middleware.CurrentSessionID(c)
	out := make([]sessionResponse, 0, len(items))
	for _, s := range items {
		out = append(out, sessionResponse{ID: string(s.ID), Method: s.Method, IP: s.IP, UserAgent: s.UserAgent,
			CreatedAt: s.CreatedAt, LastSeenAt: s.LastSeenAt, ExpiresAt: s.ExpiresAt, Current: string(s.ID) == current})
	}
	return respondJSON(c, http.StatusOK, dto.List(out, int64(len(out)), 1, len(out)))
}

func (h *AuthHandler) revokeSession(c *echo.Context) error {
	if err := h.service.Sessions.Revoke(c.Request().Context(), string(middleware.CurrentUser(c).ID), c.Param("id")); err != nil {
		return respondError(c, http.StatusNotFound, "session not found")
	}
	return c.NoContent(http.StatusNoContent)
}

func (h *AuthHandler) revokeOtherSessions(c *echo.Context) error {
	sid := middleware.CurrentSessionID(c)
	if sid == "" {
		return respondError(c, http.StatusForbidden, "sign in to manage sessions")
	}
	if err := h.service.Sessions.RevokeAll(c.Request().Context(), string(middleware.CurrentUser(c).ID), sid); err != nil {
		return respondError(c, http.StatusInternalServerError, "failed to sign out other sessions")
	}
	return c.NoContent(http.StatusNoContent)
}

func (h *AuthHandler) changePassword(c *echo.Context) error {
	if err := h.requireSession(c); err != nil {
		return err
	}
	var r struct {
		CurrentPassword string `json:"current_password"`
		NewPassword     string `json:"new_password"`
	}
	if err := bind(c, &r); err != nil {
		return err
	}
	user := middleware.CurrentUser(c)
	key := "password-change:" + string(user.ID)
	if err := h.checkLimit(c, key, 5); err != nil {
		return err
	}
	err := h.service.ChangePassword(c.Request().Context(), user, middleware.CurrentSessionID(c), r.CurrentPassword, r.NewPassword)
	switch {
	case err == nil:
		return c.NoContent(http.StatusNoContent)
	case errors.Is(err, service.ErrInvalidCredentials):
		h.limiter.Hit(c.Request().Context(), key, 15*time.Minute)
		return respondError(c, http.StatusBadRequest, "current password is incorrect")
	case errors.Is(err, service.ErrWeakPassword):
		return respondError(c, http.StatusBadRequest, err.Error())
	}
	return respondError(c, http.StatusInternalServerError, "failed to change password")
}

func (h *AuthHandler) sendVerification(c *echo.Context) error {
	user := middleware.CurrentUser(c)
	if err := h.throttle(c, "verify-send:user:"+string(user.ID), 3, time.Hour); err != nil {
		return err
	}
	if err := h.service.SendVerification(c.Request().Context(), user); err != nil {
		return respondError(c, http.StatusInternalServerError, "failed to send verification email")
	}
	return c.NoContent(http.StatusAccepted)
}

func (h *AuthHandler) mfaSetup(c *echo.Context) error {
	if err := h.requireSession(c); err != nil {
		return err
	}
	setup, err := h.service.BeginMFA(c.Request().Context(), middleware.CurrentUser(c))
	if err != nil {
		return respondError(c, http.StatusBadRequest, err.Error())
	}
	return respondJSON(c, http.StatusOK, dto.OK(setup))
}

type mfaCodeRequest struct {
	Code     string `json:"code"`
	Password string `json:"password"`
}

type recoveryCodesResponse struct {
	RecoveryCodes []string `json:"recovery_codes"`
}

func (h *AuthHandler) mfaEnable(c *echo.Context) error {
	if err := h.requireSession(c); err != nil {
		return err
	}
	var r mfaCodeRequest
	if err := bind(c, &r); err != nil {
		return err
	}
	user := middleware.CurrentUser(c)
	key := "mfa-manage:" + string(user.ID)
	if err := h.checkLimit(c, key, 10); err != nil {
		return err
	}
	codes, err := h.service.EnableMFA(c.Request().Context(), user, middleware.CurrentSessionID(c), r.Code)
	if err != nil {
		h.limiter.Hit(c.Request().Context(), key, 15*time.Minute)
		return respondError(c, http.StatusBadRequest, err.Error())
	}
	return respondJSON(c, http.StatusOK, dto.OK(recoveryCodesResponse{RecoveryCodes: codes}))
}

func (h *AuthHandler) mfaDisable(c *echo.Context) error {
	if err := h.requireSession(c); err != nil {
		return err
	}
	var r mfaCodeRequest
	if err := bind(c, &r); err != nil {
		return err
	}
	user := middleware.CurrentUser(c)
	key := "mfa-manage:" + string(user.ID)
	if err := h.checkLimit(c, key, 10); err != nil {
		return err
	}
	if err := h.service.DisableMFA(c.Request().Context(), user, r.Password, r.Code); err != nil {
		h.limiter.Hit(c.Request().Context(), key, 15*time.Minute)
		if errors.Is(err, service.ErrInvalidCredentials) {
			return respondError(c, http.StatusBadRequest, "password is incorrect")
		}
		return respondError(c, http.StatusBadRequest, err.Error())
	}
	return c.NoContent(http.StatusNoContent)
}

func (h *AuthHandler) mfaRecoveryCodes(c *echo.Context) error {
	if err := h.requireSession(c); err != nil {
		return err
	}
	var r mfaCodeRequest
	if err := bind(c, &r); err != nil {
		return err
	}
	user := middleware.CurrentUser(c)
	key := "mfa-manage:" + string(user.ID)
	if err := h.checkLimit(c, key, 10); err != nil {
		return err
	}
	codes, err := h.service.RegenerateRecoveryCodes(c.Request().Context(), user, r.Code)
	if err != nil {
		h.limiter.Hit(c.Request().Context(), key, 15*time.Minute)
		return respondError(c, http.StatusBadRequest, err.Error())
	}
	return respondJSON(c, http.StatusOK, dto.OK(recoveryCodesResponse{RecoveryCodes: codes}))
}

// requireSession refuses personal access tokens for account security changes.
func (h *AuthHandler) requireSession(c *echo.Context) error {
	if middleware.CurrentSessionID(c) == "" {
		return echo.NewHTTPError(http.StatusForbidden, "sign in to change account security")
	}
	return nil
}

// checkLimit refuses the request when the key has used up its attempts.
func (h *AuthHandler) checkLimit(c *echo.Context, key string, max int) error {
	if max <= 0 {
		return nil
	}
	if h.limiter.Count(c.Request().Context(), key) >= int64(max) {
		return tooManyAttempts(c, h.cfg.Security.LoginWindow)
	}
	return nil
}

// throttle counts this request against the key and refuses it past max.
func (h *AuthHandler) throttle(c *echo.Context, key string, max int, window time.Duration) error {
	if h.limiter.Hit(c.Request().Context(), key, window) > int64(max) {
		return tooManyAttempts(c, window)
	}
	return nil
}

func tooManyAttempts(c *echo.Context, retry time.Duration) error {
	c.Response().Header().Set("Retry-After", strconv.Itoa(int(retry.Seconds())))
	return echo.NewHTTPError(http.StatusTooManyRequests, "too many attempts; try again later")
}

func (h *AuthHandler) log(c *echo.Context) *slog.Logger {
	return slog.Default().With("path", c.Request().URL.Path)
}

func sessionMeta(c *echo.Context) service.SessionMeta {
	return service.SessionMeta{IP: c.RealIP(), UserAgent: c.Request().UserAgent()}
}

func shortHash(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:12])
}

const ssoCookie = "edms_sso"

// ssoStart sends the browser to the identity provider.
func (h *AuthHandler) ssoStart(c *echo.Context) error {
	if err := h.throttle(c, "sso:ip:"+c.RealIP(), 60, time.Hour); err != nil {
		return err
	}
	start, err := h.sso.Start(c.Request().Context(), c.QueryParam("return"))
	if err != nil {
		h.log(c).Warn("start single sign-on", "error", err)
		return c.Redirect(http.StatusFound, h.frontendURL("/auth/login?sso_error="+url.QueryEscape("Single sign-on is unavailable right now.")))
	}
	c.SetCookie(&http.Cookie{
		Name: ssoCookie, Value: start.State, Path: "/api/auth/sso", MaxAge: 600,
		HttpOnly: true, Secure: strings.HasPrefix(h.cfg.SSO.RedirectURL, "https://"), SameSite: http.SameSiteLaxMode,
	})
	return c.Redirect(http.StatusFound, start.URL)
}

// ssoCallback finishes sign-in and hands the token to the frontend in the
// URL fragment, which browsers never send to servers or put in logs.
func (h *AuthHandler) ssoCallback(c *echo.Context) error {
	c.SetCookie(&http.Cookie{Name: ssoCookie, Value: "", Path: "/api/auth/sso", MaxAge: -1, HttpOnly: true})
	fail := func(message string) error {
		return c.Redirect(http.StatusFound, h.frontendURL("/auth/login?sso_error="+url.QueryEscape(message)))
	}
	if errParam := c.QueryParam("error"); errParam != "" {
		return fail(firstNonEmptyString(c.QueryParam("error_description"), "Sign-in was cancelled."))
	}
	cookie, err := c.Cookie(ssoCookie)
	if err != nil {
		return fail(service.ErrSSOState.Error())
	}
	result, err := h.sso.Finish(c.Request().Context(), cookie.Value, c.QueryParam("state"), c.QueryParam("code"), sessionMeta(c))
	if err != nil {
		h.log(c).Warn("single sign-on", "error", err)
		observability.Logins.WithLabelValues("sso", "failed").Inc()
		switch {
		case errors.Is(err, service.ErrSSOState), errors.Is(err, service.ErrSSONotPermitted), errors.Is(err, service.ErrNoOrganization):
			return fail(err.Error())
		}
		return fail("Single sign-on failed. Try again or contact your administrator.")
	}
	observability.Logins.WithLabelValues("sso", "success").Inc()
	return c.Redirect(http.StatusFound, h.frontendURL("/auth/sso#token="+url.QueryEscape(result.Token)+"&return="+url.QueryEscape(result.ReturnTo)))
}

func (h *AuthHandler) frontendURL(path string) string {
	base := strings.TrimRight(h.cfg.SMTP.AppURL, "/")
	if base == "" {
		base = "http://localhost:3000"
	}
	return base + path
}
