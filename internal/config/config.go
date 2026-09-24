package config

import (
	"errors"
	"os"
	"time"
)

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
		JWTSecret:   os.Getenv("JWT_SECRET"),
		JWTTTL:      ttl,
	}
	// No default: a well-known fallback secret would let anyone forge tokens.
	if len(cfg.JWTSecret) < 32 {
		return cfg, errors.New("JWT_SECRET must be set and at least 32 characters")
	}
	return cfg, nil
}

func getenv(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return fallback
}
