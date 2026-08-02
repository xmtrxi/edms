package config

import (
	"bufio"
	"os"
	"strconv"
	"strings"
)

type AppConfig struct {
	Env  string
	Port string
}

type DatabaseConfig struct {
	URL       string
	Namespace string
	Database  string
	User      string
	Password  string
}

type S3Config struct {
	Region          string
	AccessKeyID     string
	SecretAccessKey string
	Endpoint        string
	Bucket          string
}
type LocalStorageConfig struct {
	Path string
}
type SharePointConfig struct {
	SiteURL      string
	SiteID       string
	TenantID     string
	ClientID     string
	ClientSecret string
	DriveID      string
	Folder       string
}
type SigningConfig struct {
	Enabled          bool
	Provider         string
	DocumensoBaseURL string
	DocumensoAPIKey  string
}
type CacheConfig struct {
	URL string
}
type QueueConfig struct {
	RedisURL    string
	Concurrency int
}
type PaperlessConfig struct {
	BaseURL string
	Token   string
}
type JWTConfig struct {
	Secret string
}
type SearchConfig struct {
	URL    string
	APIKey string
	Index  string
}
type Config struct {
	App             AppConfig
	Database        DatabaseConfig
	StorageProvider string
	S3              S3Config
	LocalStorage    LocalStorageConfig
	SharePoint      SharePointConfig
	Signing         SigningConfig
	Cache           CacheConfig
	Queue           QueueConfig
	Paperless       PaperlessConfig
	JWT             JWTConfig
	Search          SearchConfig
}

func Load(env string) Config {
	loadDotEnv(env)
	return Config{
		App: AppConfig{
			Env:  getEnvDefault("APP_ENV", "development"),
			Port: getEnvDefault("APP_PORT", "8080"),
		},
		Database: DatabaseConfig{
			URL:       getEnv("SURREALDB_URL"),
			Namespace: getEnvDefault("SURREALDB_NAMESPACE", "edms"),
			Database:  getEnvDefault("SURREALDB_DATABASE", "edms"),
			User:      getEnv("SURREALDB_USER"),
			Password:  getEnv("SURREALDB_PASS"),
		},
		StorageProvider: getEnv("STORAGE_PROVIDER"),
		S3: S3Config{
			Region:          getEnv("S3_REGION"),
			AccessKeyID:     getEnv("S3_ACCESS_KEY_ID"),
			SecretAccessKey: getEnv("S3_SECRET_ACCESS_KEY"),
			Endpoint:        getEnv("S3_ENDPOINT"),
			Bucket:          getEnv("S3_BUCKET"),
		},
		SharePoint: SharePointConfig{
			SiteURL:      getEnv("SHAREPOINT_SITE_URL"),
			SiteID:       getEnv("SHAREPOINT_SITE_ID"),
			TenantID:     getEnv("SHAREPOINT_TENANT_ID"),
			ClientID:     getEnv("SHAREPOINT_CLIENT_ID"),
			ClientSecret: getEnv("SHAREPOINT_CLIENT_SECRET"),
			DriveID:      getEnv("SHAREPOINT_DRIVE_ID"),
			Folder:       getEnv("SHAREPOINT_FOLDER"),
		},
		LocalStorage: LocalStorageConfig{
			Path: getEnv("LOCAL_STORAGE_PATH"),
		},
		Signing: SigningConfig{
			Enabled:          getEnvBool("SIGNING_ENABLED", false),
			Provider:         getEnv("SIGNING_PROVIDER"),
			DocumensoBaseURL: getEnv("DOCUMENSO_BASE_URL"),
			DocumensoAPIKey:  getEnv("DOCUMENSO_API_KEY"),
		},
		Cache: CacheConfig{
			URL: getEnv("VALKEY_URL"),
		},
		Queue: QueueConfig{
			RedisURL:    getEnv("REDIS_URL"),
			Concurrency: getEnvInt("QUEUE_CONCURRENCY", 10),
		},
		Paperless: PaperlessConfig{
			BaseURL: getEnv("PAPERLESS_BASE_URL"),
			Token:   getEnv("PAPERLESS_TOKEN"),
		},
		JWT: JWTConfig{
			Secret: getEnv("JWT_SECRET"),
		},
		Search: SearchConfig{
			URL:    getEnv("MEILI_URL"),
			APIKey: getEnv("MEILI_MASTER_KEY"),
			Index:  getEnvDefault("MEILI_INDEX", "documents"),
		},
	}
}

func loadDotEnv(filename string) {
	file, err := os.Open(filename)
	if err != nil {
		return // silently ignore if missing
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// ignore comments & empty lines
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		os.Setenv(key, value)
	}
}
func getEnv(key string) string {
	return os.Getenv(key)
}

func getEnvDefault(key string, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}

	return value
}

func getEnvInt(key string, fallback int) int {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}

	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}

	return parsed
}

func getEnvBool(key string, fallback bool) bool {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}

	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return fallback
	}

	return parsed
}
