package davfs

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"slices"
	"testing"
	"time"

	"github.com/xmtrxi/edms/internal/database"
	"github.com/xmtrxi/edms/internal/dto"
	"github.com/xmtrxi/edms/internal/service"
)

// Most FS operations go through services backed by SurrealDB. These tests
// cover what can run without a database: path handling, error mapping, file
// info, the folder-tree walk at the root, and the file handles' local logic.

func TestSplit(t *testing.T) {
	tests := []struct {
		in   string
		want []string
	}{
		{"", nil},
		{"/", nil},
		{"//", nil},
		{".", nil},
		{"/My files", []string{"My files"}},
		{"/Finance/2026/report.pdf", []string{"Finance", "2026", "report.pdf"}},
		{"Finance/2026/", []string{"Finance", "2026"}},
		{"/Finance//2026/./x", []string{"Finance", "2026", "x"}},
		{"/../../etc/passwd", []string{"etc", "passwd"}},
		{"/Finance/../HR", []string{"HR"}},
	}
	for _, tc := range tests {
		if got := split(tc.in); !slices.Equal(got, tc.want) {
			t.Errorf("split(%q) = %#v, want %#v", tc.in, got, tc.want)
		}
	}
}

func TestMapError(t *testing.T) {
	other := errors.New("boom")
	tests := []struct {
		in   error
		want error
	}{
		{nil, nil},
		{service.ErrForbidden, os.ErrPermission},
		{service.ErrAccessDenied, os.ErrPermission},
		{fmt.Errorf("wrapped: %w", service.ErrAccessDenied), os.ErrPermission},
		{service.ErrDocumentNotFound, os.ErrNotExist},
		{service.ErrFolderNotFound, os.ErrNotExist},
		{service.ErrFolderNameTaken, os.ErrExist},
		{other, other},
	}
	for _, tc := range tests {
		if got := mapError(tc.in); got != tc.want {
			t.Errorf("mapError(%v) = %v, want %v", tc.in, got, tc.want)
		}
	}
}

func TestMimeFor(t *testing.T) {
	tests := map[string]string{
		"a.pdf":         "application/pdf",
		"A.PDF":         "application/pdf",
		"scan.png":      "image/png",
		"photo.jpg":     "image/jpeg",
		"photo.JPEG":    "image/jpeg",
		"notes.txt":     "text/plain",
		"memo.docx":     "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
		"budget.xlsx":   "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
		"deck.pptx":     "application/vnd.openxmlformats-officedocument.presentationml.presentation",
		"archive.zip":   "application/octet-stream",
		"no-extension":  "application/octet-stream",
		"dir.pdf/x.bin": "application/octet-stream",
	}
	for name, want := range tests {
		if got := mimeFor(name); got != want {
			t.Errorf("mimeFor(%q) = %q, want %q", name, got, want)
		}
	}
}

func TestFileInfo(t *testing.T) {
	now := time.Date(2026, 10, 1, 9, 0, 0, 0, time.UTC)
	dir := dirInfo("Finance", now)
	if !dir.IsDir() || dir.Mode() != os.ModeDir|0o755 || dir.Name() != "Finance" || !dir.ModTime().Equal(now) || dir.Size() != 0 || dir.Sys() != nil {
		t.Fatalf("unexpected dir info %+v mode %v", dir, dir.Mode())
	}
	file := fileInfo{name: "a.pdf", size: 12, modTime: now}
	if file.IsDir() || file.Mode() != 0o644 || file.Size() != 12 {
		t.Fatalf("unexpected file info %+v mode %v", file, file.Mode())
	}
}

func TestNodeInfo(t *testing.T) {
	updated := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	doc := &node{kind: kindDoc, name: "a.pdf", doc: &database.Document{Size: 99, UpdatedAt: updated}}
	if info := doc.info(); info.IsDir() || info.Size() != 99 || info.Name() != "a.pdf" || !info.ModTime().Equal(updated) {
		t.Fatalf("doc info %+v", info)
	}
	folder := &node{kind: kindFolder, name: "HR", folder: &dto.ManagedFolderResponse{UpdatedAt: updated}}
	if info := folder.info(); !info.IsDir() || info.Name() != "HR" || !info.ModTime().Equal(updated) {
		t.Fatalf("folder info %+v", info)
	}
	for _, n := range []*node{{kind: kindRoot, name: "/"}, {kind: kindMyFiles, name: MyFiles}} {
		if info := n.info(); !info.IsDir() || info.Name() != n.name {
			t.Fatalf("%s info %+v", n.kind, info)
		}
	}
}

func TestOperationsRequireActor(t *testing.T) {
	// Without an actor every operation is refused before any service is used,
	// so a zero Services value is enough.
	f := New(Services{})
	ctx := context.Background()
	if _, err := f.Stat(ctx, "/"); !errors.Is(err, os.ErrPermission) {
		t.Fatalf("Stat: %v", err)
	}
	if err := f.Mkdir(ctx, "/x", 0); !errors.Is(err, os.ErrPermission) {
		t.Fatalf("Mkdir: %v", err)
	}
	if err := f.RemoveAll(ctx, "/x"); !errors.Is(err, os.ErrPermission) {
		t.Fatalf("RemoveAll: %v", err)
	}
	if err := f.Rename(ctx, "/x", "/y"); !errors.Is(err, os.ErrPermission) {
		t.Fatalf("Rename: %v", err)
	}
	if _, err := f.OpenFile(ctx, "/x", os.O_RDONLY, 0); !errors.Is(err, os.ErrPermission) {
		t.Fatalf("OpenFile: %v", err)
	}
	if _, err := userOf(service.WithActor(ctx, service.Actor{})); !errors.Is(err, os.ErrPermission) {
		t.Fatalf("an actor without a user ID must be refused: %v", err)
	}
	if user, err := userOf(service.WithActor(ctx, service.Actor{UserID: "user:1"})); err != nil || user != "user:1" {
		t.Fatalf("userOf = %q, %v", user, err)
	}
}

func testView() *view {
	updated := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
	return &view{fs: New(Services{}), user: "user:1", folders: []dto.ManagedFolderResponse{
		{ID: "folder:finance", Name: "Finance", UpdatedAt: updated},
		{ID: "folder:hr", Name: "HR", UpdatedAt: updated},
		{ID: "folder:2026", ParentID: "folder:finance", Name: "2026", UpdatedAt: updated},
	}}
}

func TestChildFolders(t *testing.T) {
	v := testView()
	names := func(folders []dto.ManagedFolderResponse) []string {
		out := []string{}
		for _, f := range folders {
			out = append(out, f.Name)
		}
		return out
	}
	if got := names(v.childFolders("")); !slices.Equal(got, []string{"Finance", "HR"}) {
		t.Fatalf("top level = %v", got)
	}
	if got := names(v.childFolders("folder:finance")); !slices.Equal(got, []string{"2026"}) {
		t.Fatalf("Finance children = %v", got)
	}
	if got := v.childFolders("folder:hr"); got == nil || len(got) != 0 {
		t.Fatalf("HR children should be empty non-nil, got %#v", got)
	}
}

func TestResolveTopLevel(t *testing.T) {
	v := testView()
	ctx := context.Background()
	tests := []struct {
		path     string
		kind     string
		name     string
		folderID string
	}{
		{"", kindRoot, "/", ""},
		{"/", kindRoot, "/", ""},
		{"/../..", kindRoot, "/", ""},
		{"/My files", kindMyFiles, MyFiles, ""},
		{"/My files/", kindMyFiles, MyFiles, ""},
		{"/Finance", kindFolder, "Finance", "folder:finance"},
		{"HR/", kindFolder, "HR", "folder:hr"},
		// A subfolder is found before any document lookup is needed.
		{"/Finance/2026", kindFolder, "2026", "folder:2026"},
	}
	for _, tc := range tests {
		n, err := v.resolve(ctx, tc.path)
		if err != nil {
			t.Fatalf("resolve(%q): %v", tc.path, err)
		}
		if n.kind != tc.kind || n.name != tc.name {
			t.Fatalf("resolve(%q) = %s %q, want %s %q", tc.path, n.kind, n.name, tc.kind, tc.name)
		}
		if tc.folderID != "" && n.folder.ID != tc.folderID {
			t.Fatalf("resolve(%q) folder = %q, want %q", tc.path, n.folder.ID, tc.folderID)
		}
	}
	for _, missing := range []string{"/Nope", "/finance", "/2026"} {
		if _, err := v.resolve(ctx, missing); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("resolve(%q): expected not-exist, got %v", missing, err)
		}
	}
}

func TestRootListing(t *testing.T) {
	v := testView()
	d := &dirFile{v: v, n: &node{kind: kindRoot, name: "/"}, ctx: context.Background()}
	infos, err := d.Readdir(0)
	if err != nil {
		t.Fatalf("Readdir: %v", err)
	}
	names := []string{}
	for _, info := range infos {
		if !info.IsDir() {
			t.Fatalf("root entries are all directories, got file %q", info.Name())
		}
		names = append(names, info.Name())
	}
	if !slices.Equal(names, []string{MyFiles, "Finance", "HR"}) {
		t.Fatalf("root listing = %v", names)
	}

	// A directory is listed once; afterwards Readdir reports the end.
	if infos, err := d.Readdir(0); err != nil || infos != nil {
		t.Fatalf("second Readdir(0) = %v, %v", infos, err)
	}
	if _, err := d.Readdir(10); err != io.EOF {
		t.Fatalf("second Readdir(10): expected EOF, got %v", err)
	}
}

func TestDirFileIsNotReadableOrWritable(t *testing.T) {
	d := &dirFile{v: testView(), n: &node{kind: kindRoot, name: "/"}}
	if _, err := d.Read(make([]byte, 1)); !errors.Is(err, os.ErrInvalid) {
		t.Fatalf("Read: %v", err)
	}
	if _, err := d.Write([]byte("x")); !errors.Is(err, os.ErrPermission) {
		t.Fatalf("Write: %v", err)
	}
	if _, err := d.Seek(0, io.SeekStart); !errors.Is(err, os.ErrInvalid) {
		t.Fatalf("Seek: %v", err)
	}
	if info, err := d.Stat(); err != nil || !info.IsDir() {
		t.Fatalf("Stat = %v, %v", info, err)
	}
	if err := d.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
}

func TestReadFileSizeQueriesDoNotDownload(t *testing.T) {
	// No actor and no services: any attempt to download would fail.
	r := &readFile{fs: New(Services{}), ctx: context.Background(), n: &node{kind: kindDoc, name: "a.pdf", doc: &database.Document{Size: 1234}}}
	if size, err := r.Seek(0, io.SeekEnd); err != nil || size != 1234 {
		t.Fatalf("Seek(0, end) = %d, %v", size, err)
	}
	if pos, err := r.Seek(0, io.SeekStart); err != nil || pos != 0 {
		t.Fatalf("Seek(0, start) = %d, %v", pos, err)
	}
	if r.temp != nil {
		t.Fatal("size queries must not download the document")
	}
	// Real reads need the content and are refused without a user.
	if _, err := r.Read(make([]byte, 1)); !errors.Is(err, os.ErrPermission) {
		t.Fatalf("Read: %v", err)
	}
	if _, err := r.Seek(10, io.SeekStart); !errors.Is(err, os.ErrPermission) {
		t.Fatalf("Seek(10): %v", err)
	}
	if _, err := r.Write([]byte("x")); !errors.Is(err, os.ErrPermission) {
		t.Fatalf("Write: %v", err)
	}
	if _, err := r.Readdir(0); !errors.Is(err, os.ErrInvalid) {
		t.Fatalf("Readdir: %v", err)
	}
	if info, err := r.Stat(); err != nil || info.Size() != 1234 || info.IsDir() {
		t.Fatalf("Stat = %v, %v", info, err)
	}
	if err := r.Close(); err != nil {
		t.Fatalf("Close without a download: %v", err)
	}
}

func TestReadFileCloseRemovesTempFile(t *testing.T) {
	temp, err := os.CreateTemp(t.TempDir(), "read-*")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := temp.WriteString("content"); err != nil {
		t.Fatal(err)
	}
	if _, err := temp.Seek(0, io.SeekStart); err != nil {
		t.Fatal(err)
	}
	r := &readFile{n: &node{kind: kindDoc, doc: &database.Document{Size: 7}}, temp: temp}
	// Once loaded, reads and seeks go to the temporary file.
	buf := make([]byte, 4)
	if n, err := r.Read(buf); err != nil || string(buf[:n]) != "cont" {
		t.Fatalf("Read = %q, %v", buf[:n], err)
	}
	if pos, err := r.Seek(0, io.SeekEnd); err != nil || pos != 7 {
		t.Fatalf("Seek(end) = %d, %v", pos, err)
	}
	if err := r.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if _, err := os.Stat(temp.Name()); !os.IsNotExist(err) {
		t.Fatalf("temporary file should be removed, stat err %v", err)
	}
}

func TestWriteFileEmptyNewFileIsSkipped(t *testing.T) {
	// Clients create an empty file before writing. With no content and no
	// existing document, Close in My files must not touch any service.
	temp, err := os.CreateTemp(t.TempDir(), "dav-*")
	if err != nil {
		t.Fatal(err)
	}
	w := &writeFile{File: temp, fs: New(Services{}), ctx: context.Background(), user: "user:1", name: "new.docx", parent: &node{kind: kindMyFiles, name: MyFiles}}
	if info, err := w.Stat(); err != nil || info.Name() != "new.docx" || info.Size() != 0 || info.IsDir() {
		t.Fatalf("Stat = %v, %v", info, err)
	}
	if _, err := w.Readdir(0); !errors.Is(err, os.ErrInvalid) {
		t.Fatalf("Readdir: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if _, err := os.Stat(temp.Name()); !os.IsNotExist(err) {
		t.Fatalf("temporary file should be removed, stat err %v", err)
	}
}

func TestWriteFileSizeCountsCopiedBytes(t *testing.T) {
	temp, err := os.CreateTemp(t.TempDir(), "dav-*")
	if err != nil {
		t.Fatal(err)
	}
	defer temp.Close()
	w := &writeFile{File: temp, name: "a.txt"}
	// io.Copy may use the embedded *os.File's ReadFrom; size must still be right.
	if _, err := io.Copy(w, io.LimitReader(zeroReader{}, 5000)); err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write([]byte("tail")); err != nil {
		t.Fatal(err)
	}
	if info, _ := w.Stat(); info.Size() != 5004 {
		t.Fatalf("size = %d, want 5004", info.Size())
	}
}

type zeroReader struct{}

func (zeroReader) Read(p []byte) (int, error) {
	clear(p)
	return len(p), nil
}
