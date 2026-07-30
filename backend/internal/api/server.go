package api

import (
	"context"
	"net/http"

	"github.com/labstack/echo/v5"
	"github.com/labstack/echo/v5/middleware"
	"github.com/xmtrxi/edms/internal/config"
)

type Server struct {
	echo   *echo.Echo
	config config.Config
}

func New(cfg config.Config) *Server {
	e := echo.New()
	e.Use(middleware.RequestLogger())
	e.Use(middleware.Recover())

	s := &Server{
		echo:   e,
		config: cfg,
	}
	s.registerRoutes()

	return s
}

func (s *Server) Start() error {
	start := echo.StartConfig{
		Address:    ":" + s.config.App.Port,
		HideBanner: true,
	}

	return start.Start(context.Background(), s.echo)
}

func (s *Server) Handler() http.Handler {
	return s.echo
}

func (s *Server) registerRoutes() {
	s.echo.GET("/health", s.health)
	s.echo.GET("/docs", s.swagger)
	s.echo.GET("/openapi.yaml", s.openAPI)

	api := s.echo.Group("/api")
	api.GET("/health", s.health)

	documents := api.Group("/documents")
	if s.config.Signing.Enabled {
		documents.POST("/:id/sign", s.signDocument)
	}
}

func (s *Server) health(c *echo.Context) error {
	return c.JSON(http.StatusOK, map[string]any{
		"status": "ok",
		"env":    s.config.App.Env,
		"features": map[string]bool{
			"signing": s.config.Signing.Enabled,
		},
	})
}

func (s *Server) signDocument(c *echo.Context) error {
	return c.JSON(http.StatusNotImplemented, map[string]string{
		"error": "document signing is enabled but not implemented yet",
	})
}
