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
			if header == "" || !strings.HasPrefix(header, "Bearer ") {
				return echo.NewHTTPError(http.StatusUnauthorized, "missing bearer token")
			}

			claims, err := auth.ParseToken(strings.TrimPrefix(header, "Bearer "))
			if err != nil || claims.UserID == "" {
				return echo.NewHTTPError(http.StatusUnauthorized, "invalid token")
			}

			ctx := c.Request().Context()
			user, err := auth.UserByID(ctx, claims.UserID)
			if err != nil || user == nil || user.Status != database.UserStatusActive {
				return echo.NewHTTPError(http.StatusUnauthorized, "invalid token")
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
