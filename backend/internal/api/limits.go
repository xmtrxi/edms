package api

import (
	"net/http"
	"strings"

	"github.com/labstack/echo/v5"

	"github.com/xmtrxi/edms/internal/config"
	"github.com/xmtrxi/edms/internal/handler"
)

// bodyLimit caps request bodies. File uploads, upload chunks and WebDAV
// writes may be as large as an upload; everything else is small JSON.
func bodyLimit(cfg config.Config) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			req := c.Request()
			if req.Body == nil || req.Body == http.NoBody {
				return next(c)
			}
			limit := cfg.Security.BodyLimit
			if carriesFile(req) {
				limit = cfg.Uploads.MaxBytes
			}
			if limit <= 0 {
				return next(c)
			}
			if req.ContentLength > limit {
				return echo.NewHTTPError(http.StatusRequestEntityTooLarge, "request body is too large")
			}
			req.Body = http.MaxBytesReader(c.Response(), req.Body, limit)
			return next(c)
		}
	}
}

func carriesFile(req *http.Request) bool {
	path := req.URL.Path
	if path == handler.WebDAVPrefix || strings.HasPrefix(path, handler.WebDAVPrefix+"/") {
		return true
	}
	if req.Method == http.MethodPut && strings.Contains(path, "/uploads/") {
		return true
	}
	return strings.HasPrefix(req.Header.Get(echo.HeaderContentType), echo.MIMEMultipartForm)
}
