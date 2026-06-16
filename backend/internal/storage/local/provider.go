package local

import (
	"context"
	"fmt"
	"io"
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
func (s LocalProvider) Download(ctx context.Context, path string) (io.Reader, error) {
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
