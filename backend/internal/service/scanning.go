package service

import (
	"context"
	"errors"
	"io"
	"log/slog"

	"github.com/surrealdb/surrealdb.go"

	"github.com/xmtrxi/edms/internal/observability"
	"github.com/xmtrxi/edms/internal/scan"
)

var (
	ErrMalwareDetected = errors.New("this file was blocked because it contains malware")
	ErrScanUnavailable = errors.New("files can't be checked for malware right now; try again later")
)

// scanner checks uploads before they are stored; nil turns scanning off.
// It is set once at startup.
var (
	scanner      scan.Scanner
	scanRequired bool
)

// UseScanner turns on malware scanning. When required, uploads are refused
// while the scanner can't be reached.
func UseScanner(s scan.Scanner, required bool) {
	scanner, scanRequired = s, required
}

// ScanningEnabled reports whether uploads are scanned.
func ScanningEnabled() bool { return scanner != nil }

// checkFile scans a file about to be stored and rewinds it. Infected files
// are refused and recorded in the audit log.
func checkFile(ctx context.Context, db *surrealdb.DB, file io.ReadSeeker, name string) error {
	if scanner == nil {
		return nil
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return err
	}
	err := scanner.Scan(ctx, file)
	if _, seekErr := file.Seek(0, io.SeekStart); seekErr != nil {
		return seekErr
	}
	var infected *scan.Infected
	switch {
	case err == nil:
		observability.MalwareScans.WithLabelValues("clean").Inc()
		return nil
	case errors.As(err, &infected):
		observability.MalwareScans.WithLabelValues("infected").Inc()
		observability.Uploads.WithLabelValues("blocked").Inc()
		actor, _ := ActorFrom(ctx)
		slog.Warn("blocked infected upload", "name", name, "signature", infected.Signature, "actor", actor.UserID)
		_ = writeAudit(ctx, db, actor.UserID, "upload.malware_blocked", "file", name, map[string]any{"signature": infected.Signature})
		return ErrMalwareDetected
	}
	observability.MalwareScans.WithLabelValues("error").Inc()
	if scanRequired {
		slog.Error("malware scan failed; upload refused", "name", name, "error", err)
		return ErrScanUnavailable
	}
	slog.Warn("malware scan failed; storing unscanned", "name", name, "error", err)
	return nil
}
