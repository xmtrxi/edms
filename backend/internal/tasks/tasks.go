package tasks

import (
	"log/slog"

	"github.com/xmtrxi/edms/internal/config"
	"github.com/xmtrxi/edms/internal/service"
)

type Tasks struct {
	ocr    *service.OCRService
	search *service.SearchService
	cfg    config.Config
	log    *slog.Logger
}

func New(cfg config.Config, ocr *service.OCRService, search *service.SearchService, logger *slog.Logger) *Tasks {
	return &Tasks{ocr: ocr, search: search, cfg: cfg, log: logger}
}
