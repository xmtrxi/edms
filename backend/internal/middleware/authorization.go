package middleware

import (
	"net/http"

	"github.com/labstack/echo/v5"

	"github.com/xmtrxi/edms/internal/database"
	"github.com/xmtrxi/edms/internal/service"
)

// RequirePermission authorizes an authenticated request inside the current
// organization. Owners and admins may do everything; guests only what their
// role grants. Resource services additionally check folder and file access.
func RequirePermission(authz *service.AuthorizationService, object, action string) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			user := CurrentUser(c)
			if user == nil {
				return echo.NewHTTPError(http.StatusUnauthorized, "unauthorized")
			}
			if IsOrgAdmin(c) {
				return next(c)
			}
			if authz == nil || !authz.Can(string(user.ID), CurrentOrgID(c), object, action) {
				return echo.NewHTTPError(http.StatusForbidden, "access denied")
			}
			return next(c)
		}
	}
}

// RequireOrgRole allows only members holding one of the given org roles.
func RequireOrgRole(roles ...string) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			membership := CurrentMembership(c)
			if membership == nil {
				return echo.NewHTTPError(http.StatusForbidden, "access denied")
			}
			for _, role := range roles {
				if membership.Role == role {
					return next(c)
				}
			}
			return echo.NewHTTPError(http.StatusForbidden, "access denied")
		}
	}
}

// RequireOrgAdmin allows owners and admins of the current organization.
func RequireOrgAdmin() echo.MiddlewareFunc {
	return RequireOrgRole(database.MembershipRoleOwner, database.MembershipRoleAdmin)
}
