package service

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/surrealdb/surrealdb.go"

	"github.com/xmtrxi/edms/internal/database"
	"github.com/xmtrxi/edms/internal/queue"
	"github.com/xmtrxi/edms/internal/scan"
	"github.com/xmtrxi/edms/internal/storage"
)

// Thresholds after which background work counts as stuck.
const (
	stuckEmailAfter = 30 * time.Minute
	stuckJobAfter   = time.Hour
)

// ReconcileService finds work that fell between systems, such as emails
// that never left or jobs whose worker died, and puts it back on track.
type ReconcileService struct {
	db        *surrealdb.DB
	docs      *DocumentService
	email     *OperationsService
	pdf       *PDFService
	storage   storage.Provider
	scanner   scan.Scanner
	inspector *queue.Inspector
	log       *slog.Logger
}

func NewReconcileService(db *surrealdb.DB, docs *DocumentService, email *OperationsService, pdf *PDFService, provider storage.Provider, scanner scan.Scanner, inspector *queue.Inspector, logger *slog.Logger) *ReconcileService {
	if logger == nil {
		logger = slog.Default()
	}
	return &ReconcileService{db: db, docs: docs, email: email, pdf: pdf, storage: provider, scanner: scanner, inspector: inspector, log: logger}
}

// HealthCheck is one line of the integration report.
type HealthCheck struct {
	Name   string `json:"name"`
	Status string `json:"status"` // ok, warning or failing
	Detail string `json:"detail"`
	Count  int    `json:"count,omitempty"`
	// Fixable means Reconcile can repair it.
	Fixable bool `json:"fixable,omitempty"`
}

// Report checks every integration the API depends on.
func (s *ReconcileService) Report(ctx context.Context) []HealthCheck {
	now := time.Now().UTC()
	checks := []HealthCheck{}
	add := func(name string, count int, fixable bool, okText, problem string) {
		check := HealthCheck{Name: name, Status: "ok", Detail: okText, Count: count}
		if count > 0 {
			check.Status, check.Detail, check.Fixable = "warning", fmt.Sprintf(problem, count), fixable
		}
		checks = append(checks, check)
	}

	stuckEmails := s.count(ctx, "SELECT count() AS count FROM email_deliveries WHERE status = 'queued' AND created_at < $before GROUP ALL", now.Add(-stuckEmailAfter))
	add("Email waiting to send", stuckEmails, true, "No email is waiting longer than expected.", "%d emails have waited more than 30 minutes to send.")
	failedEmails := s.count(ctx, "SELECT count() AS count FROM email_deliveries WHERE status IN ['failed', 'bounced'] AND updated_at > $before GROUP ALL", now.Add(-7*24*time.Hour))
	add("Email failures", failedEmails, false, "No email failed or bounced this week.", "%d emails failed or bounced this week; review them under Email deliveries.")

	stuckOCR := s.count(ctx, "SELECT count() AS count FROM processing_jobs WHERE status IN ['queued', 'running'] AND updated_at < $before GROUP ALL", now.Add(-stuckJobAfter))
	add("Text extraction", stuckOCR, true, "No document is stuck in text extraction.", "%d documents have been waiting over an hour for text extraction.")

	stuckPDF := s.count(ctx, "SELECT count() AS count FROM pdf_jobs WHERE status IN ['queued', 'running', 'retrying'] AND updated_at < $before GROUP ALL", now.Add(-stuckJobAfter))
	add("PDF tools", stuckPDF, true, "No PDF job is stuck.", "%d PDF jobs have made no progress for over an hour.")

	if s.inspector != nil {
		check := HealthCheck{Name: "Task queue", Status: "ok", Detail: "No task has used up its retries."}
		if stats, err := s.inspector.Stats(); err != nil {
			check.Status, check.Detail = "failing", "The task queue can't be reached: "+err.Error()
		} else {
			for _, q := range stats {
				check.Count += q.Dead
			}
			if check.Count > 0 {
				check.Status, check.Fixable = "warning", true
				check.Detail = fmt.Sprintf("%d tasks failed every retry and are waiting in the dead-letter list.", check.Count)
			}
		}
		checks = append(checks, check)
	} else {
		checks = append(checks, HealthCheck{Name: "Task queue", Status: "warning", Detail: "No task queue is configured, so background work runs inside the API."})
	}

	if s.storage != nil {
		check := HealthCheck{Name: "File storage (" + s.storage.Name() + ")", Status: "ok", Detail: "Storage answers."}
		if err := s.storage.HealthCheck(ctx); err != nil {
			check.Status, check.Detail = "failing", err.Error()
		}
		checks = append(checks, check)
	}
	check := HealthCheck{Name: "Malware scanning", Status: "warning", Detail: "Uploads are not scanned. Set CLAMAV_ADDRESS to turn scanning on."}
	if s.scanner != nil {
		check.Status, check.Detail = "ok", "ClamAV answers."
		if err := s.scanner.Ping(ctx); err != nil {
			check.Status, check.Detail = "failing", "ClamAV can't be reached: "+err.Error()
		}
	}
	return append(checks, check)
}

// ReconcileResult counts what was put back on track.
type ReconcileResult struct {
	Emails    int `json:"emails"`
	OCR       int `json:"ocr"`
	PDFJobs   int `json:"pdf_jobs"`
	DeadTasks int `json:"dead_tasks"`
}

// Reconcile retries stuck work. With requeueDead it also moves dead tasks
// back to the queue, which is only done when an administrator asks.
func (s *ReconcileService) Reconcile(ctx context.Context, requeueDead bool) (ReconcileResult, error) {
	var result ReconcileResult
	now := time.Now().UTC()

	emails, err := queryRows[database.EmailDelivery](ctx, s.db,
		"SELECT * FROM email_deliveries WHERE status = 'queued' AND created_at < $before LIMIT 200",
		map[string]any{"before": now.Add(-stuckEmailAfter)})
	if err != nil {
		return result, err
	}
	for _, item := range emails {
		if _, err := s.email.RetryEmail(ctx, string(item.ID)); err == nil {
			result.Emails++
		}
	}

	jobs, err := queryRows[database.ProcessingJob](ctx, s.db,
		"SELECT * FROM processing_jobs WHERE status IN ['queued', 'running'] AND updated_at < $before LIMIT 100",
		map[string]any{"before": now.Add(-stuckJobAfter)})
	if err != nil {
		return result, err
	}
	for _, job := range jobs {
		if s.restageOCR(ctx, job) {
			result.OCR++
		}
	}

	if s.pdf != nil && s.pdf.Enabled() {
		pdfJobs, err := queryRows[database.PDFJob](ctx, s.db,
			"SELECT * FROM pdf_jobs WHERE status IN ['queued', 'running', 'retrying'] AND updated_at < $before LIMIT 100",
			map[string]any{"before": now.Add(-stuckJobAfter)})
		if err != nil {
			return result, err
		}
		for i := range pdfJobs {
			job := s.pdf.update(ctx, &pdfJobs[i], map[string]any{"status": database.PDFJobQueued})
			if err := s.pdf.dispatch(WithActor(ctx, Actor{UserID: job.UserID, OrgID: job.OrgID}), job); err == nil {
				result.PDFJobs++
			}
		}
	}

	if requeueDead && s.inspector != nil {
		n, err := s.inspector.RequeueAll()
		if err != nil {
			return result, err
		}
		result.DeadTasks = n
	}
	if result != (ReconcileResult{}) {
		s.log.Info("reconciled background work", "emails", result.Emails, "ocr", result.OCR, "pdf_jobs", result.PDFJobs, "dead_tasks", result.DeadTasks)
	}
	return result, nil
}

// restageOCR closes a stuck extraction job and queues a fresh one.
func (s *ReconcileService) restageOCR(ctx context.Context, job database.ProcessingJob) bool {
	now := time.Now().UTC()
	if _, err := surrealdb.Merge[database.ProcessingJob](ctx, s.db, mustRecordID(string(job.ID)), map[string]any{
		"status": database.ProcessingJobStatusFailed, "last_error": "stopped responding; queued again", "updated_at": now, "completed_at": now,
	}); err != nil {
		return false
	}
	doc, err := surrealdb.Select[database.Document](ctx, s.db, mustRecordID(job.DocumentID))
	if err != nil || doc == nil || doc.ID == "" || doc.CurrentVersionID == "" {
		return false
	}
	version, err := surrealdb.Select[database.DocumentVersion](ctx, s.db, mustRecordID(doc.CurrentVersionID))
	if err != nil || version == nil {
		return false
	}
	s.docs.stageOCR(ctx, doc, version)
	return true
}

func (s *ReconcileService) count(ctx context.Context, statement string, before time.Time) int {
	n, err := queryCount(ctx, s.db, statement, map[string]any{"before": before})
	if err != nil {
		s.log.Warn("integration check", "error", err)
	}
	return n
}
