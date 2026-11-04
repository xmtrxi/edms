package api

import (
	"context"
	"crypto/subtle"
	"log"
	"net/http"
	"time"

	"github.com/hibiken/asynqmon"
	"github.com/labstack/echo/v5"
	"github.com/labstack/echo/v5/middleware"
	"github.com/xmtrxi/edms/internal/config"
	"github.com/xmtrxi/edms/internal/handler"
	"github.com/xmtrxi/edms/internal/observability"
	"github.com/xmtrxi/edms/internal/queue"
)

type Server struct {
	echo   *echo.Echo
	config config.Config
}

func New(cfg config.Config, deps handler.Deps) *Server {
	e := echo.NewWithConfig(echo.Config{
		NoGroupAutoRegister404Routes: true,
		// Record IDs contain a colon, which clients encode in paths.
		Router: echo.NewRouter(echo.RouterConfig{UnescapePathParamValues: true}),
	})
	if cfg.Security.TrustProxy {
		// Proxies on loopback and private networks are trusted by default.
		e.IPExtractor = echo.ExtractIPFromXFFHeader()
	} else {
		e.IPExtractor = echo.ExtractIPDirect()
	}
	e.Use(middleware.RequestLogger())
	e.Use(middleware.Recover())
	e.Use(observability.TracingMiddleware())
	e.Use(observability.Middleware())
	e.Use(middleware.SecureWithConfig(middleware.SecureConfig{
		XSSProtection:      "0",
		ContentTypeNosniff: "nosniff",
		XFrameOptions:      "DENY",
		HSTSMaxAge:         31536000,
		ReferrerPolicy:     "no-referrer",
	}))
	e.Use(bodyLimit(cfg))
	origins := cfg.Security.CORSOrigins
	if len(origins) == 0 {
		origins = []string{"http://localhost:3000"}
	}
	e.Use(middleware.CORSWithConfig(middleware.CORSConfig{
		AllowOrigins:  origins,
		AllowMethods:  []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodPatch, http.MethodOptions},
		AllowHeaders:  []string{echo.HeaderOrigin, echo.HeaderContentType, echo.HeaderAccept, echo.HeaderAuthorization, "X-Visitor-Token", "X-Link-Password", "Mcp-Session-Id", "Mcp-Protocol-Version"},
		ExposeHeaders: []string{"X-Preview-Kind", echo.HeaderContentDisposition},
	}))

	s := &Server{
		echo:   e,
		config: cfg,
	}
	s.registerTaskRoutes()
	s.registerMetrics()
	s.registerRoutes(deps)

	return s
}

// registerMetrics serves Prometheus metrics at /metrics. When monitoring
// credentials are set, scrapers must send them as HTTP basic auth.
func (s *Server) registerMetrics() {
	var guards []echo.MiddlewareFunc
	if auth := s.monitoringAuth(); auth != nil {
		guards = append(guards, auth)
	}
	s.echo.GET("/metrics", echo.WrapHandler(observability.Handler()), guards...)
}

// monitoringAuth checks MONITORING_USER and MONITORING_PASSWORD; it is nil
// when they are not set.
func (s *Server) monitoringAuth() echo.MiddlewareFunc {
	sec := s.config.Security
	if sec.MonitoringUser == "" || sec.MonitoringPassword == "" {
		return nil
	}
	return middleware.BasicAuthWithConfig(middleware.BasicAuthConfig{
		Realm: "EDMS monitoring",
		Validator: func(c *echo.Context, user, password string) (bool, error) {
			userOK := subtle.ConstantTimeCompare([]byte(user), []byte(sec.MonitoringUser)) == 1
			passwordOK := subtle.ConstantTimeCompare([]byte(password), []byte(sec.MonitoringPassword)) == 1
			return userOK && passwordOK, nil
		},
	})
}

// registerTaskRoutes serves the task dashboard behind HTTP basic auth. Its
// browser UI makes its own requests, so it cannot use bearer tokens.
func (s *Server) registerTaskRoutes() {
	if s.monitoringAuth() == nil || s.config.Queue.RedisURL == "" {
		return
	}
	opts, err := queue.RedisOptions(s.config.Queue.RedisURL)
	if err != nil {
		log.Fatal("unable to parse redis url: \n", err)
	}
	mon := asynqmon.New(asynqmon.Options{RootPath: "/monitoring/tasks", RedisConnOpt: opts})
	auth := s.monitoringAuth()
	s.echo.Any("/monitoring/tasks", echo.WrapHandler(mon), auth)
	s.echo.Any("/monitoring/tasks/*", echo.WrapHandler(mon), auth)
}

func (s *Server) registerRoutes(deps handler.Deps) {
	handler.NewDocsHandler().RegisterRoutes(s.echo)

	handler.NewWebDAVHandler(deps).Register(s.echo)

	api := s.echo.Group("/api")
	handler.NewHealthHandler(deps).RegisterRoutes(s.echo, api)
	handler.NewAuthHandler(deps).RegisterRoutes(api)
	handler.NewOrganizationsHandler(deps).RegisterRoutes(api)
	handler.NewSharingHandler(deps).RegisterRoutes(api)
	handler.NewFileRequestsHandler(deps).RegisterRoutes(api)
	handler.NewMCPHandler(deps).RegisterRoutes(api)
	handler.NewESignHandler(deps).RegisterRoutes(api)
	handler.NewRealtimeHandler(deps).RegisterRoutes(api)
	handler.NewCommentsHandler(deps).RegisterRoutes(api)
	handler.NewNotificationsHandler(deps).RegisterRoutes(api)
	handler.NewEditorHandler(deps).RegisterRoutes(api)
	handler.NewAutomationHandler(deps).RegisterRoutes(api)
	handler.NewRetentionHandler(deps).RegisterRoutes(api)
	handler.NewCategoriesHandler(deps).RegisterRoutes(api)
	handler.NewDepartmentsHandler(deps).RegisterRoutes(api)
	handler.NewAdministrationHandler(deps).RegisterRoutes(api)
	handler.NewFormsHandler(deps).RegisterRoutes(api)
	handler.NewVisitorsHandler(deps).RegisterRoutes(api)
	handler.NewWorkspaceHandler(deps).RegisterRoutes(api)
	handler.NewAIHandler(deps).RegisterRoutes(api)
	handler.NewOperationsHandler(deps).RegisterRoutes(api)
	handler.NewFoldersHandler(deps).RegisterRoutes(api)
	handler.NewDocumentsHandler(deps).RegisterRoutes(api)
	handler.NewVersionsHandler(deps).RegisterRoutes(api)
	handler.NewPDFToolsHandler(deps).RegisterRoutes(api)
	handler.NewIntegrationsHandler(deps).RegisterRoutes(api)
	handler.NewMicrosoft365Handler(deps).RegisterRoutes(api)
	handler.NewRoutingHandler(deps).RegisterRoutes(api)
	handler.NewSearchHandler(deps).RegisterRoutes(api)
}

func (s *Server) Start(ctx context.Context) error {
	start := echo.StartConfig{
		Address:    ":" + s.config.App.Port,
		HideBanner: true,
		// Uploads, downloads and WebSockets stream for a long time, so only
		// slow headers and idle connections are cut off.
		BeforeServeFunc: func(server *http.Server) error {
			server.ReadHeaderTimeout = 15 * time.Second
			server.IdleTimeout = 2 * time.Minute
			server.MaxHeaderBytes = 1 << 20
			return nil
		},
	}

	return start.Start(ctx, s.echo)
}

func (s *Server) Handler() http.Handler {
	return s.echo
}
