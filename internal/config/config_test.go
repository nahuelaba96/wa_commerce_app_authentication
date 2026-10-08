package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadReadsDotEnvAndPreservesEnvironmentOverrides(t *testing.T) {
	workingDirectory := t.TempDir()
	dotEnv := `AUTH_DATABASE_URL=postgres://file_user:file_password@localhost:5432/auth?sslmode=disable
BACKEND_INTERNAL_URL=http://localhost:8080
INTERNAL_SERVICE_TOKEN=file-token
AUTH_PRIVATE_KEY_PATH=/tmp/signing-key.pem
AUTH_ISSUER=http://localhost:8081
AUTH_AUDIENCE=commerce-api
AUTH_LISTEN_ADDRESS=:9090
AUTH_KEY_ID=key-from-file
`
	if err := os.WriteFile(filepath.Join(workingDirectory, ".env"), []byte(dotEnv), 0600); err != nil {
		t.Fatal(err)
	}

	originalDirectory, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(workingDirectory); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(originalDirectory); err != nil {
			t.Errorf("restore working directory: %v", err)
		}
	})

	t.Setenv("AUTH_DATABASE_URL", "postgres://environment_user:environment_password@localhost:5432/auth?sslmode=disable")
	t.Setenv("AUTH_LISTEN_ADDRESS", ":7070")
	for name, value := range map[string]string{
		"BACKEND_INTERNAL_URL":   "http://localhost:8080",
		"INTERNAL_SERVICE_TOKEN": "environment-token",
		"AUTH_PRIVATE_KEY_PATH":  "/tmp/signing-key.pem",
		"AUTH_ISSUER":            "http://localhost:8081",
		"AUTH_AUDIENCE":          "commerce-api",
	} {
		t.Setenv(name, value)
	}
	originalKeyID, hadKeyID := os.LookupEnv("AUTH_KEY_ID")
	if err := os.Unsetenv("AUTH_KEY_ID"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if hadKeyID {
			if err := os.Setenv("AUTH_KEY_ID", originalKeyID); err != nil {
				t.Errorf("restore AUTH_KEY_ID: %v", err)
			}
		} else if err := os.Unsetenv("AUTH_KEY_ID"); err != nil {
			t.Errorf("clear AUTH_KEY_ID: %v", err)
		}
	})

	cfg, err := Load()
	if err != nil {
		t.Fatalf("load configuration: %v", err)
	}
	if cfg.DatabaseURL != "postgres://environment_user:environment_password@localhost:5432/auth?sslmode=disable" {
		t.Errorf("database URL did not preserve environment override: %q", cfg.DatabaseURL)
	}
	if cfg.ListenAddress != ":7070" {
		t.Errorf("listen address = %q, want environment override :7070", cfg.ListenAddress)
	}
	if cfg.KeyID != "key-from-file" {
		t.Errorf("key ID = %q, want value from .env", cfg.KeyID)
	}
}
