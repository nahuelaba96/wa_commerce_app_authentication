package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	ListenAddress           string
	DatabaseURL             string
	BackendURL              string
	InternalServiceToken    string
	PrivateKeyPath          string
	KeyID                   string
	Issuer                  string
	Audience                string
	AllowedOrigins          []string
	CookieName              string
	CookieDomain            string
	CookieSecure            bool
	AccessTokenTTL          time.Duration
	RefreshTokenTTL         time.Duration
	BackendTimeout          time.Duration
	OnboardingRetryInterval time.Duration
	LoginLockBase           time.Duration
	LoginLockMaximum        time.Duration
}

func Load() (Config, error) {
	if err := godotenv.Load(); err != nil && !errors.Is(err, os.ErrNotExist) {
		return Config{}, fmt.Errorf("load .env file: %w", err)
	}

	cfg := Config{
		ListenAddress:           env("AUTH_LISTEN_ADDRESS", ":8081"),
		DatabaseURL:             os.Getenv("AUTH_DATABASE_URL"),
		BackendURL:              os.Getenv("BACKEND_INTERNAL_URL"),
		InternalServiceToken:    os.Getenv("INTERNAL_SERVICE_TOKEN"),
		PrivateKeyPath:          os.Getenv("AUTH_PRIVATE_KEY_PATH"),
		KeyID:                   env("AUTH_KEY_ID", "auth-rsa-1"),
		Issuer:                  os.Getenv("AUTH_ISSUER"),
		Audience:                os.Getenv("AUTH_AUDIENCE"),
		AllowedOrigins:          splitCSV(env("AUTH_ALLOWED_ORIGINS", "http://localhost:4200")),
		CookieName:              env("AUTH_REFRESH_COOKIE_NAME", "auth_refresh"),
		CookieDomain:            os.Getenv("AUTH_COOKIE_DOMAIN"),
		AccessTokenTTL:          15 * time.Minute,
		RefreshTokenTTL:         7 * 24 * time.Hour,
		BackendTimeout:          5 * time.Second,
		OnboardingRetryInterval: 30 * time.Second,
		LoginLockBase:           1 * time.Second,
		LoginLockMaximum:        15 * time.Minute,
	}
	if value := os.Getenv("AUTH_COOKIE_SECURE"); value != "" {
		secure, err := strconv.ParseBool(value)
		if err != nil {
			return Config{}, fmt.Errorf("AUTH_COOKIE_SECURE: %w", err)
		}
		cfg.CookieSecure = secure
	} else {
		cfg.CookieSecure = true
	}

	var missing []string
	for name, value := range map[string]string{
		"AUTH_DATABASE_URL":      cfg.DatabaseURL,
		"BACKEND_INTERNAL_URL":   cfg.BackendURL,
		"INTERNAL_SERVICE_TOKEN": cfg.InternalServiceToken,
		"AUTH_PRIVATE_KEY_PATH":  cfg.PrivateKeyPath,
		"AUTH_ISSUER":            cfg.Issuer,
		"AUTH_AUDIENCE":          cfg.Audience,
	} {
		if strings.TrimSpace(value) == "" {
			missing = append(missing, name)
		}
	}
	if len(missing) > 0 {
		return Config{}, fmt.Errorf("required configuration is missing: %s", strings.Join(missing, ", "))
	}
	if len(cfg.AllowedOrigins) == 0 {
		return Config{}, errors.New("AUTH_ALLOWED_ORIGINS must contain at least one origin")
	}
	for _, origin := range cfg.AllowedOrigins {
		if origin == "*" {
			return Config{}, errors.New("AUTH_ALLOWED_ORIGINS cannot contain wildcard origins")
		}
	}
	return cfg, nil
}

func env(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}

func splitCSV(value string) []string {
	var values []string
	for _, item := range strings.Split(value, ",") {
		if item = strings.TrimSpace(item); item != "" {
			values = append(values, item)
		}
	}
	return values
}
