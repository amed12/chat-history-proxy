package config

import (
    "log"
    "os"
    "strconv"

    "github.com/joho/godotenv"
)

// Config holds all runtime configuration loaded from environment variables.
type Config struct {
    Port                  string
    DatabaseURL           string
    QiscusAppID           string
    QiscusSecretKey       string
    QiscusBaseURL         string
    QiscusMultichannelURL string
    WebhookSecret         string
    JWTSecret             string
    JWTExpiryHours        int
}

// Load reads .env (if present) then environment variables.
// Panics if any required variable is missing.
func Load() Config {
    // Ignore error — .env file is optional in production (use real env vars).
    _ = godotenv.Load()

    cfg := Config{
        Port:                  getEnv("PORT", "8080"),
        DatabaseURL:           requireEnv("DATABASE_URL"),
        QiscusAppID:           requireEnv("QISCUS_APP_ID"),
        QiscusSecretKey:       requireEnv("QISCUS_SECRET_KEY"),
        QiscusBaseURL:         getEnv("QISCUS_BASE_URL", "https://api3.qiscus.com"),
        QiscusMultichannelURL: getEnv("QISCUS_MULTICHANNEL_URL", "https://multichannel.qiscus.com"),
        WebhookSecret:         requireEnv("WEBHOOK_SECRET"),
        JWTSecret:             requireEnv("JWT_SECRET"),
        JWTExpiryHours:        getEnvInt("JWT_EXPIRY_HOURS", 720),
    }

    return cfg
}

func requireEnv(key string) string {
    v := os.Getenv(key)
    if v == "" {
        log.Fatalf("required environment variable %q is not set", key)
    }
    return v
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
