// Package davfs exposes the EDMS drive as a WebDAV file system, so it can be
// mounted in Windows Explorer, macOS Finder or any WebDAV client. Every
// operation goes through the same services, and therefore the same access
// rules, as the web app.
package davfs

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"strings"
	"time"

	"golang.org/x/net/webdav"

	"github.com/xmtrxi/edms/internal/database"
	"github.com/xmtrxi/edms/internal/dto"
	"github.com/xmtrxi/edms/internal/service"
	"github.com/xmtrxi/edms/internal/storage"
)

// MyFiles is the virtual folder holding documents outside every folder.
const MyFiles = "My files"

// Services are the application services the file system delegates to.
type Services struct {
	Docs      *service.DocumentService
	Folders   *service.FolderService
	Versions  *service.VersionService
	Workspace *service.WorkspaceService
	Search    *service.SearchService
	Access    *service.AccessService
	Storage   storage.Provider
}

type FS struct{ s Services }

func New(services Services) *FS { return &FS{s: services} }

var _ webdav.FileSystem = (*FS)(nil)

// node is a resolved path: the root, My files, a folder or a document.
type node struct {
	kind   string
	name   string
	folder *dto.ManagedFolderResponse
	doc    *database.Document
}

const (
	kindRoot    = "root"
	kindMyFiles = "myfiles"
	kindFolder  = "folder"
	kindDoc     = "doc"
)

func userOf(ctx context.Context) (string, error) {
	actor, ok := service.ActorFrom(ctx)
	if !ok || actor.UserID == "" {
		return "", os.ErrPermission
	}
	return actor.UserID, nil
}

func split(name string) []string {
	clean := strings.Trim(path.Clean("/"+name), "/")
	if clean == "" {
		return nil
	}
	return strings.Split(clean, "/")
}

// view is one request's snapshot of the visible folder tree.
type view struct {
	fs      *FS
	user    string
	folders []dto.ManagedFolderResponse
}

func (f *FS) view(ctx context.Context) (*view, error) {
	user, err := userOf(ctx)
	if err != nil {
		return nil, err
	}
	folders, err := f.s.Folders.List(ctx, user)
	if err != nil {
		return nil, err
	}
	return &view{fs: f, user: user, folders: folders}, nil
}

func (v *view) childFolders(parentID string) []dto.ManagedFolderResponse {
	result := []dto.ManagedFolderResponse{}
	for _, folder := range v.folders {
		if folder.ParentID == parentID {
			result = append(result, folder)
		}
	}
	return result
}

// documents lists the documents directly in a folder ("root" for My files),
// giving duplicate names a " (n)" suffix so every entry is addressable.
func (v *view) documents(ctx context.Context, folderID string) ([]database.Document, []string, error) {
	result, err := v.fs.s.Search.Search(ctx, service.SearchInput{OwnerID: v.user, FolderID: folderID, Page: 1, PageSize: 2000})
	if err != nil {
		return nil, nil, err
	}
	docs := make([]database.Document, 0, len(result.Items))
	names := make([]string, 0, len(result.Items))
	seen := map[string]int{}
	for _, item := range result.Items {
		if item.Document.Status == database.DocumentStatusTrashed || item.Document.Status == database.DocumentStatusArchived {
			continue
		}
		name := item.Document.Name
		if n := seen[strings.ToLower(name)]; n > 0 {
			ext := path.Ext(name)
			name = fmt.Sprintf("%s (%d)%s", strings.TrimSuffix(name, ext), n+1, ext)
		}
		seen[strings.ToLower(item.Document.Name)]++
		docs = append(docs, item.Document)
		names = append(names, name)
	}
	return docs, names, nil
}

func (v *view) resolve(ctx context.Context, name string) (*node, error) {
	parts := split(name)
	if len(parts) == 0 {
		return &node{kind: kindRoot, name: "/"}, nil
	}
	current := &node{kind: kindRoot}
	for i, part := range parts {
		next, err := v.child(ctx, current, part)
		if err != nil {
			return nil, err
		}
		if next.kind == kindDoc && i != len(parts)-1 {
			return nil, os.ErrNotExist
		}
		current = next
	}
	return current, nil
}

func (v *view) child(ctx context.Context, parent *node, name string) (*node, error) {
	switch parent.kind {
	case kindRoot:
		if name == MyFiles {
			return &node{kind: kindMyFiles, name: MyFiles}, nil
		}
		for _, folder := range v.childFolders("") {
			if folder.Name == name {
				f := folder
				return &node{kind: kindFolder, name: name, folder: &f}, nil
			}
		}
		return nil, os.ErrNotExist
	case kindMyFiles, kindFolder:
		folderID := service.RootFolderID
		if parent.kind == kindFolder {
			folderID = parent.folder.ID
			for _, folder := range v.childFolders(folderID) {
				if folder.Name == name {
					f := folder
					return &node{kind: kindFolder, name: name, folder: &f}, nil
				}
			}
		}
		docs, names, err := v.documents(ctx, folderID)
		if err != nil {
			return nil, err
		}
		for i := range docs {
			if names[i] == name {
				return &node{kind: kindDoc, name: name, doc: &docs[i]}, nil
			}
		}
	}
	return nil, os.ErrNotExist
}

func (v *view) children(ctx context.Context, n *node) ([]os.FileInfo, error) {
	infos := []os.FileInfo{}
	switch n.kind {
	case kindRoot:
		infos = append(infos, dirInfo(MyFiles, time.Now()))
		for _, folder := range v.childFolders("") {
			infos = append(infos, dirInfo(folder.Name, folder.UpdatedAt))
		}
		return infos, nil
	case kindFolder:
		for _, folder := range v.childFolders(n.folder.ID) {
			infos = append(infos, dirInfo(folder.Name, folder.UpdatedAt))
		}
	}
	folderID := service.RootFolderID
	if n.kind == kindFolder {
		folderID = n.folder.ID
	}
	docs, names, err := v.documents(ctx, folderID)
	if err != nil {
		return nil, err
	}
	for i := range docs {
		infos = append(infos, fileInfo{name: names[i], size: docs[i].Size, modTime: docs[i].UpdatedAt})
	}
	return infos, nil
}

func (f *FS) Stat(ctx context.Context, name string) (os.FileInfo, error) {
	v, err := f.view(ctx)
	if err != nil {
		return nil, err
	}
	n, err := v.resolve(ctx, name)
	if err != nil {
		return nil, err
	}
	return n.info(), nil
}

func (n *node) info() os.FileInfo {
	switch n.kind {
	case kindDoc:
		return fileInfo{name: n.name, size: n.doc.Size, modTime: n.doc.UpdatedAt}
	case kindFolder:
		return dirInfo(n.name, n.folder.UpdatedAt)
	}
	return dirInfo(n.name, time.Now())
}

func (f *FS) Mkdir(ctx context.Context, name string, _ os.FileMode) error {
	v, err := f.view(ctx)
	if err != nil {
		return err
	}
	parent, err := v.resolve(ctx, path.Dir(path.Clean("/"+name)))
	if err != nil {
		return err
	}
	parentID := ""
	switch parent.kind {
	case kindFolder:
		parentID = parent.folder.ID
	case kindMyFiles:
		// My files holds documents only; folders belong to the organization.
		return os.ErrPermission
	}
	_, err = f.s.Folders.Create(ctx, v.user, service.FolderInput{Name: path.Base(name), ParentID: parentID, InheritPermissions: true})
	return mapError(err)
}

func (f *FS) RemoveAll(ctx context.Context, name string) error {
	v, err := f.view(ctx)
	if err != nil {
		return err
	}
	n, err := v.resolve(ctx, name)
	if err != nil {
		return err
	}
	switch n.kind {
	case kindDoc:
		return mapError(f.s.Workspace.MoveToTrash(ctx, v.user, string(n.doc.ID)))
	case kindFolder:
		return mapError(f.s.Folders.Delete(ctx, v.user, n.folder.ID))
	}
	return os.ErrPermission
}

func (f *FS) Rename(ctx context.Context, oldName, newName string) error {
	v, err := f.view(ctx)
	if err != nil {
		return err
	}
	n, err := v.resolve(ctx, oldName)
	if err != nil {
		return err
	}
	target, err := v.resolve(ctx, path.Dir(path.Clean("/"+newName)))
	if err != nil {
		return err
	}
	base := path.Base(newName)
	switch n.kind {
	case kindDoc:
		update := service.DocumentUpdate{Name: &base}
		switch target.kind {
		case kindFolder:
			if target.folder.ID != n.doc.FolderID {
				update.FolderID = &target.folder.ID
			}
		case kindMyFiles:
			if n.doc.FolderID != "" {
				root := service.RootFolderID
				update.FolderID = &root
			}
		default:
			return os.ErrPermission
		}
		_, err = f.s.Docs.Update(ctx, v.user, string(n.doc.ID), update)
		return mapError(err)
	case kindFolder:
		parentID := ""
		switch target.kind {
		case kindFolder:
			parentID = target.folder.ID
		case kindRoot:
		default:
			return os.ErrPermission
		}
		_, err = f.s.Folders.Update(ctx, v.user, n.folder.ID, service.FolderInput{
			Name: base, ParentID: parentID, Description: n.folder.Description, Color: n.folder.Color, InheritPermissions: n.folder.InheritPermissions,
		})
		return mapError(err)
	}
	return os.ErrPermission
}

func (f *FS) OpenFile(ctx context.Context, name string, flag int, _ os.FileMode) (webdav.File, error) {
	v, err := f.view(ctx)
	if err != nil {
		return nil, err
	}
	writing := flag&(os.O_WRONLY|os.O_RDWR|os.O_CREATE|os.O_TRUNC) != 0
	n, err := v.resolve(ctx, name)
	if err != nil && !(errors.Is(err, os.ErrNotExist) && writing) {
		return nil, err
	}
	if n != nil && n.kind != kindDoc {
		if writing {
			return nil, os.ErrPermission
		}
		return &dirFile{v: v, n: n, ctx: ctx}, nil
	}
	if writing {
		parent, err := v.resolve(ctx, path.Dir(path.Clean("/"+name)))
		if err != nil {
			return nil, err
		}
		if parent.kind != kindFolder && parent.kind != kindMyFiles {
			return nil, os.ErrPermission
		}
		temp, err := os.CreateTemp("", "edms-dav-*")
		if err != nil {
			return nil, err
		}
		return &writeFile{File: temp, fs: f, ctx: ctx, user: v.user, name: path.Base(name), parent: parent, existing: n}, nil
	}
	return &readFile{fs: f, ctx: ctx, n: n}, nil
}

func mapError(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, service.ErrForbidden), errors.Is(err, service.ErrAccessDenied):
		return os.ErrPermission
	case errors.Is(err, service.ErrDocumentNotFound), errors.Is(err, service.ErrFolderNotFound):
		return os.ErrNotExist
	case errors.Is(err, service.ErrFolderNameTaken):
		return os.ErrExist
	}
	return err
}

// dirFile lists a directory.
type dirFile struct {
	v      *view
	n      *node
	ctx    context.Context
	listed bool
}

func (d *dirFile) Close() error                   { return nil }
func (d *dirFile) Read([]byte) (int, error)       { return 0, os.ErrInvalid }
func (d *dirFile) Seek(int64, int) (int64, error) { return 0, os.ErrInvalid }
func (d *dirFile) Write([]byte) (int, error)      { return 0, os.ErrPermission }
func (d *dirFile) Stat() (os.FileInfo, error)     { return d.n.info(), nil }
func (d *dirFile) Readdir(count int) ([]fs.FileInfo, error) {
	if d.listed {
		if count > 0 {
			return nil, io.EOF
		}
		return nil, nil
	}
	d.listed = true
	return d.v.children(d.ctx, d.n)
}

// readFile downloads the document into a temporary file on first access, so
// clients can seek freely.
type readFile struct {
	fs   *FS
	ctx  context.Context
	n    *node
	temp *os.File
}

func (r *readFile) load() error {
	if r.temp != nil {
		return nil
	}
	user, err := userOf(r.ctx)
	if err != nil {
		return err
	}
	_, body, err := r.fs.s.Docs.Download(r.ctx, user, string(r.n.doc.ID))
	if err != nil {
		return mapError(err)
	}
	defer body.Close()
	temp, err := os.CreateTemp("", "edms-dav-read-*")
	if err != nil {
		return err
	}
	if _, err := io.Copy(temp, body); err != nil {
		_ = temp.Close()
		_ = os.Remove(temp.Name())
		return err
	}
	if _, err := temp.Seek(0, io.SeekStart); err != nil {
		return err
	}
	r.temp = temp
	return nil
}

func (r *readFile) Read(p []byte) (int, error) {
	if err := r.load(); err != nil {
		return 0, err
	}
	return r.temp.Read(p)
}

func (r *readFile) Seek(offset int64, whence int) (int64, error) {
	// Size queries do not need the bytes.
	if r.temp == nil && offset == 0 && whence == io.SeekEnd {
		return r.n.doc.Size, nil
	}
	if r.temp == nil && offset == 0 && whence == io.SeekStart {
		return 0, nil
	}
	if err := r.load(); err != nil {
		return 0, err
	}
	return r.temp.Seek(offset, whence)
}

func (r *readFile) Close() error {
	if r.temp == nil {
		return nil
	}
	name := r.temp.Name()
	err := r.temp.Close()
	_ = os.Remove(name)
	return err
}

func (r *readFile) Readdir(int) ([]fs.FileInfo, error) { return nil, os.ErrInvalid }
func (r *readFile) Stat() (os.FileInfo, error)         { return r.n.info(), nil }
func (r *readFile) Write([]byte) (int, error)          { return 0, os.ErrPermission }

// writeFile collects the upload and stores it on Close: a new version of an
// existing document, or a new document in the folder.
//
// The size is always read from the temporary file: io.Copy writes through
// the embedded *os.File's ReadFrom, so counting in Write would miss bytes.
type writeFile struct {
	*os.File
	fs       *FS
	ctx      context.Context
	user     string
	name     string
	parent   *node
	existing *node
}

func (w *writeFile) size() int64 {
	info, err := w.File.Stat()
	if err != nil {
		return 0
	}
	return info.Size()
}

func (w *writeFile) Stat() (os.FileInfo, error) {
	return fileInfo{name: w.name, size: w.size(), modTime: time.Now()}, nil
}

func (w *writeFile) Readdir(int) ([]fs.FileInfo, error) { return nil, os.ErrInvalid }

func (w *writeFile) Close() error {
	defer func() {
		_ = w.File.Close()
		_ = os.Remove(w.File.Name())
	}()
	if _, err := w.File.Seek(0, io.SeekStart); err != nil {
		return err
	}
	if w.existing != nil && w.existing.kind == kindDoc {
		_, _, err := w.fs.s.Versions.AddVersion(w.ctx, w.user, string(w.existing.doc.ID), service.NewVersionInput{
			File: w.File, FileName: w.name, Source: database.VersionSourceUpload, Summary: "Saved over WebDAV",
		})
		return mapError(err)
	}
	folderID := ""
	if w.parent.kind == kindFolder {
		folderID = w.parent.folder.ID
		if err := w.fs.s.Access.RequireFolder(w.ctx, w.user, "upload", folderID); err != nil {
			return os.ErrPermission
		}
	}
	// Clients often create an empty file before writing; skip that step.
	if w.size() == 0 {
		return nil
	}
	_, _, err := w.fs.s.Docs.Upload(w.ctx, service.UploadInput{
		OwnerID: w.user, FolderID: folderID, Name: w.name, MimeType: mimeFor(w.name), File: w.File,
		Metadata: map[string]any{"source": "webdav"},
	})
	return mapError(err)
}

func mimeFor(name string) string {
	switch strings.ToLower(path.Ext(name)) {
	case ".pdf":
		return "application/pdf"
	case ".png":
		return "image/png"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".txt":
		return "text/plain"
	case ".docx":
		return "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
	case ".xlsx":
		return "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
	case ".pptx":
		return "application/vnd.openxmlformats-officedocument.presentationml.presentation"
	}
	return "application/octet-stream"
}

type fileInfo struct {
	name    string
	size    int64
	modTime time.Time
	dir     bool
}

func dirInfo(name string, modTime time.Time) fileInfo {
	return fileInfo{name: name, modTime: modTime, dir: true}
}

func (i fileInfo) Name() string       { return i.name }
func (i fileInfo) Size() int64        { return i.size }
func (i fileInfo) ModTime() time.Time { return i.modTime }
func (i fileInfo) IsDir() bool        { return i.dir }
func (i fileInfo) Sys() any           { return nil }
func (i fileInfo) Mode() os.FileMode {
	if i.dir {
		return os.ModeDir | 0o755
	}
	return 0o644
}
