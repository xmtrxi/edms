package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/xmtrxi/edms/internal/api"
	"github.com/xmtrxi/edms/internal/cache"
	"github.com/xmtrxi/edms/internal/config"
	"github.com/xmtrxi/edms/internal/database"
	"github.com/xmtrxi/edms/internal/database/migrations"
	"github.com/xmtrxi/edms/internal/docling"
	"github.com/xmtrxi/edms/internal/gotenberg"
	"github.com/xmtrxi/edms/internal/handler"
	"github.com/xmtrxi/edms/internal/queue"
	"github.com/xmtrxi/edms/internal/search"
	"github.com/xmtrxi/edms/internal/service"
	"github.com/xmtrxi/edms/internal/storage"
	"github.com/xmtrxi/edms/internal/tasks"
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	cfg := config.Load(".env")

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	db, err := database.Connect(ctx, cfg.Database)
	if err != nil {
		logger.Error("connect database", "error", err)
		os.Exit(1)
	}
	defer db.Close(ctx)

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
		defer cacheClient.Close()
	}

	var queueClient *queue.Client
	if cfg.Queue.RedisURL != "" {
		queueClient, err = queue.NewClient(cfg.Queue.RedisURL)
		if err != nil {
			logger.Error("connect queue", "error", err)
			os.Exit(1)
		}
		defer queueClient.Close()
	}

	var doclingClient *docling.Client
	if cfg.OCR.Docling.BaseURL != "" {
		doclingClient = docling.New(cfg.OCR.Docling)
	}
	var gotenbergClient *gotenberg.Client
	if cfg.OCR.Gotenberg != "" {
		gotenbergClient = gotenberg.New(cfg.OCR.Gotenberg, cfg.OCR.Docling.Timeout)
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

	if queueClient != nil {
		worker, err := tasks.NewServer(cfg, db.Client(), ocrSvc, searchSvc, logger)
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

	server := api.New(cfg, handler.Deps{
		Cfg:     cfg,
		Logger:  logger,
		DB:      db,
		Cache:   cacheClient,
		Storage: provider,
		Queue:   queueClient,
		Search:  searchClient,
		Office:  gotenbergClient,
	})
	if err := server.Start(ctx); err != nil {
		logger.Error("api server stopped", "error", err)
		os.Exit(1)
	}
}
