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
	UsePathStyle    bool
}
type SigningConfig struct {
	Enabled          bool
	Provider         string
	DocumensoBaseURL string
	DocumensoAPIKey  string
}
type Config struct {
	App             AppConfig
	Database        DatabaseConfig
	StorageProvider string
	S3              S3Config
	Signing         SigningConfig
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
		StorageProvider: getEnvDefault("STORAGE_PROVIDER", "s3"),
		S3: S3Config{
			Region:          getEnvDefault("S3_REGION", "us-east-1"),
			AccessKeyID:     getEnv("S3_ACCESS_KEY_ID"),
			SecretAccessKey: getEnv("S3_SECRET_ACCESS_KEY"),
			Endpoint:        getEnv("S3_ENDPOINT"),
			Bucket:          getEnvDefault("S3_BUCKET", "edms-documents"),
			UsePathStyle:    getEnvBool("S3_USE_PATH_STYLE", true),
		},
		Signing: SigningConfig{
			Enabled:          getEnvBool("SIGNING_ENABLED", false),
			Provider:         getEnv("SIGNING_PROVIDER"),
			DocumensoBaseURL: getEnv("DOCUMENSO_BASE_URL"),
			DocumensoAPIKey:  getEnv("DOCUMENSO_API_KEY"),
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
