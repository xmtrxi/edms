package api

import (
	"context"
	"net/http"

	"github.com/labstack/echo/v5"
	"github.com/labstack/echo/v5/middleware"
	"github.com/xmtrxi/edms/internal/config"
	"github.com/xmtrxi/edms/internal/storage"
)

type Server struct {
	echo      *echo.Echo
	config    config.Config
	storage   storage.Provider
	documents *documentStore
}

func New(cfg config.Config, store storage.Provider) *Server {
	e := echo.New()
	e.Use(middleware.RequestLogger())
	e.Use(middleware.Recover())
	e.Use(middleware.CORSWithConfig(middleware.CORSConfig{
		AllowOrigins: []string{"http://localhost:3000", "http://127.0.0.1:3000"},
		AllowMethods: []string{http.MethodGet, http.MethodPost, http.MethodDelete, http.MethodOptions},
		AllowHeaders: []string{echo.HeaderOrigin, echo.HeaderContentType, echo.HeaderAccept, echo.HeaderAuthorization},
	}))

	s := &Server{
		echo:      e,
		config:    cfg,
		storage:   store,
		documents: newDocumentStore(),
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
	documents.GET("", s.listDocuments)
	documents.POST("", s.uploadDocument)
	documents.GET("/:id", s.getDocument)
	documents.GET("/:id/download", s.downloadDocument)
	documents.GET("/:id/url", s.getDocumentURL)
	documents.DELETE("/:id", s.deleteDocument)
	if s.config.Signing.Enabled {
		documents.POST("/:id/sign", s.signDocument)
	}
}

func (s *Server) health(c *echo.Context) error {
	storageName := ""
	if s.storage != nil {
		storageName = s.storage.Name()
	}
	return c.JSON(http.StatusOK, map[string]any{
		"status":           "ok",
		"env":              s.config.App.Env,
		"storage_provider": storageName,
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
