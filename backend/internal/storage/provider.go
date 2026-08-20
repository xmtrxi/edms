package storage

import (
	"context"
	"fmt"
	"io"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/xmtrxi/edms/internal/config"
	s3prov "github.com/xmtrxi/edms/internal/storage/s3"
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

// EnsureBucketer is optionally implemented by providers that can auto-create
// their backing store (e.g. an S3 bucket).
type EnsureBucketer interface {
	EnsureBucket(ctx context.Context) error
}

func NewProvider(ctx context.Context, cfg config.Config) (Provider, error) {
	switch cfg.StorageProvider {
	case "s3":
		awsConfig := aws.Config{
			Region:      cfg.S3.Region,
			Credentials: aws.NewCredentialsCache(credentials.NewStaticCredentialsProvider(cfg.S3.AccessKeyID, cfg.S3.SecretAccessKey, "")),
		}

		client := s3.NewFromConfig(awsConfig, func(o *s3.Options) {
			if cfg.S3.Endpoint != "" {
				o.BaseEndpoint = aws.String(cfg.S3.Endpoint)
			}
			if cfg.S3.UsePathStyle {
				o.UsePathStyle = true
			}
		})

		return s3prov.New(client, cfg.S3.Bucket), nil

	default:
		return nil, fmt.Errorf("unsupported storage provider: %q (supported: s3)", cfg.StorageProvider)
	}
}
