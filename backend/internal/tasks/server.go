package tasks

import (
	"context"
	"log/slog"

	"github.com/hibiken/asynq"
	"github.com/surrealdb/surrealdb.go"

	"github.com/xmtrxi/edms/internal/config"
	"github.com/xmtrxi/edms/internal/observability"
	"github.com/xmtrxi/edms/internal/queue"
	"github.com/xmtrxi/edms/internal/service"
)

type Server struct {
	inner *asynq.Server
	mux   *asynq.ServeMux
}

func NewServer(cfg config.Config, db *surrealdb.DB, ocr *service.OCRService, search *service.SearchService, pdf *service.PDFService, logger *slog.Logger) (*Server, error) {
	opts, err := queue.RedisOptions(cfg.Queue.RedisURL)
	if err != nil {
		return nil, err
	}

	t := New(cfg, ocr, search, service.NewOperationsService(db, cfg.SMTP), logger)
	t.labels = service.NewLabelService(db, ocr.Storage(), service.NewAIProvider(cfg.AI), logger)
	t.pdf = pdf
	mux := asynq.NewServeMux()
	// One span per task, so background work shows up in traces too.
	mux.Use(func(next asynq.Handler) asynq.Handler {
		return asynq.HandlerFunc(func(ctx context.Context, task *asynq.Task) error {
			ctx, span := observability.StartSpan(ctx, "task "+task.Type())
			defer span.End()
			err := next.ProcessTask(ctx, task)
			if err != nil {
				span.RecordError(err)
			}
			return err
		})
	})
	mux.HandleFunc(queue.TaskOCR, t.OCRDocument)
	mux.HandleFunc(queue.TaskReindex, t.Reindex)
	mux.HandleFunc(queue.TaskEmail, t.DeliverEmail)
	if pdf != nil && pdf.Enabled() {
		mux.HandleFunc(queue.TaskPDF, t.RunPDF)
	}

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
