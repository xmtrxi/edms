package tasks

import (
	"log/slog"

	"github.com/hibiken/asynq"
	"github.com/surrealdb/surrealdb.go"

	"github.com/xmtrxi/edms/internal/config"
	"github.com/xmtrxi/edms/internal/queue"
	"github.com/xmtrxi/edms/internal/service"
)

type Server struct {
	inner *asynq.Server
	mux   *asynq.ServeMux
}

func NewServer(cfg config.Config, db *surrealdb.DB, ocr *service.OCRService, search *service.SearchService, logger *slog.Logger) (*Server, error) {
	opts, err := queue.RedisOptions(cfg.Queue.RedisURL)
	if err != nil {
		return nil, err
	}

	t := New(cfg, ocr, search, service.NewOperationsService(db, cfg.SMTP), logger)
	t.labels = service.NewLabelService(db, ocr.Storage(), service.NewAIProvider(cfg.AI), logger)
	mux := asynq.NewServeMux()
	mux.HandleFunc(queue.TaskOCR, t.OCRDocument)
	mux.HandleFunc(queue.TaskReindex, t.Reindex)
	mux.HandleFunc(queue.TaskEmail, t.DeliverEmail)

	inner := asynq.NewServer(opts, asynq.Config{
		Concurrency: cfg.Queue.Concurrency,
	})

	return &Server{inner: inner, mux: mux}, nil
}

func (s *Server) Start() error {
	return s.inner.Start(s.mux)
}

func (s *Server) Shutdown() {
	s.inner.Shutdown()
}
