package local

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
)

type LocalProvider struct {
	path string
}

func New(path string) *LocalProvider {
	return &LocalProvider{
		path: path,
	}
}
func (s LocalProvider) Name() string {
	return "local"
}
func (s LocalProvider) Upload(ctx context.Context, path string, file io.Reader) error {
	filePath := filepath.Base(path)
	dst, err := os.Create(filepath.Join(s.path + filePath))
	if err != nil {
		return err
	}
	defer dst.Close()
	io.Copy(dst, file)
	return nil
}
func (s LocalProvider) Download(ctx context.Context, path string) (io.ReadCloser, error) {
	return os.Open(path)
}
func (s LocalProvider) Move(ctx context.Context, previousPath, newPath string) error {
	return os.Rename(newPath, previousPath)
}
func (s LocalProvider) Delete(ctx context.Context, path string) error {
	return os.Remove(path)
}
func (s LocalProvider) HealthCheck(ctx context.Context) error {
	return nil
}
func (s LocalProvider) Exists(ctx context.Context, path string) (bool, error) {
	_, err := os.Stat(path)
	if err != nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, nil
}
func (s LocalProvider) GetUrl(ctx context.Context, path string) (string, error) {
	return fmt.Sprintf("%s/files/%s", s.path, path), nil
}

func (s LocalProvider) List(ctx context.Context, prefix string, cursor *string, limit int) ([]string, *string, error) {
	var files []string
	err := filepath.Walk(s.path, func(path string, info fs.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		relative, err := filepath.Rel(s.path, path)
		if err != nil {
			return err
		}
		files = append(files, relative)
		return nil
	})
	if err != nil {
		return nil, nil, err
	}
	return files, nil, nil
}
