package middleware

import (
	"errors"
	"net/http"
	"strings"

	"github.com/labstack/echo/v5"

	"github.com/xmtrxi/edms/internal/database"
	"github.com/xmtrxi/edms/internal/service"
)

const (
	userKey       = "edms.user"
	orgKey        = "edms.org"
	membershipKey = "edms.membership"
	tokenKey      = "edms.api_token"
)

// RequireAuth authenticates the bearer token and resolves the organization the
// request acts in. A token scoped to an organization the user has left falls
// back to another of their organizations, so removal takes effect at once.
func RequireAuth(auth *service.AuthService) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			if auth == nil {
				return echo.NewHTTPError(http.StatusUnauthorized, "auth is not configured")
			}

			header := c.Request().Header.Get(echo.HeaderAuthorization)
			bearer, ok := strings.CutPrefix(header, "Bearer ")
			if !ok {
				// Browsers can't set headers on WebSockets, so the token rides
				// in the subprotocol list instead of the URL, which gets logged.
				bearer, ok = websocketToken(c.Request())
			}
			if !ok || bearer == "" {
				return echo.NewHTTPError(http.StatusUnauthorized, "missing bearer token")
			}

			ctx := c.Request().Context()
			var claims service.TokenClaims
			var user *database.User
			if strings.HasPrefix(bearer, service.APITokenPrefix) {
				// Personal access token: fixed organization, limited scopes.
				token, tokenUser, err := auth.Tokens.Authenticate(ctx, bearer)
				if err != nil {
					return echo.NewHTTPError(http.StatusUnauthorized, "invalid token")
				}
				// MCP sends every call as a POST and checks write scope per tool.
				if !safeMethod(c.Request().Method) && !service.HasScope(token, service.ScopeWrite) && !strings.HasSuffix(c.Request().URL.Path, "/mcp") {
					return echo.NewHTTPError(http.StatusForbidden, "this token is read-only")
				}
				claims, user = service.TokenClaims{UserID: string(tokenUser.ID), OrgID: token.OrgID}, tokenUser
				c.Set(tokenKey, token)
			} else {
				parsed, err := auth.ParseToken(bearer)
				if err != nil || parsed.UserID == "" {
					return echo.NewHTTPError(http.StatusUnauthorized, "invalid token")
				}
				claims = parsed
				user, err = auth.UserByID(ctx, claims.UserID)
				if err != nil || user == nil || user.Status != database.UserStatusActive {
					return echo.NewHTTPError(http.StatusUnauthorized, "invalid token")
				}
			}

			org, membership, err := auth.Orgs.Resolve(ctx, user, claims.OrgID)
			if err != nil {
				if errors.Is(err, service.ErrNoOrganization) {
					return echo.NewHTTPError(http.StatusForbidden, err.Error())
				}
				return echo.NewHTTPError(http.StatusInternalServerError, "failed to resolve organization")
			}

			c.Set(userKey, user)
			c.Set(orgKey, org)
			c.Set(membershipKey, membership)
			c.SetRequest(c.Request().WithContext(service.WithActor(ctx, service.Actor{
				UserID: string(user.ID), OrgID: string(org.ID), OrgRole: membership.Role, Email: user.Email,
			})))
			return next(c)
		}
	}
}

func CurrentUser(c *echo.Context) *database.User {
	user, _ := c.Get(userKey).(*database.User)
	return user
}

// CurrentOrg is the organization the request acts in.
func CurrentOrg(c *echo.Context) *database.Organization {
	org, _ := c.Get(orgKey).(*database.Organization)
	return org
}

// CurrentOrgID is a convenience for services that scope queries by organization.
func CurrentOrgID(c *echo.Context) string {
	if org := CurrentOrg(c); org != nil {
		return string(org.ID)
	}
	return ""
}

// CurrentMembership is the signed-in user's membership in CurrentOrg.
func CurrentMembership(c *echo.Context) *database.Membership {
	membership, _ := c.Get(membershipKey).(*database.Membership)
	return membership
}

// IsOrgAdmin reports whether the current member is an owner or admin.
func IsOrgAdmin(c *echo.Context) bool {
	membership := CurrentMembership(c)
	return membership != nil && (membership.Role == database.MembershipRoleOwner || membership.Role == database.MembershipRoleAdmin)
}

// CurrentAPIToken is the personal access token used for this request, if any.
func CurrentAPIToken(c *echo.Context) *database.APIToken {
	token, _ := c.Get(tokenKey).(*database.APIToken)
	return token
}

// WebSocketProtocol is offered by browsers as the first subprotocol, followed
// by their access token.
const WebSocketProtocol = "edms.v1"

func websocketToken(r *http.Request) (string, bool) {
	if !strings.EqualFold(r.Header.Get("Upgrade"), "websocket") {
		return "", false
	}
	var protocols []string
	for _, value := range r.Header.Values("Sec-WebSocket-Protocol") {
		for _, part := range strings.Split(value, ",") {
			protocols = append(protocols, strings.TrimSpace(part))
		}
	}
	if len(protocols) < 2 || protocols[0] != WebSocketProtocol {
		return "", false
	}
	return protocols[1], true
}

func safeMethod(method string) bool {
	switch method {
	case http.MethodGet, http.MethodHead, http.MethodOptions, "PROPFIND":
		return true
	}
	return false
}
