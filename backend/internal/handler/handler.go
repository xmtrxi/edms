package handler

import (
	"log/slog"
	"net/http"
	"strconv"

	"github.com/labstack/echo/v5"
	"github.com/surrealdb/surrealdb.go"

	"github.com/xmtrxi/edms/internal/cache"
	"github.com/xmtrxi/edms/internal/config"
	"github.com/xmtrxi/edms/internal/database"
	"github.com/xmtrxi/edms/internal/paperless"
	"github.com/xmtrxi/edms/internal/queue"
	"github.com/xmtrxi/edms/internal/search"
	"github.com/xmtrxi/edms/internal/storage"
)

type Deps struct {
	Cfg       config.Config
	Logger    *slog.Logger
	DB        *database.DB
	Cache     *cache.Cache
	Storage   storage.Provider
	Queue     *queue.Client
	Paperless *paperless.Client
	Search    *search.Client
}

func (d Deps) Client() *surrealdb.DB {
	if d.DB == nil {
		return nil
	}
	return d.DB.Client()
}

func respondJSON[T any](c *echo.Context, code int, data T) error {
	return c.JSON(code, data)
}

func respondError(c *echo.Context, code int, message string) error {
	return c.JSON(code, map[string]string{"error": message})
}

func bind(c *echo.Context, dst any) error {
	if err := c.Bind(dst); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}
	return nil
}

func parsePageParams(c *echo.Context) (page, pageSize int) {
	page, pageSize = 1, 50
	if raw := c.QueryParam("page"); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil && n > 0 {
			page = n
		}
	}
	if raw := c.QueryParam("page_size"); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil && n > 0 {
			pageSize = n
		}
	}
	return page, pageSize
}
