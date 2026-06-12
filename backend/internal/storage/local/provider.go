package local

import (
	"context"
	"io"
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
	return nil
}
func (s LocalProvider) Download(ctx context.Context, path string) (io.Reader, error) {
	return nil, nil
}
func (s LocalProvider) Move(ctx context.Context, previousPath, newPath string) error {
	return nil
}
func (s LocalProvider) Delete(ctx context.Context, path string) error {
	return nil
}
func (s LocalProvider) HealthCheck(ctx context.Context) error {
	return nil
}
func (s LocalProvider) Exists(ctx context.Context, path string) (bool, error) {
	return true, nil
}
func (s LocalProvider) GetUrl(ctx context.Context, path string) (string, error) {
	return "", nil
}
