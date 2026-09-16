package config

import (
	"strings"
	"testing"
)

func clearRequiredEnv(t *testing.T) {
	t.Helper()
	for _, name := range requiredEnvVars {
		t.Setenv(name, "")
	}
}

func TestLoad_MissingRequiredVars(t *testing.T) {
	clearRequiredEnv(t)

	_, err := Load()
	if err == nil {
		t.Fatal("expected an error when required env vars are missing, got nil")
	}
	for _, name := range requiredEnvVars {
		if !strings.Contains(err.Error(), name) {
			t.Errorf("expected error to mention %q, got: %v", name, err)
		}
	}
}

func TestLoad_PartiallyMissing(t *testing.T) {
	clearRequiredEnv(t)
	t.Setenv("QISCUS_APP_ID", "app-1")
	t.Setenv("QISCUS_SECRET_KEY", "secret-1")

	_, err := Load()
	if err == nil {
		t.Fatal("expected an error when JWT_PUBLIC_KEY is missing, got nil")
	}
	if !strings.Contains(err.Error(), "JWT_PUBLIC_KEY") {
		t.Errorf("expected error to mention JWT_PUBLIC_KEY, got: %v", err)
	}
	if strings.Contains(err.Error(), "QISCUS_APP_ID") {
		t.Errorf("did not expect error to mention QISCUS_APP_ID (it was set), got: %v", err)
	}
}

func TestLoad_AllSet(t *testing.T) {
	t.Setenv("QISCUS_APP_ID", "app-1")
	t.Setenv("QISCUS_SECRET_KEY", "secret-1")
	t.Setenv("JWT_PUBLIC_KEY", "public-key-pem")
	t.Setenv("APP_PORT", "")
	t.Setenv("QISCUS_BASE_URL", "")
	t.Setenv("CACHE_TTL_SECONDS", "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if cfg.QiscusAppID != "app-1" {
		t.Errorf("QiscusAppID = %q, want %q", cfg.QiscusAppID, "app-1")
	}
	if cfg.Port != "8081" {
		t.Errorf("Port default = %q, want %q", cfg.Port, "8081")
	}
	if cfg.QiscusBaseURL != "https://api3.qiscus.com" {
		t.Errorf("QiscusBaseURL default = %q, want default", cfg.QiscusBaseURL)
	}
	if cfg.CacheTTLSeconds != 60 {
		t.Errorf("CacheTTLSeconds default = %d, want 60", cfg.CacheTTLSeconds)
	}
}

func TestLoad_CustomOptionalValues(t *testing.T) {
	t.Setenv("QISCUS_APP_ID", "app-1")
	t.Setenv("QISCUS_SECRET_KEY", "secret-1")
	t.Setenv("JWT_PUBLIC_KEY", "public-key-pem")
	t.Setenv("APP_PORT", "9090")
	t.Setenv("QISCUS_BASE_URL", "https://staging.qiscus.example.com")
	t.Setenv("CACHE_TTL_SECONDS", "30")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if cfg.Port != "9090" {
		t.Errorf("Port = %q, want %q", cfg.Port, "9090")
	}
	if cfg.QiscusBaseURL != "https://staging.qiscus.example.com" {
		t.Errorf("QiscusBaseURL = %q, want custom value", cfg.QiscusBaseURL)
	}
	if cfg.CacheTTLSeconds != 30 {
		t.Errorf("CacheTTLSeconds = %d, want 30", cfg.CacheTTLSeconds)
	}
}
