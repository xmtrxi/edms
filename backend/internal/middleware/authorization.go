package middleware

import (
	"net/http"

	"github.com/labstack/echo/v5"
	"github.com/xmtrxi/edms/internal/service"
)

// RequirePermission authorizes an authenticated request against the org domain.
// Resource services additionally check ownership and resource domains.
func RequirePermission(authz *service.AuthorizationService, object, action string) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			user := CurrentUser(c)
			if user == nil {
				return echo.NewHTTPError(http.StatusUnauthorized, "unauthorized")
			}
			if authz == nil || !authz.Can(string(user.ID), service.OrganizationDomain, object, action) {
				return echo.NewHTTPError(http.StatusForbidden, "access denied")
			}
			return next(c)
		}
	}
}
