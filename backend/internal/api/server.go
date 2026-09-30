package api

import (
	"context"
	"log"
	"net/http"
	"net/url"

	"github.com/hibiken/asynq"
	"github.com/hibiken/asynqmon"
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
		// Record IDs contain a colon, which clients encode in paths.
		Router: echo.NewRouter(echo.RouterConfig{UnescapePathParamValues: true}),
	})
	e.Use(middleware.RequestLogger())
	e.Use(middleware.Recover())
	e.Use(middleware.CORSWithConfig(middleware.CORSConfig{
		AllowOrigins:  []string{"http://localhost:3000", "http://127.0.0.1:3000", "http://localhost:3001"},
		AllowMethods:  []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodPatch, http.MethodOptions},
		AllowHeaders:  []string{echo.HeaderOrigin, echo.HeaderContentType, echo.HeaderAccept, echo.HeaderAuthorization, "X-Visitor-Token", "X-Link-Password", "Mcp-Session-Id", "Mcp-Protocol-Version"},
		ExposeHeaders: []string{"X-Preview-Kind", echo.HeaderContentDisposition},
	}))

	s := &Server{
		echo:   e,
		config: cfg,
	}
	s.registerTaskRoutes()
	s.registerRoutes(deps)

	return s
}
func (s *Server) registerTaskRoutes() {
	u, err := url.Parse(s.config.Cache.URL)
	if err != nil {
		log.Fatal("unable to parse redis url: \n", err)
	}
	var (
		username string
		password string
	)
	if u.User != nil {
		username = u.User.Username()
		password, _ = u.User.Password()
	}
	mon := asynqmon.New(asynqmon.Options{
		RootPath: "/monitoring/tasks",
		RedisConnOpt: asynq.RedisClientOpt{
			Addr:     u.Host,
			Username: username,
			Password: password,
		},
	})
	s.echo.Any("/monitoring/tasks/*", echo.WrapHandler(mon))
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
	handler.NewEditorHandler(deps).RegisterRoutes(api)
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
	handler.NewRoutingHandler(deps).RegisterRoutes(api)
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
