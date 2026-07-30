package main

import (
	"log/slog"
	"os"

	"github.com/xmtrxi/edms/internal/api"
	"github.com/xmtrxi/edms/internal/config"
)

func main() {
	cfg := config.Load(".env")
	server := api.New(cfg)

	if err := server.Start(); err != nil {
		slog.Error("api server stopped", "error", err)
		os.Exit(1)
	}
}
