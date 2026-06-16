package s3

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

type S3Provider struct {
	client  *s3.Client
	bucket  string
	presign *s3.PresignClient
}

func New(client *s3.Client, bucket string) *S3Provider {
	return &S3Provider{
		client:  client,
		bucket:  bucket,
		presign: s3.NewPresignClient(client),
	}
}

func (s S3Provider) Name() string {
	return "s3"
}
func (s S3Provider) Upload(ctx context.Context, path string, file io.Reader) error {
	_, err := s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(s.bucket),
		Body:   file,
		Key:    aws.String(path),
	})
	return err
}
func (s S3Provider) Download(ctx context.Context, path string) (io.ReadCloser, error) {
	resp, err := s.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(path),
	})
	if err != nil {
		return nil, err
	}
	return resp.Body, nil
}
func (s S3Provider) Move(ctx context.Context, previousPath, newPath string) error {
	_, err := s.client.CopyObject(ctx, &s3.CopyObjectInput{
		Bucket:     aws.String(s.bucket),
		Key:        aws.String(newPath),
		CopySource: aws.String(fmt.Sprintf("%s/%s", s.bucket, previousPath)),
	})
	if err != nil {
		return err
	}
	_, err = s.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(previousPath),
	})
	return err
}
func (s S3Provider) Delete(ctx context.Context, path string) error {
	_, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(path),
	})
	return err
}
func (s S3Provider) HealthCheck(ctx context.Context) error {
	_, err := s.client.HeadBucket(ctx, &s3.HeadBucketInput{
		Bucket: aws.String(s.bucket),
	})
	return err
}
func (s S3Provider) Exists(ctx context.Context, path string) (bool, error) {
	_, err := s.client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(path),
	})
	if err != nil {
		var notFound *types.NotFound
		if ok := errors.As(err, &notFound); ok {
			return false, nil
		}

		return false, err
	}

	return true, nil
}
func (s S3Provider) GetUrl(ctx context.Context, path string) (string, error) {
	result, err := s.presign.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(path),
	})
	if err != nil {
		return "", err
	}
	return result.URL, nil
}

func (s S3Provider) List(ctx context.Context, prefix string, cursor *string, limit int) ([]string, *string, error) {
	result, err := s.client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
		Bucket:            aws.String(s.bucket),
		Prefix:            aws.String(prefix),
		ContinuationToken: cursor,
		MaxKeys:           aws.Int32(int32(limit)),
	})
	if err != nil {
		return nil, nil, err
	}
	files := make([]string, 0, len(result.Contents))
	for idx := range result.Contents {
		files = append(files, aws.ToString(result.Contents[idx].Key))
	}
	return files, result.NextContinuationToken, nil
}
