package main

import (
	"context"
	"log/slog"
	"os"

	"github.com/xmtrxi/edms/internal/api"
	"github.com/xmtrxi/edms/internal/config"
	"github.com/xmtrxi/edms/internal/storage"
)

func main() {
	cfg := config.Load(".env")
	ctx := context.Background()

	// Initialise the storage provider (S3 / MinIO).
	store, err := storage.NewProvider(ctx, cfg)
	if err != nil {
		slog.Error("failed to create storage provider", "error", err)
		os.Exit(1)
	}

	// Auto-create the bucket if the provider supports it.
	if eb, ok := store.(storage.EnsureBucketer); ok {
		if err := eb.EnsureBucket(ctx); err != nil {
			slog.Error("failed to ensure storage bucket exists", "error", err)
			os.Exit(1)
		}
	}

	// Verify connectivity before accepting traffic.
	if err := store.HealthCheck(ctx); err != nil {
		slog.Error("storage health check failed — is MinIO running?", "error", err)
		os.Exit(1)
	}
	slog.Info("storage provider ready", "provider", store.Name())

	server := api.New(cfg, store)

	if err := server.Start(); err != nil {
		slog.Error("api server stopped", "error", err)
		os.Exit(1)
	}
}
