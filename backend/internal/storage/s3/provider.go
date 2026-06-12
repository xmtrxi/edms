package s3

import (
	"context"
	"io"

	"github.com/aws/aws-sdk-go-v2/service/s3"
)

type S3Provider struct {
	client *s3.Client
	bucket string
}

func New(client *s3.Client, bucket string) *S3Provider {
	return &S3Provider{
		client: client,
		bucket: bucket,
	}
}

func (s S3Provider) Name() string {
	return "s3"
}
func (s S3Provider) Upload(ctx context.Context, path string, file io.Reader) error {
	return nil
}
func (s S3Provider) Download(ctx context.Context, path string) (io.Reader, error) {
	return nil, nil
}
func (s S3Provider) Move(ctx context.Context, previousPath, newPath string) error {
	return nil
}
func (s S3Provider) Delete(ctx context.Context, path string) error {
	return nil
}
func (s S3Provider) HealthCheck(ctx context.Context) error {
	return nil
}
func (s S3Provider) Exists(ctx context.Context, path string) (bool, error) {
	return true, nil
}
func (s S3Provider) GetUrl(ctx context.Context, path string) (string, error) {
	return "", nil
}
