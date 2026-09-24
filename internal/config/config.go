package config

import (
	"errors"
	"log/slog"
	"os"
	"time"
)

// DefaultJWTSecret lets testers run the API with zero configuration.
// Always override JWT_SECRET outside local testing.
const DefaultJWTSecret = "mkp-backend-test-2025-default-jwt-secret"

type Config struct {
	Port        string
	DatabaseURL string
	JWTSecret   string
	JWTTTL      time.Duration
}

func Load() (Config, error) {
	ttl, err := time.ParseDuration(getenv("JWT_TTL", "24h"))
	if err != nil {
		ttl = 24 * time.Hour
	}
	cfg := Config{
		Port:        getenv("PORT", "8080"),
		DatabaseURL: getenv("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/cinema?sslmode=disable"),
		JWTSecret:   getenv("JWT_SECRET", DefaultJWTSecret),
		JWTTTL:      ttl,
	}
	if cfg.JWTSecret == DefaultJWTSecret {
		slog.Warn("JWT_SECRET not set, using the built-in default secret (OK for local testing only)")
	} else if len(cfg.JWTSecret) < 32 {
		return cfg, errors.New("JWT_SECRET must be at least 32 characters")
	}
	return cfg, nil
}

func getenv(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return fallback
}
