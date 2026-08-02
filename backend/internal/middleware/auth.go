package middleware

import (
	"net/http"
	"strings"

	"github.com/labstack/echo/v5"

	"github.com/xmtrxi/edms/internal/database"
	"github.com/xmtrxi/edms/internal/service"
)

const userKey = "edms.user"

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

			userID, err := auth.ParseToken(strings.TrimPrefix(header, "Bearer "))
			if err != nil || userID == "" {
				return echo.NewHTTPError(http.StatusUnauthorized, "invalid token")
			}

			user, err := auth.UserByID(c.Request().Context(), userID)
			if err != nil || user == nil {
				return echo.NewHTTPError(http.StatusUnauthorized, "invalid token")
			}

			c.Set(userKey, user)
			return next(c)
		}
	}
}

func CurrentUser(c *echo.Context) *database.User {
	user, _ := c.Get(userKey).(*database.User)
	return user
}
