package config

import (
	"os"
	"time"
)

type Config struct {
	Port        string
	DatabaseURL string
	JWTSecret   string
	JWTTTL      time.Duration
}

func Load() Config {
	ttl, err := time.ParseDuration(getenv("JWT_TTL", "24h"))
	if err != nil {
		ttl = 24 * time.Hour
	}
	return Config{
		Port:        getenv("PORT", "8080"),
		DatabaseURL: getenv("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/cinema?sslmode=disable"),
		JWTSecret:   getenv("JWT_SECRET", "change-me-in-production"),
		JWTTTL:      ttl,
	}
}

func getenv(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return fallback
}
