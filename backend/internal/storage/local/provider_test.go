package local

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func upload(t *testing.T, p *LocalProvider, path, content string) {
	t.Helper()
	if err := p.Upload(context.Background(), path, strings.NewReader(content)); err != nil {
		t.Fatalf("Upload(%q): %v", path, err)
	}
}

func read(t *testing.T, p *LocalProvider, path string) string {
	t.Helper()
	body, err := p.Download(context.Background(), path)
	if err != nil {
		t.Fatalf("Download(%q): %v", path, err)
	}
	defer func() { _ = body.Close() }()
	data, err := io.ReadAll(body)
	if err != nil {
		t.Fatalf("read %q: %v", path, err)
	}
	return string(data)
}

func TestUploadDownloadRoundTrip(t *testing.T) {
	root := t.TempDir()
	p := New(root)
	upload(t, p, "org/a/b/report.pdf", "hello")
	if got := read(t, p, "org/a/b/report.pdf"); got != "hello" {
		t.Fatalf("got %q", got)
	}
	if _, err := os.Stat(filepath.Join(root, "org", "a", "b", "report.pdf")); err != nil {
		t.Fatalf("file not written under root: %v", err)
	}
	// Overwrite truncates the previous content.
	upload(t, p, "org/a/b/report.pdf", "hi")
	if got := read(t, p, "org/a/b/report.pdf"); got != "hi" {
		t.Fatalf("overwrite: got %q", got)
	}
}

func TestPathsCannotEscapeRoot(t *testing.T) {
	parent := t.TempDir()
	root := filepath.Join(parent, "store")
	p := New(root)
	ctx := context.Background()

	for _, path := range []string{"../escape.txt", "../../escape.txt", "/../escape.txt", "a/../../escape.txt", "/etc/../../escape.txt"} {
		t.Run(path, func(t *testing.T) {
			upload(t, p, path, "x")
			if _, err := os.Stat(filepath.Join(parent, "escape.txt")); err == nil {
				t.Fatalf("upload of %q escaped the storage root", path)
			}
			resolved, err := p.GetUrl(ctx, path)
			if err != nil {
				t.Fatal(err)
			}
			if !strings.HasPrefix(resolved, root+string(filepath.Separator)) {
				t.Fatalf("GetUrl(%q) = %q, outside root %q", path, resolved, root)
			}
		})
	}
	if _, err := os.Stat(filepath.Join(root, "escape.txt")); err != nil {
		t.Fatalf("traversal paths should be clamped into the root: %v", err)
	}

	// Reading, deleting and moving are clamped the same way.
	outside := filepath.Join(parent, "secret.txt")
	if err := os.WriteFile(outside, []byte("secret"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := p.Download(ctx, "../secret.txt"); !os.IsNotExist(err) {
		t.Fatalf("Download outside root: expected not-exist, got %v", err)
	}
	if exists, err := p.Exists(ctx, "../secret.txt"); err != nil || exists {
		t.Fatalf("Exists outside root = %v, %v", exists, err)
	}
	if err := p.Delete(ctx, "../secret.txt"); !os.IsNotExist(err) {
		t.Fatalf("Delete outside root: expected not-exist, got %v", err)
	}
	if err := p.Move(ctx, "../secret.txt", "stolen.txt"); !os.IsNotExist(err) {
		t.Fatalf("Move from outside root: expected not-exist, got %v", err)
	}
	if _, err := os.Stat(outside); err != nil {
		t.Fatalf("file outside root was touched: %v", err)
	}
}

func TestExistsAndDelete(t *testing.T) {
	p := New(t.TempDir())
	ctx := context.Background()
	if exists, err := p.Exists(ctx, "missing.txt"); err != nil || exists {
		t.Fatalf("Exists(missing) = %v, %v", exists, err)
	}
	upload(t, p, "dir/file.txt", "x")
	if exists, err := p.Exists(ctx, "dir/file.txt"); err != nil || !exists {
		t.Fatalf("Exists(file) = %v, %v", exists, err)
	}
	if err := p.Delete(ctx, "dir/file.txt"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if exists, _ := p.Exists(ctx, "dir/file.txt"); exists {
		t.Fatal("file still exists after Delete")
	}
	if err := p.Delete(ctx, "dir/file.txt"); !os.IsNotExist(err) {
		t.Fatalf("second Delete: expected not-exist, got %v", err)
	}
}

func TestMoveCreatesDestinationDirectories(t *testing.T) {
	p := New(t.TempDir())
	ctx := context.Background()
	upload(t, p, "inbox/a.txt", "payload")
	if err := p.Move(ctx, "inbox/a.txt", "archive/2026/10/a.txt"); err != nil {
		t.Fatalf("Move: %v", err)
	}
	if exists, _ := p.Exists(ctx, "inbox/a.txt"); exists {
		t.Fatal("source still exists after Move")
	}
	if got := read(t, p, "archive/2026/10/a.txt"); got != "payload" {
		t.Fatalf("moved content = %q", got)
	}
}

func TestHealthCheck(t *testing.T) {
	if err := New("").HealthCheck(context.Background()); err == nil {
		t.Fatal("expected error for empty path")
	}
	root := filepath.Join(t.TempDir(), "nested", "store")
	if err := New(root).HealthCheck(context.Background()); err != nil {
		t.Fatalf("HealthCheck: %v", err)
	}
	if info, err := os.Stat(root); err != nil || !info.IsDir() {
		t.Fatalf("HealthCheck should create the root directory: %v", err)
	}
	file := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(file, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := New(file).HealthCheck(context.Background()); err == nil {
		t.Fatal("expected error when root is a regular file")
	}
}

func TestListPagination(t *testing.T) {
	p := New(t.TempDir())
	ctx := context.Background()
	for _, path := range []string{"docs/c.txt", "docs/a.txt", "docs/sub/b.txt", "docs/d.txt", "other/x.txt"} {
		upload(t, p, path, "x")
	}

	all, next, err := p.List(ctx, "docs", nil, 0)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	want := []string{"docs/a.txt", "docs/c.txt", "docs/d.txt", "docs/sub/b.txt"}
	if !slices.Equal(all, want) || next != nil {
		t.Fatalf("List(no limit) = %v, next %v; want %v, nil", all, next, want)
	}

	var pages [][]string
	var cursor *string
	for range 10 {
		items, nextCursor, err := p.List(ctx, "docs", cursor, 3)
		if err != nil {
			t.Fatalf("List page: %v", err)
		}
		pages = append(pages, items)
		if nextCursor == nil {
			break
		}
		cursor = nextCursor
	}
	if len(pages) != 2 || !slices.Equal(pages[0], want[:3]) || !slices.Equal(pages[1], want[3:]) {
		t.Fatalf("unexpected pages %v", pages)
	}

	tests := []struct {
		name   string
		cursor string
		want   []string
	}{
		{"past end", "99", []string{}},
		{"invalid cursor starts at zero", "nope", want[:2]},
		{"negative cursor starts at zero", "-1", want[:2]},
		{"empty cursor starts at zero", "", want[:2]},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cursor := tc.cursor
			items, _, err := p.List(ctx, "docs", &cursor, 2)
			if err != nil {
				t.Fatalf("List: %v", err)
			}
			if !slices.Equal(items, tc.want) || items == nil {
				t.Fatalf("got %#v, want %#v", items, tc.want)
			}
		})
	}
}

func TestListWholeStoreAndTraversalPrefix(t *testing.T) {
	p := New(t.TempDir())
	upload(t, p, "a.txt", "x")
	upload(t, p, "b/c.txt", "x")
	for _, prefix := range []string{"", "/", "..", "../.."} {
		items, _, err := p.List(context.Background(), prefix, nil, 0)
		if err != nil {
			t.Fatalf("List(%q): %v", prefix, err)
		}
		if !slices.Equal(items, []string{"a.txt", "b/c.txt"}) {
			t.Fatalf("List(%q) = %v", prefix, items)
		}
	}
}

func TestListHonoursCancelledContext(t *testing.T) {
	p := New(t.TempDir())
	upload(t, p, "a.txt", "x")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, _, err := p.List(ctx, "", nil, 0); err == nil {
		t.Fatal("expected cancelled context to abort List")
	}
}

func TestListMissingPrefixIsEmpty(t *testing.T) {
	items, next, err := New(t.TempDir()).List(context.Background(), "never-written", nil, 0)
	if err != nil || len(items) != 0 || next != nil {
		t.Fatalf("expected empty listing, got %v, %v, %v", items, next, err)
	}
}

func TestName(t *testing.T) {
	if New("x").Name() != "local" {
		t.Fatal("unexpected provider name")
	}
}
