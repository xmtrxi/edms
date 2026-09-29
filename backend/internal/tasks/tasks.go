package tasks

import (
	"log/slog"

	"github.com/xmtrxi/edms/internal/config"
	"github.com/xmtrxi/edms/internal/service"
)

type Tasks struct {
	ocr    *service.OCRService
	search *service.SearchService
	email  *service.OperationsService
	cfg    config.Config
	log    *slog.Logger
}

func New(cfg config.Config, ocr *service.OCRService, search *service.SearchService, email *service.OperationsService, logger *slog.Logger) *Tasks {
	return &Tasks{ocr: ocr, search: search, email: email, cfg: cfg, log: logger}
}
