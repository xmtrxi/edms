package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"path"
	"strings"
	"time"

	"github.com/hibiken/asynq"
	"github.com/surrealdb/surrealdb.go"

	"github.com/xmtrxi/edms/internal/database"
	"github.com/xmtrxi/edms/internal/observability"
	"github.com/xmtrxi/edms/internal/pdftools"
	"github.com/xmtrxi/edms/internal/queue"
)

var (
	ErrPDFToolsDisabled = errors.New("PDF tools are not configured")
	ErrPDFJobNotFound   = errors.New("PDF job not found")
)

// maxPDFInput caps each file sent to the PDF server.
const maxPDFInput = 200 << 20

// PDFService runs PDF tools as queued jobs. Each job stores exactly one
// output: a new version of the document, or a new document for merges.
type PDFService struct {
	db       *surrealdb.DB
	client   *pdftools.Client
	docs     *DocumentService
	versions *VersionService
	previews *PreviewService
	queue    *queue.Client
	log      *slog.Logger
}

func NewPDFService(db *surrealdb.DB, client *pdftools.Client, docs *DocumentService, versions *VersionService, previews *PreviewService, q *queue.Client, logger *slog.Logger) *PDFService {
	if logger == nil {
		logger = slog.Default()
	}
	return &PDFService{db: db, client: client, docs: docs, versions: versions, previews: previews, queue: q, log: logger}
}

func (s *PDFService) Enabled() bool { return s.client != nil }

// PDFJobInput asks for one tool run.
type PDFJobInput struct {
	Operation   string           `json:"operation"`
	DocumentIDs []string         `json:"document_ids"`
	Options     pdftools.Options `json:"options"`
	// Name is the merged document's name; the first file's folder holds it.
	Name string `json:"name"`
	// IdempotencyKey makes repeated submissions return the first job.
	IdempotencyKey string `json:"idempotency_key"`
}

// Create checks access and queues the job.
func (s *PDFService) Create(ctx context.Context, userID string, in PDFJobInput) (*database.PDFJob, error) {
	if !s.Enabled() {
		return nil, ErrPDFToolsDisabled
	}
	if !pdftools.IsOperation(in.Operation) {
		return nil, fmt.Errorf("unknown PDF tool %q", in.Operation)
	}
	if err := in.Options.Validate(in.Operation, len(in.DocumentIDs)); err != nil {
		return nil, err
	}
	key := strings.TrimSpace(in.IdempotencyKey)
	if len(key) > 100 {
		return nil, fmt.Errorf("idempotency key is too long")
	}
	if key != "" {
		existing, err := queryRows[database.PDFJob](ctx, s.db, "SELECT * FROM pdf_jobs WHERE user_id = $user AND idempotency_key = $key LIMIT 1", map[string]any{"user": userID, "key": key})
		if err != nil {
			return nil, err
		}
		if len(existing) > 0 {
			return &existing[0], nil
		}
	}

	capability := "create_version"
	if in.Operation == pdftools.OpMerge {
		capability = "view"
	}
	var first *database.Document
	for _, id := range in.DocumentIDs {
		doc, err := s.docs.GetFor(ctx, userID, capability, id)
		if err != nil {
			return nil, err
		}
		if kind, _ := PreviewKind(doc.MimeType, doc.Name); kind != PreviewPDF {
			return nil, fmt.Errorf("%s isn't a PDF or an Office file that can become one", doc.Name)
		}
		if first == nil {
			first = doc
		}
	}
	folderID, name := "", ""
	if in.Operation == pdftools.OpMerge {
		folderID = first.FolderID
		if folderID != "" {
			if err := s.docs.access.RequireFolder(ctx, userID, "upload", folderID); err != nil {
				return nil, err
			}
		}
		name = strings.TrimSpace(in.Name)
		if name == "" {
			name = strings.TrimSuffix(first.Name, path.Ext(first.Name)) + " (merged)"
		}
		if !strings.EqualFold(path.Ext(name), ".pdf") {
			name += ".pdf"
		}
	}

	options := map[string]any{}
	raw, _ := json.Marshal(in.Options)
	_ = json.Unmarshal(raw, &options)
	now := time.Now().UTC()
	record := map[string]any{
		"org_id": orgOf(ctx), "user_id": userID, "operation": in.Operation, "document_ids": in.DocumentIDs,
		"options": options, "status": database.PDFJobQueued, "attempts": 0, "created_at": now, "updated_at": now,
	}
	if key != "" {
		record["idempotency_key"] = key
	}
	if folderID != "" {
		record["folder_id"] = folderID
	}
	if name != "" {
		record["output_name"] = name
	}
	job, err := surrealdb.Create[database.PDFJob](ctx, s.db, "pdf_jobs", record)
	if err != nil {
		return nil, err
	}
	_ = writeAudit(ctx, s.db, userID, "pdf.job_queued", "pdf_job", string(job.ID), map[string]any{"operation": in.Operation, "documents": in.DocumentIDs})
	return job, s.dispatch(ctx, job)
}

// dispatch hands the job to the queue, or runs it in the background when no
// queue is configured.
func (s *PDFService) dispatch(ctx context.Context, job *database.PDFJob) error {
	if s.queue != nil {
		// The task ID makes a double enqueue of the same attempt a no-op.
		return s.queue.Enqueue(ctx, queue.TaskPDF, queue.PDFPayload{JobID: string(job.ID)},
			asynq.MaxRetry(4), asynq.Timeout(15*time.Minute), asynq.TaskID(fmt.Sprintf("%s#%d", job.ID, job.Attempts)))
	}
	go func() {
		ctx := context.WithoutCancel(ctx)
		for attempt := 0; attempt < 3; attempt++ {
			err := s.Process(ctx, string(job.ID), attempt == 2)
			if err == nil || errors.Is(err, asynq.SkipRetry) {
				return
			}
			time.Sleep(time.Duration(attempt+1) * 10 * time.Second)
		}
	}()
	return nil
}

// Process runs a job. finalAttempt marks the job failed instead of retrying
// when it does not succeed. A job that already succeeded is left alone, so
// redelivered tasks never store a second output.
func (s *PDFService) Process(ctx context.Context, jobID string, finalAttempt bool) error {
	job, err := surrealdb.Select[database.PDFJob](ctx, s.db, mustRecordID(jobID))
	if err != nil || job == nil || job.ID == "" {
		return fmt.Errorf("%w: %v", asynq.SkipRetry, ErrPDFJobNotFound)
	}
	if job.Status == database.PDFJobSucceeded || job.Status == database.PDFJobFailed {
		return nil
	}
	ctx = WithActor(ctx, Actor{UserID: job.UserID, OrgID: job.OrgID})
	job = s.update(ctx, job, map[string]any{"status": database.PDFJobRunning, "attempts": job.Attempts + 1, "error": ""})

	err = s.run(ctx, job)
	if err == nil {
		observability.PDFJobs.WithLabelValues(job.Operation, database.PDFJobSucceeded).Inc()
		return nil
	}
	var httpErr *pdftools.HTTPError
	permanent := errors.Is(err, asynq.SkipRetry) || (errors.As(err, &httpErr) && !httpErr.Retryable()) ||
		errors.Is(err, ErrForbidden) || errors.Is(err, ErrDocumentNotFound) || errors.Is(err, ErrMalwareDetected)
	if permanent || finalAttempt {
		now := time.Now().UTC()
		s.update(ctx, job, map[string]any{"status": database.PDFJobFailed, "error": userMessage(err), "completed_at": now})
		s.log.Warn("PDF job failed", "job", jobID, "operation", job.Operation, "error", err)
		observability.PDFJobs.WithLabelValues(job.Operation, database.PDFJobFailed).Inc()
		return fmt.Errorf("%w: %v", asynq.SkipRetry, err)
	}
	s.update(ctx, job, map[string]any{"status": database.PDFJobRetrying, "error": userMessage(err)})
	return err
}

func (s *PDFService) run(ctx context.Context, job *database.PDFJob) error {
	var opts pdftools.Options
	raw, _ := json.Marshal(job.Options)
	_ = json.Unmarshal(raw, &opts)

	inputs := make([]pdftools.Input, 0, len(job.DocumentIDs))
	var first *database.Document
	for _, id := range job.DocumentIDs {
		doc, err := s.docs.GetFor(ctx, job.UserID, "view", id)
		if err != nil {
			return err
		}
		if first == nil {
			first = doc
		}
		rendition, err := s.previews.Rendition(ctx, doc)
		if err != nil {
			return fmt.Errorf("prepare %s: %w", doc.Name, err)
		}
		body, err := io.ReadAll(io.LimitReader(rendition.Body, maxPDFInput+1))
		_ = rendition.Body.Close()
		if err != nil {
			return err
		}
		if len(body) > maxPDFInput {
			return fmt.Errorf("%w: %s is too large for PDF tools", asynq.SkipRetry, doc.Name)
		}
		inputs = append(inputs, pdftools.Input{Name: doc.Name, Body: bytes.NewReader(body)})
	}
	out, err := s.client.Run(ctx, job.Operation, inputs, opts)
	if err != nil {
		return err
	}

	now := time.Now().UTC()
	if job.Operation == pdftools.OpMerge {
		doc, _, err := s.docs.Upload(ctx, UploadInput{
			OrgID: job.OrgID, OwnerID: job.UserID, FolderID: job.FolderID, Name: job.OutputName, MimeType: "application/pdf",
			File: bytes.NewReader(out), Metadata: map[string]any{"source": "pdf_tools", "merged_from": job.DocumentIDs, "pdf_job_id": string(job.ID)},
		})
		if err != nil {
			return err
		}
		s.update(ctx, job, map[string]any{"status": database.PDFJobSucceeded, "result_document_id": string(doc.ID), "completed_at": now})
	} else {
		name := strings.TrimSuffix(first.Name, path.Ext(first.Name)) + ".pdf"
		version, _, err := s.versions.AddVersion(ctx, job.UserID, string(first.ID), NewVersionInput{
			File: bytes.NewReader(out), FileName: name, MimeType: "application/pdf",
			Source: database.VersionSourceStirlingPDF, Summary: pdfSummary(job.Operation, opts),
		})
		if err != nil {
			return err
		}
		s.update(ctx, job, map[string]any{"status": database.PDFJobSucceeded, "result_document_id": string(first.ID), "result_version_id": string(version.ID), "completed_at": now})
	}
	_ = writeAudit(ctx, s.db, job.UserID, "pdf.job_succeeded", "pdf_job", string(job.ID), map[string]any{"operation": job.Operation})
	return nil
}

// update saves job fields and tells the person's open tabs.
func (s *PDFService) update(ctx context.Context, job *database.PDFJob, fields map[string]any) *database.PDFJob {
	fields["updated_at"] = time.Now().UTC()
	updated, err := surrealdb.Merge[database.PDFJob](ctx, s.db, mustRecordID(string(job.ID)), fields)
	if err != nil || updated == nil {
		s.log.Warn("update PDF job", "job", job.ID, "error", err)
		return job
	}
	publishUser(ctx, updated.UserID, "pdf_job", updated)
	return updated
}

// Get returns one of the person's jobs.
func (s *PDFService) Get(ctx context.Context, userID, id string) (*database.PDFJob, error) {
	rid, err := database.ParseRecordID(id)
	if err != nil || rid.Table != "pdf_jobs" {
		return nil, ErrPDFJobNotFound
	}
	job, err := surrealdb.Select[database.PDFJob](ctx, s.db, rid)
	if err != nil || job == nil || job.UserID != userID {
		return nil, ErrPDFJobNotFound
	}
	return job, nil
}

// List returns the person's recent jobs, optionally for one document.
func (s *PDFService) List(ctx context.Context, userID, documentID string) ([]database.PDFJob, error) {
	statement := "SELECT * FROM pdf_jobs WHERE user_id = $user AND org_id = $org"
	vars := map[string]any{"user": userID, "org": orgOf(ctx)}
	if documentID != "" {
		statement += " AND document_ids CONTAINS $document"
		vars["document"] = documentID
	}
	return queryRows[database.PDFJob](ctx, s.db, statement+" ORDER BY created_at DESC LIMIT 50", vars)
}

// Retry queues a failed job again.
func (s *PDFService) Retry(ctx context.Context, userID, id string) (*database.PDFJob, error) {
	job, err := s.Get(ctx, userID, id)
	if err != nil {
		return nil, err
	}
	if job.Status != database.PDFJobFailed {
		return nil, fmt.Errorf("only failed jobs can be retried")
	}
	job = s.update(ctx, job, map[string]any{"status": database.PDFJobQueued, "error": ""})
	return job, s.dispatch(ctx, job)
}

func pdfSummary(op string, opts pdftools.Options) string {
	switch op {
	case pdftools.OpCompress:
		return fmt.Sprintf("Compressed (level %d)", opts.Level)
	case pdftools.OpRotate:
		return fmt.Sprintf("Rotated pages %d°", opts.Angle)
	case pdftools.OpWatermark:
		return "Added watermark \"" + opts.Text + "\""
	case pdftools.OpPageNumbers:
		return "Added page numbers"
	}
	return "Changed with PDF tools"
}

// userMessage keeps errors short enough to show in the app.
func userMessage(err error) string {
	message := strings.TrimPrefix(err.Error(), asynq.SkipRetry.Error()+": ")
	if len(message) > 300 {
		message = message[:300]
	}
	return message
}
