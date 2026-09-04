package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type Config struct {
	Address           string
	DataDir           string
	SessionCookieName string
	SecureCookies     bool
	AdminPassword     string
}

func Load() (Config, error) {
	dataDir := value("CYBERLIFE_DATA_DIR", "./runtime-data")
	absoluteDataDir, err := filepath.Abs(dataDir)
	if err != nil {
		return Config{}, fmt.Errorf("resolve data directory: %w", err)
	}
	adminPassword := os.Getenv("CYBERLIFE_ADMIN_PASSWORD")
	if strings.TrimSpace(adminPassword) == "" {
		return Config{}, fmt.Errorf("CYBERLIFE_ADMIN_PASSWORD is required")
	}
	return Config{
		Address:           value("CYBERLIFE_ADDR", "127.0.0.1:8080"),
		DataDir:           absoluteDataDir,
		SessionCookieName: value("CYBERLIFE_SESSION_COOKIE", "cyberlife_session"),
		SecureCookies:     strings.EqualFold(os.Getenv("CYBERLIFE_SECURE_COOKIES"), "true"),
		AdminPassword:     adminPassword,
	}, nil
}

func value(name, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(name)); value != "" {
		return value
	}
	return fallback
}
