package sharepoint

import (
	"context"
	"io"

	msgraphsdkgo "github.com/microsoftgraph/msgraph-sdk-go"
)

type SharePointProvider struct {
	tenantID string
	siteID   string
	client   *msgraphsdkgo.GraphServiceClient
}

func New(tenantID, siteID string, client *msgraphsdkgo.GraphServiceClient) *SharePointProvider {
	return &SharePointProvider{
		tenantID: tenantID,
		siteID:   siteID,
		client:   client,
	}
}
func (s SharePointProvider) Name() string {
	return "sharepoint"
}
func (s SharePointProvider) Upload(ctx context.Context, path string, file io.Reader) error {
	return nil
}
func (s SharePointProvider) Download(ctx context.Context, path string) (io.Reader, error) {
	return nil, nil
}
func (s SharePointProvider) Move(ctx context.Context, previousPath, newPath string) error {
	return nil
}
func (s SharePointProvider) Delete(ctx context.Context, path string) error {
	return nil
}
func (s SharePointProvider) HealthCheck(ctx context.Context) error {
	return nil
}
func (s SharePointProvider) Exists(ctx context.Context, path string) (bool, error) {
	return true, nil
}
func (s SharePointProvider) GetUrl(ctx context.Context, path string) (string, error) {
	return "", nil
}
