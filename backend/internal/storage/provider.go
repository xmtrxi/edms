package storage

import (
	"context"
	"io"
)

type Provider interface {
	Name() string
	Upload(ctx context.Context, path string, file io.Reader) error
	Download(ctx context.Context, path string) (io.ReadCloser, error)
	Move(ctx context.Context, previousPath string, newPath string) error
	Delete(ctx context.Context, path string) error
	HealthCheck(ctx context.Context) error
	Exists(ctx context.Context, path string) (bool, error)
	GetUrl(ctx context.Context, path string) (string, error)
	List(ctx context.Context, prefix string, cursor *string, limit int) ([]string, *string, error)
}
