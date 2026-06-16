package sharepoint

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/url"

	msgraphsdkgo "github.com/microsoftgraph/msgraph-sdk-go"
	"github.com/microsoftgraph/msgraph-sdk-go/drives"
	"github.com/microsoftgraph/msgraph-sdk-go/models"
)

type SharePointProvider struct {
	tenantID string
	siteID   string
	driveID  string // Memoized or injected default document library ID
	client   *msgraphsdkgo.GraphServiceClient
}

func New(tenantID, siteID string, client *msgraphsdkgo.GraphServiceClient) *SharePointProvider {
	return &SharePointProvider{
		tenantID: tenantID,
		siteID:   siteID,
		client:   client,
	}
}

func (s *SharePointProvider) Name() string {
	return "sharepoint"
}

// getDriveID ensures we have the site's default Drive ID for the canonical endpoints.
func (s *SharePointProvider) getDriveID(ctx context.Context) (string, error) {
	if s.driveID != "" {
		return s.driveID, nil
	}
	// Fetch the default document library ('drive') of the SharePoint Site
	drive, err := s.client.Sites().BySiteId(s.siteID).Drive().Get(ctx, nil)
	if err != nil {
		return "", fmt.Errorf("failed to retrieve site default drive: %w", err)
	}
	if drive.GetId() == nil {
		return "", fmt.Errorf("drive ID returned nil")
	}
	s.driveID = *drive.GetId()
	return s.driveID, nil
}

// Upload uploads a file to a specific path using canonical drive item syntax.
func (s *SharePointProvider) Upload(ctx context.Context, path string, file io.Reader) error {
	driveID, err := s.getDriveID(ctx)
	if err != nil {
		return err
	}

	data, err := io.ReadAll(file)
	if err != nil {
		return fmt.Errorf("failed to read file data: %w", err)
	}

	// Address via item path format syntax: root:/path/to/file.ext:
	itemPath := fmt.Sprintf("root:/%s:", path)

	_, err = s.client.Drives().ByDriveId(driveID).Items().ByDriveItemId(itemPath).Content().Put(ctx, data, nil)
	if err != nil {
		return fmt.Errorf("failed to upload item: %w", err)
	}
	return nil
}

// Download retrieves a file from SharePoint as an io.ReadCloser.
func (s *SharePointProvider) Download(ctx context.Context, path string) (io.ReadCloser, error) {
	driveID, err := s.getDriveID(ctx)
	if err != nil {
		return nil, err
	}

	itemPath := fmt.Sprintf("root:/%s:", path)

	content, err := s.client.Drives().ByDriveId(driveID).Items().ByDriveItemId(itemPath).Content().Get(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to download item: %w", err)
	}
	res := io.NopCloser(bytes.NewReader(content))
	return res, nil
}

// Move renames or moves a file by updating its metadata parameters.
func (s *SharePointProvider) Move(ctx context.Context, previousPath, newPath string) error {
	driveID, err := s.getDriveID(ctx)
	if err != nil {
		return err
	}

	requestBody := models.NewDriveItem()
	requestBody.SetName(&newPath)

	itemPath := fmt.Sprintf("root:/%s:", previousPath)

	_, err = s.client.Drives().ByDriveId(driveID).Items().ByDriveItemId(itemPath).Patch(ctx, requestBody, nil)
	if err != nil {
		return fmt.Errorf("failed to move item: %w", err)
	}
	return nil
}

// Delete removes a file or folder from the SharePoint site.
func (s *SharePointProvider) Delete(ctx context.Context, path string) error {
	driveID, err := s.getDriveID(ctx)
	if err != nil {
		return err
	}

	itemPath := fmt.Sprintf("root:/%s:", path)

	err = s.client.Drives().ByDriveId(driveID).Items().ByDriveItemId(itemPath).Delete(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to delete item: %w", err)
	}
	return nil
}

// HealthCheck verifies connectivity by checking the explicit Graph status of the target Site.
func (s *SharePointProvider) HealthCheck(ctx context.Context) error {
	_, err := s.client.Sites().BySiteId(s.siteID).Get(ctx, nil)
	if err != nil {
		return fmt.Errorf("sharepoint site health check failed: %w", err)
	}
	return nil
}

// Exists checks if an element can be found at the given target path.
func (s *SharePointProvider) Exists(ctx context.Context, path string) (bool, error) {
	driveID, err := s.getDriveID(ctx)
	if err != nil {
		return false, err
	}

	itemPath := fmt.Sprintf("root:/%s:", path)

	_, err = s.client.Drives().ByDriveId(driveID).Items().ByDriveItemId(itemPath).Get(ctx, nil)
	if err != nil {
		// Non-existent resources throw an error status (such as 404)
		return false, nil
	}
	return true, nil
}

// GetUrl retrieves the absolute web reference link for the item.
func (s *SharePointProvider) GetUrl(ctx context.Context, path string) (string, error) {
	driveID, err := s.getDriveID(ctx)
	if err != nil {
		return "", err
	}

	itemPath := fmt.Sprintf("root:/%s:", path)

	item, err := s.client.Drives().ByDriveId(driveID).Items().ByDriveItemId(itemPath).Get(ctx, nil)
	if err != nil {
		return "", fmt.Errorf("failed to fetch item details: %w", err)
	}

	if item.GetWebUrl() == nil {
		return "", fmt.Errorf("web url property missing from graph response")
	}
	return *item.GetWebUrl(), nil
}

// List pulls folder items using a path-encoded skip token cursor strategy.
func (s *SharePointProvider) List(ctx context.Context, prefix string, cursor *string, limit int) ([]string, *string, error) {
	driveID, err := s.getDriveID(ctx)
	if err != nil {
		return nil, nil, err
	}

	var itemPath string
	if prefix == "" || prefix == "/" {
		itemPath = "root"
	} else {
		itemPath = fmt.Sprintf("root:/%s:", prefix)
	}

	// Setup pagination configuration fields matching Graph's expected generated structural parameters
	requestConfig := &drives.ItemItemsItemChildrenRequestBuilderGetRequestConfiguration{
		QueryParameters: &drives.ItemItemsItemChildrenRequestBuilderGetQueryParameters{},
	}

	topVal := int32(limit)
	requestConfig.QueryParameters.Top = &topVal

	if cursor != nil && *cursor != "" {
		requestConfig.Headers.Add("skiptoken", *cursor)
	}

	childrenCollection, err := s.client.Drives().ByDriveId(driveID).Items().ByDriveItemId(itemPath).Children().Get(ctx, requestConfig)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to list children matching target structure: %w", err)
	}

	var result []string
	for _, item := range childrenCollection.GetValue() {
		if item.GetName() != nil {
			result = append(result, *item.GetName())
		}
	}

	var nextCursor *string
	if childrenCollection.GetOdataNextLink() != nil {
		u, err := url.Parse(*childrenCollection.GetOdataNextLink())
		if err == nil {
			token := u.Query().Get("$skiptoken")
			if token != "" {
				nextCursor = &token
			}
		}
	}

	return result, nextCursor, nil
}
