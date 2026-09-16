// Package config loads runtime configuration for this proxy
// entirely from environment variables. Nothing here is hardcoded per
// deployment — swapping client/environment means changing .env, never this
// code.
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

// Config holds all runtime configuration for this proxy service.
type Config struct {
	Port            string
	QiscusAppID     string
	QiscusSecretKey string
	QiscusBaseURL   string
	JWTPublicKeyPEM string
	CacheTTLSeconds int
}

// requiredEnvVars lists the variables that have no safe default: a missing
// one means the service cannot run correctly, so Load fails loudly instead
// of starting with an empty credential.
var requiredEnvVars = []string{"QISCUS_APP_ID", "QISCUS_SECRET_KEY", "JWT_PUBLIC_KEY"}

// Load reads configuration from environment variables. It returns an error
// naming every missing required variable instead of starting with an empty
// credential.
func Load() (Config, error) {
	var missing []string
	for _, name := range requiredEnvVars {
		if os.Getenv(name) == "" {
			missing = append(missing, name)
		}
	}
	if len(missing) > 0 {
		return Config{}, fmt.Errorf(
			"missing required environment variable(s): %s",
			strings.Join(missing, ", "),
		)
	}

	return Config{
		Port:            getEnv("APP_PORT", "8081"),
		QiscusAppID:     os.Getenv("QISCUS_APP_ID"),
		QiscusSecretKey: os.Getenv("QISCUS_SECRET_KEY"),
		QiscusBaseURL:   getEnv("QISCUS_BASE_URL", "https://api3.qiscus.com"),
		JWTPublicKeyPEM: os.Getenv("JWT_PUBLIC_KEY"),
		CacheTTLSeconds: getEnvInt("CACHE_TTL_SECONDS", 60),
	}, nil
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			return i
		}
	}
	return fallback
}
