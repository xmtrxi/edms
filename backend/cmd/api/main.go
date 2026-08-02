package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/xmtrxi/edms/internal/api"
	"github.com/xmtrxi/edms/internal/cache"
	"github.com/xmtrxi/edms/internal/config"
	"github.com/xmtrxi/edms/internal/database"
	"github.com/xmtrxi/edms/internal/database/migrations"
	"github.com/xmtrxi/edms/internal/handler"
	"github.com/xmtrxi/edms/internal/paperless"
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
		logger.Error("run migrations", "error", err)
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

	var paperlessClient *paperless.Client
	if cfg.Paperless.BaseURL != "" {
		paperlessClient = paperless.New(cfg.Paperless)
	}

	var searchClient *search.Client
	if cfg.Search.URL != "" {
		searchClient = search.New(cfg.Search)
	}

	ocrSvc := service.NewOCRService(db.Client(), provider, paperlessClient, searchClient, logger)
	searchSvc := service.NewSearchService(db.Client(), searchClient, logger)

	if queueClient != nil {
		worker, err := tasks.NewServer(cfg, ocrSvc, searchSvc, logger)
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
		Cfg:       cfg,
		Logger:    logger,
		DB:        db,
		Cache:     cacheClient,
		Storage:   provider,
		Queue:     queueClient,
		Paperless: paperlessClient,
		Search:    searchClient,
	})
	if err := server.Start(); err != nil {
		logger.Error("api server stopped", "error", err)
		os.Exit(1)
	}
}
