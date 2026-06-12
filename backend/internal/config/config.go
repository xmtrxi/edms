package config

import (
	"bufio"
	"os"
	"strings"
)

type S3Config struct {
	Region          string
	AccessKeyID     string
	SecretAccessKey string
	Endpoint        string
}
type LocalStorageConfig struct {
	Path string
}
type SharePointConfig struct {
	SiteURL      string
	TenantID     string
	ClientID     string
	ClientSecret string
	DriveID      string
	Folder       string
}
type Config struct {
	S3           S3Config
	LocalStorage LocalStorageConfig
	SharePoint   SharePointConfig
}

func Load(env string) Config {
	loadDotEnv(env)
	return Config{
		S3: S3Config{
			Region:          getEnv("S3_REGION"),
			AccessKeyID:     getEnv("S3_ACCESS_KEY_ID"),
			SecretAccessKey: getEnv("S3_SECRET_ACCESS_KEY"),
			Endpoint:        getEnv("S3_ENDPOINT"),
		},
		SharePoint: SharePointConfig{
			SiteURL:      getEnv("SHAREPOINT_SITE_URL"),
			TenantID:     getEnv("SHAREPOINT_TENANT_ID"),
			ClientID:     getEnv("SHAREPOINT_CLIENT_ID"),
			ClientSecret: getEnv("SHAREPOINT_CLIENT_SECRET"),
			DriveID:      getEnv("SHAREPOINT_DRIVE_ID"),
			Folder:       getEnv("SHAREPOINT_FOLDER"),
		},
		LocalStorage: LocalStorageConfig{
			Path: getEnv("LOCAL_STORAGE_PATH"),
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
