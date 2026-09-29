package config

import (
	"bufio"
	"os"
	"strconv"
	"strings"
	"time"
)

type AppConfig struct {
	Env  string
	Port string
}

// DeploymentConfig decides how organizations are provisioned. A self-hosted
// install runs exactly one organization; SaaS lets anyone sign up.
type DeploymentConfig struct {
	Mode               string
	Signup             string
	OrgName            string
	AdminEmail         string
	PersonalWorkspaces bool
	MultiOrg           bool
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
	Enabled                bool
	Provider               string
	DocumensoBaseURL       string
	DocumensoAPIKey        string
	DocumensoWebhookSecret string
}
type CacheConfig struct {
	URL string
}
type QueueConfig struct {
	RedisURL    string
	Concurrency int
}
type DoclingConfig struct {
	BaseURL string
	APIKey  string
	Timeout time.Duration
}
type OCRConfig struct {
	Provider  string
	Docling   DoclingConfig
	Gotenberg string
}
type JWTConfig struct {
	Secret string
}
type SMTPConfig struct{ Host, Port, Username, Password, From, AppURL string }
type SearchConfig struct {
	URL    string
	APIKey string
	Index  string
}
type AIConfig struct {
	Enabled        bool
	BaseURL        string
	APIKey         string
	ChatModel      string
	EmbeddingModel string
}
type Config struct {
	App             AppConfig
	Deployment      DeploymentConfig
	Database        DatabaseConfig
	StorageProvider string
	S3              S3Config
	LocalStorage    LocalStorageConfig
	SharePoint      SharePointConfig
	Signing         SigningConfig
	Cache           CacheConfig
	Queue           QueueConfig
	OCR             OCRConfig
	JWT             JWTConfig
	SMTP            SMTPConfig
	Search          SearchConfig
	AI              AIConfig
}

func Load(env string) Config {
	loadDotEnv(env)
	return Config{
		App: AppConfig{
			Env:  getEnvDefault("APP_ENV", "development"),
			Port: getEnvDefault("APP_PORT", "8080"),
		},
		Deployment: loadDeployment(),
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
			Enabled:                getEnvBool("SIGNING_ENABLED", false),
			Provider:               getEnvDefault("SIGNING_PROVIDER", "documenso"),
			DocumensoBaseURL:       getEnv("DOCUMENSO_BASE_URL"),
			DocumensoAPIKey:        getEnv("DOCUMENSO_API_KEY"),
			DocumensoWebhookSecret: getEnv("DOCUMENSO_WEBHOOK_SECRET"),
		},
		Cache: CacheConfig{
			URL: getEnv("VALKEY_URL"),
		},
		Queue: QueueConfig{
			RedisURL:    getEnv("REDIS_URL"),
			Concurrency: getEnvInt("QUEUE_CONCURRENCY", 10),
		},
		OCR: OCRConfig{
			Provider: getEnvDefault("OCR_PROVIDER", "docling"),
			Docling: DoclingConfig{
				BaseURL: getEnv("DOCLING_BASE_URL"),
				APIKey:  getEnv("DOCLING_API_KEY"),
				Timeout: getEnvDuration("DOCLING_TIMEOUT", 10*time.Minute),
			},
			Gotenberg: getEnv("GOTENBERG_BASE_URL"),
		},
		JWT: JWTConfig{
			Secret: getEnv("JWT_SECRET"),
		},
		SMTP: SMTPConfig{Host: getEnv("SMTP_HOST"), Port: getEnvDefault("SMTP_PORT", "587"), Username: getEnv("SMTP_USERNAME"), Password: getEnv("SMTP_PASSWORD"), From: getEnv("SMTP_FROM"), AppURL: getEnvDefault("APP_URL", "http://localhost:3000")},
		Search: SearchConfig{
			URL:    getEnv("MEILI_URL"),
			APIKey: getEnv("MEILI_MASTER_KEY"),
			Index:  getEnvDefault("MEILI_INDEX", "documents"),
		},
		AI: AIConfig{
			Enabled:        getEnvBool("AI_ENABLED", false),
			BaseURL:        getEnv("AI_BASE_URL"),
			APIKey:         getEnv("AI_API_KEY"),
			ChatModel:      getEnv("AI_CHAT_MODEL"),
			EmbeddingModel: getEnv("AI_EMBEDDING_MODEL"),
		},
	}
}

const (
	ModeSaaS       = "saas"
	ModeSelfHosted = "selfhosted"

	SignupOpen   = "open"
	SignupInvite = "invite"
)

func loadDeployment() DeploymentConfig {
	mode := strings.ToLower(getEnvDefault("EDMS_MODE", ModeSelfHosted))
	if mode != ModeSaaS {
		mode = ModeSelfHosted
	}
	signupDefault, personalDefault := SignupInvite, false
	if mode == ModeSaaS {
		signupDefault, personalDefault = SignupOpen, true
	}
	signup := strings.ToLower(getEnvDefault("EDMS_SIGNUP", signupDefault))
	if signup != SignupOpen {
		signup = SignupInvite
	}
	return DeploymentConfig{
		Mode:               mode,
		Signup:             signup,
		OrgName:            getEnv("EDMS_ORG_NAME"),
		AdminEmail:         getEnv("EDMS_ADMIN_EMAIL"),
		PersonalWorkspaces: getEnvBool("EDMS_PERSONAL_WORKSPACES", personalDefault),
		MultiOrg:           getEnvBool("EDMS_MULTI_ORG", mode == ModeSaaS),
	}
}

func getEnvDuration(key string, fallback time.Duration) time.Duration {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := time.ParseDuration(value)
	if err != nil {
		return fallback
	}
	return parsed
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
