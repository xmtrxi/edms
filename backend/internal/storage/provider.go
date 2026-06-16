package storage

import (
	"context"
	"fmt"
	"io"

	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	auth "github.com/microsoft/kiota-authentication-azure-go"
	msgraphsdkgo "github.com/microsoftgraph/msgraph-sdk-go"
	"github.com/xmtrxi/edms/internal/config"
	"github.com/xmtrxi/edms/internal/storage/local"
	s3prov "github.com/xmtrxi/edms/internal/storage/s3"
	"github.com/xmtrxi/edms/internal/storage/sharepoint"
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

func NewProvider(ctx context.Context, cfg config.Config) (Provider, error) {
	switch cfg.StorageProvider {
	case "local":
		return local.New(cfg.LocalStorage.Path), nil

	case "s3":
		awsConfig := aws.Config{
			Region: cfg.S3.Region,
			EndpointResolver: aws.EndpointResolverFunc(func(service, region string) (aws.Endpoint, error) {
				return aws.Endpoint{URL: cfg.S3.Endpoint}, nil
			}),
			Credentials: aws.NewCredentialsCache(credentials.NewStaticCredentialsProvider(cfg.S3.AccessKeyID, cfg.S3.SecretAccessKey, "")),
		}

		client := s3.NewFromConfig(awsConfig, func(o *s3.Options) {
			if cfg.S3.Endpoint != "" {
				o.BaseEndpoint = aws.String(cfg.S3.Endpoint)
				o.UsePathStyle = true
			}
		})

		return s3prov.New(client, cfg.S3.Bucket), nil

	case "sharepoint":
		cred, err := azidentity.NewClientSecretCredential(cfg.SharePoint.TenantID, cfg.SharePoint.ClientID, cfg.SharePoint.ClientSecret, nil)
		if err != nil {
			return nil, fmt.Errorf("sharepoint: failed to create credential: %w", err)
		}

		authProvider, err := auth.NewAzureIdentityAuthenticationProviderWithScopes(cred, []string{"https://graph.microsoft.com/.default"})
		if err != nil {
			return nil, fmt.Errorf("sharepoint: failed to create auth provider: %w", err)
		}

		adapter, err := msgraphsdkgo.NewGraphRequestAdapter(authProvider)
		if err != nil {
			return nil, fmt.Errorf("sharepoint: failed to create request adapter: %w", err)
		}

		client := msgraphsdkgo.NewGraphServiceClient(adapter)

		return sharepoint.New(cfg.SharePoint.TenantID, cfg.SharePoint.SiteID, client), nil

	default:
		return nil, fmt.Errorf("unsupported storage provider: %q (supported: local, s3, sharepoint)", cfg.StorageProvider)
	}
}
