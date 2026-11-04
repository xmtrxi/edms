package service

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/xmtrxi/edms/internal/scan"
	"github.com/xmtrxi/edms/internal/testdb"
)

type fakeScanner struct{ err error }

func (f fakeScanner) Scan(_ context.Context, r io.Reader) error {
	_, _ = io.Copy(io.Discard, r)
	return f.err
}
func (f fakeScanner) Ping(context.Context) error { return nil }

func withScanner(t *testing.T, s scan.Scanner, required bool) {
	t.Helper()
	UseScanner(s, required)
	t.Cleanup(func() { UseScanner(nil, false) })
}

func TestCheckFileRewindsCleanFiles(t *testing.T) {
	withScanner(t, fakeScanner{}, false)
	file := strings.NewReader("hello")
	if err := checkFile(context.Background(), nil, file, "a.txt"); err != nil {
		t.Fatal(err)
	}
	rest, _ := io.ReadAll(file)
	if string(rest) != "hello" {
		t.Fatalf("file not rewound after scanning, read %q", rest)
	}
}

func TestCheckFileWhenScannerIsDown(t *testing.T) {
	down := fakeScanner{err: errors.New("connection refused")}
	withScanner(t, down, false)
	if err := checkFile(context.Background(), nil, strings.NewReader("x"), "a.txt"); err != nil {
		t.Fatalf("optional scanning should let the file through, got %v", err)
	}
	withScanner(t, down, true)
	if err := checkFile(context.Background(), nil, strings.NewReader("x"), "a.txt"); !errors.Is(err, ErrScanUnavailable) {
		t.Fatalf("required scanning should refuse, got %v", err)
	}
}

func TestCheckFileBlocksMalware(t *testing.T) {
	db := testdb.Open(t)
	withScanner(t, fakeScanner{err: &scan.Infected{Signature: "Eicar-Test"}}, false)
	if err := checkFile(context.Background(), db, strings.NewReader("x"), "evil.pdf"); !errors.Is(err, ErrMalwareDetected) {
		t.Fatalf("expected malware to be refused, got %v", err)
	}
}
