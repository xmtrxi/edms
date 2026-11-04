package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/valkey-io/valkey-go"

	"github.com/xmtrxi/edms/internal/api"
	"github.com/xmtrxi/edms/internal/cache"
	"github.com/xmtrxi/edms/internal/config"
	"github.com/xmtrxi/edms/internal/database"
	"github.com/xmtrxi/edms/internal/database/migrations"
	"github.com/xmtrxi/edms/internal/docling"
	"github.com/xmtrxi/edms/internal/gotenberg"
	"github.com/xmtrxi/edms/internal/handler"
	"github.com/xmtrxi/edms/internal/observability"
	"github.com/xmtrxi/edms/internal/pdftools"
	"github.com/xmtrxi/edms/internal/queue"
	"github.com/xmtrxi/edms/internal/realtime"
	"github.com/xmtrxi/edms/internal/scan"
	"github.com/xmtrxi/edms/internal/search"
	"github.com/xmtrxi/edms/internal/service"
	"github.com/xmtrxi/edms/internal/storage"
	"github.com/xmtrxi/edms/internal/tasks"
)

// version is set at build time with -ldflags "-X main.version=...".
var version = "dev"

func main() {
	if len(os.Args) > 1 && os.Args[1] == "healthcheck" {
		os.Exit(healthcheck())
	}
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil)).With("version", version)
	cfg := config.Load(".env")
	cfg.App.Version = version
	if (cfg.App.Env == "production" || cfg.App.Env == "prod") && len(cfg.JWT.Secret) < 32 {
		logger.Error("JWT_SECRET must be set to at least 32 characters in production")
		os.Exit(1)
	}

	service.UseEmailEvents(cfg.EmailEvents.Provider)

	shutdownTracing, err := observability.SetupTracing(context.Background(), version)
	if err != nil {
		logger.Error("set up tracing", "error", err)
		os.Exit(1)
	}
	defer func() {
		flushCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = shutdownTracing(flushCtx)
	}()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	db, err := database.Connect(ctx, cfg.Database)
	if err != nil {
		logger.Error("connect database", "error", err)
		os.Exit(1)
	}
	defer func() { _ = db.Close(ctx) }()

	if err := database.AutoMigrate(ctx, db, migrations.Migrations()); err != nil {
		// Migrations are now idempotent (IF NOT EXISTS + already-exists swallow),
		// but keep a defensive check so an old DB with pre-existing schema doesn't
		// crash the API on restart - "dont worry if the fields exist".
		if strings.Contains(strings.ToLower(err.Error()), "already exists") {
			logger.Warn("migrations already applied, continuing", "error", err)
		} else {
			logger.Error("run migrations", "error", err)
			os.Exit(1)
		}
	}

	if err := service.NewOrganizationService(cfg, db.Client(), logger).Bootstrap(ctx); err != nil {
		logger.Error("bootstrap organization", "error", err)
		os.Exit(1)
	}

	provider, err := storage.NewProvider(ctx, cfg)
	if err != nil {
		logger.Error("create storage provider", "error", err)
		os.Exit(1)
	}

	var cacheClient *cache.Cache
	if cfg.Cache.URL != "" {
		cacheClient, err = cache.NewClient(cfg.Cache.URL)
		if err != nil {
			logger.Error("connect cache", "error", err)
			os.Exit(1)
		}
		defer func() { _ = cacheClient.Close() }()
	}

	var queueClient *queue.Client
	if cfg.Queue.RedisURL != "" {
		queueClient, err = queue.NewClient(cfg.Queue.RedisURL)
		if err != nil {
			logger.Error("connect queue", "error", err)
			os.Exit(1)
		}
		defer func() { _ = queueClient.Close() }()
	}

	var doclingClient *docling.Client
	if cfg.OCR.Docling.BaseURL != "" {
		doclingClient = docling.New(cfg.OCR.Docling)
	}
	var gotenbergClient *gotenberg.Client
	if cfg.OCR.Gotenberg != "" {
		gotenbergClient = gotenberg.New(cfg.OCR.Gotenberg, cfg.OCR.Docling.Timeout)
	}

	var scanner scan.Scanner
	if cfg.Scan.ClamAVAddress != "" {
		scanner = scan.NewClamd(cfg.Scan.ClamAVAddress, cfg.Scan.Timeout)
		service.UseScanner(scanner, cfg.Scan.Required)
		if err := scanner.Ping(ctx); err != nil {
			logger.Warn("malware scanner is not reachable yet", "address", cfg.Scan.ClamAVAddress, "required", cfg.Scan.Required, "error", err)
		}
	}

	var searchClient *search.Client
	if cfg.Search.URL != "" {
		searchClient = search.New(cfg.Search)
	}

	if searchClient != nil && cfg.AI.EmbeddingDimensions > 0 && cfg.AI.EmbeddingModel != "" {
		searchClient.EnableSemantic(cfg.AI.EmbeddingDimensions, service.NewAIProvider(cfg.AI).Embed)
	}
	if searchClient != nil {
		if err := searchClient.Configure(ctx); err != nil {
			logger.Error("configure search", "error", err)
			os.Exit(1)
		}
	}

	ocrSvc := service.NewOCRService(db.Client(), provider, doclingClient, gotenbergClient, searchClient, logger)
	searchSvc := service.NewSearchService(db.Client(), provider, searchClient, logger)

	backgroundDocs := service.NewDocumentService(cfg, db.Client(), provider, queueClient, searchClient, logger)
	pdfSvc := service.NewPDFService(db.Client(), pdfClient(cfg), backgroundDocs,
		service.NewVersionService(db.Client(), provider, backgroundDocs),
		service.NewPreviewService(backgroundDocs, provider, gotenbergClient), queueClient, logger)

	if queueClient != nil {
		worker, err := tasks.NewServer(cfg, db.Client(), ocrSvc, searchSvc, pdfSvc, logger)
		if err != nil {
			logger.Error("start worker", "error", err)
			os.Exit(1)
		}
		defer worker.Shutdown()
		go func() {
			if err := worker.Start(); err != nil {
				logger.Error("worker stopped", "error", err)
			}
		}()
	}

	var valkeyClient valkey.Client
	if cacheClient != nil {
		valkeyClient = cacheClient.Client
	}
	live := realtime.New(valkeyClient, logger)
	service.UseRealtime(live)
	go live.Run(ctx)

	mail := service.NewOperationsService(db.Client(), cfg.SMTP, queueClient)
	notificationSvc := service.NewNotificationService(db.Client(), mail, cfg.SMTP, logger)
	service.UseNotifications(notificationSvc)
	service.UseAutomation(service.NewAutomationService(cfg, db.Client(), backgroundDocs, mail, logger))
	claim := func(ctx context.Context, ttl time.Duration) bool {
		if valkeyClient == nil {
			return true
		}
		// One instance runs each pass; the lock expires before the next.
		err := valkeyClient.Do(ctx, valkeyClient.B().Set().Key("edms:maintenance").Value("1").Nx().Ex(ttl).Build()).Error()
		return err == nil
	}
	maintenance := service.NewMaintenance(
		service.NewUploadService(cfg.Uploads, db.Client(), backgroundDocs),
		service.NewRetentionService(cfg, db.Client(), provider, logger),
		claim, logger,
	)
	maintenance.Add("purge sessions", service.NewSessionService(db.Client(), cfg.JWT.SessionTTL).Purge)
	maintenance.Add("purge notifications", notificationSvc.Purge)
	maintenance.Add("send workflow digests", notificationSvc.SendDigests)
	m365 := service.NewMicrosoft365Service(db.Client(), provider, backgroundDocs, service.NewVersionService(db.Client(), provider, backgroundDocs), logger)
	maintenance.Add("check in Microsoft 365 edits", m365.SyncOpen)
	reconciler := service.NewReconcileService(db.Client(), backgroundDocs, mail, pdfSvc, provider, scanner, nil, logger)
	maintenance.Add("retry stuck work", func(ctx context.Context) error {
		_, err := reconciler.Reconcile(ctx, false)
		return err
	})
	go maintenance.Start(ctx, time.Hour)

	server := api.New(cfg, handler.Deps{
		Cfg:     cfg,
		Logger:  logger,
		DB:      db,
		Cache:   cacheClient,
		Storage: provider,
		Queue:   queueClient,
		Search:  searchClient,
		Office:  gotenbergClient,
		Scanner: scanner,

		Realtime: live,
	})
	if err := server.Start(ctx); err != nil {
		logger.Error("api server stopped", "error", err)
		os.Exit(1)
	}
}

// healthcheck lets container runtimes probe the API from inside a minimal
// image that has no shell or curl.
func healthcheck() int {
	port := os.Getenv("APP_PORT")
	if port == "" {
		port = "8080"
	}
	client := http.Client{Timeout: 4 * time.Second}
	resp, err := client.Get("http://127.0.0.1:" + port + "/health")
	if err != nil {
		return 1
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return 1
	}
	return 0
}

// pdfClient connects Stirling-PDF when it is configured.
func pdfClient(cfg config.Config) *pdftools.Client {
	if cfg.PDFTools.URL == "" {
		return nil
	}
	return pdftools.New(cfg.PDFTools.URL, cfg.PDFTools.APIKey, cfg.PDFTools.Timeout)
}
