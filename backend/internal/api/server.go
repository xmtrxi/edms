package api

import (
	"context"
	"net/http"

	"github.com/labstack/echo/v5"
	"github.com/labstack/echo/v5/middleware"
	"github.com/xmtrxi/edms/internal/config"
	"github.com/xmtrxi/edms/internal/handler"
)

type Server struct {
	echo   *echo.Echo
	config config.Config
}

func New(cfg config.Config, deps handler.Deps) *Server {
	e := echo.NewWithConfig(echo.Config{
		NoGroupAutoRegister404Routes: true,
	})
	e.Use(middleware.RequestLogger())
	e.Use(middleware.Recover())
	e.Use(middleware.CORSWithConfig(middleware.CORSConfig{
		AllowOrigins: []string{"http://localhost:3000", "http://127.0.0.1:3000"},
		AllowMethods: []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodPatch, http.MethodOptions},
		AllowHeaders: []string{echo.HeaderOrigin, echo.HeaderContentType, echo.HeaderAccept, echo.HeaderAuthorization},
	}))

	s := &Server{
		echo:   e,
		config: cfg,
	}
	s.registerRoutes(deps)

	return s
}

func (s *Server) registerRoutes(deps handler.Deps) {
	handler.NewDocsHandler().RegisterRoutes(s.echo)

	api := s.echo.Group("/api")
	handler.NewHealthHandler(deps).RegisterRoutes(s.echo, api)
	handler.NewAuthHandler(deps).RegisterRoutes(api)
	handler.NewDocumentsHandler(deps).RegisterRoutes(api)
	handler.NewSearchHandler(deps).RegisterRoutes(api)
}

func (s *Server) Start(ctx context.Context) error {
	start := echo.StartConfig{
		Address:    ":" + s.config.App.Port,
		HideBanner: true,
	}

	return start.Start(ctx, s.echo)
}

func (s *Server) Handler() http.Handler {
	return s.echo
}
