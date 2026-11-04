package service

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/xmtrxi/edms/internal/config"
	"github.com/xmtrxi/edms/internal/database"
	"github.com/xmtrxi/edms/internal/pdftools"
	"github.com/xmtrxi/edms/internal/storage/local"
)

func TestPDFJobStoresExactlyOneVersion(t *testing.T) {
	auth := newTestAuth(t, nil)
	ctx := context.Background()
	ada, _ := auth.Register(ctx, RegisterInput{Email: "ada@example.com", Password: "correct horse"})
	actorCtx := WithActor(ctx, Actor{UserID: string(ada.ID), OrgID: ada.LastOrgID, OrgRole: database.MembershipRoleOwner})

	var calls atomic.Int32
	stirling := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		_, _ = io.WriteString(w, "%PDF-1.7 rotated")
	}))
	defer stirling.Close()

	provider := local.New(t.TempDir())
	cfg := config.Config{StorageProvider: "local"}
	docs := NewDocumentService(cfg, auth.DB, provider, nil, nil, nil)
	versions := NewVersionService(auth.DB, provider, docs)
	pdf := NewPDFService(auth.DB, pdftools.New(stirling.URL, "", time.Minute), docs, versions, NewPreviewService(docs, provider, nil), nil, nil)

	doc, _, err := docs.Upload(actorCtx, UploadInput{OrgID: ada.LastOrgID, OwnerID: string(ada.ID), Name: "Report.pdf", MimeType: "application/pdf", File: strings.NewReader("%PDF-1.4 original")})
	if err != nil {
		t.Fatal(err)
	}

	in := PDFJobInput{Operation: pdftools.OpRotate, DocumentIDs: []string{string(doc.ID)}, Options: pdftools.Options{Angle: 90}, IdempotencyKey: "click-1"}
	job, err := pdf.Create(actorCtx, string(ada.ID), in)
	if err != nil {
		t.Fatal(err)
	}
	again, err := pdf.Create(actorCtx, string(ada.ID), in)
	if err != nil || again.ID != job.ID {
		t.Fatalf("the same idempotency key should return the first job, got %v (%v)", again.ID, err)
	}

	var done *database.PDFJob
	for deadline := time.Now().Add(15 * time.Second); time.Now().Before(deadline); time.Sleep(100 * time.Millisecond) {
		done, _ = pdf.Get(actorCtx, string(ada.ID), string(job.ID))
		if done != nil && (done.Status == database.PDFJobSucceeded || done.Status == database.PDFJobFailed) {
			break
		}
	}
	if done == nil || done.Status != database.PDFJobSucceeded || done.ResultVersionID == "" {
		t.Fatalf("job did not succeed: %+v", done)
	}

	// A redelivered task must not store a second version.
	if err := pdf.Process(actorCtx, string(job.ID), false); err != nil {
		t.Fatal(err)
	}
	if calls.Load() != 1 {
		t.Fatalf("Stirling called %d times, want 1", calls.Load())
	}
	history, err := versions.List(actorCtx, string(ada.ID), string(doc.ID))
	if err != nil {
		t.Fatal(err)
	}
	if len(history) != 2 {
		t.Fatalf("expected the original and one new version, got %d", len(history))
	}

	jobs, err := pdf.List(actorCtx, string(ada.ID), string(doc.ID))
	if err != nil || len(jobs) != 1 {
		t.Fatalf("listing by document: %d jobs, %v", len(jobs), err)
	}
	if _, err := pdf.Create(actorCtx, string(ada.ID), PDFJobInput{Operation: pdftools.OpMerge, DocumentIDs: []string{string(doc.ID)}}); err == nil {
		t.Fatal("merging a single file should be refused")
	}
}
