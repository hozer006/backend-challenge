// Package config loads application settings from environment variables.
package config

import (
	"errors"
	"fmt"
	"os"
	"time"
)

// Config holds all runtime settings for the service.
type Config struct {
	HTTPPort      string
	GRPCPort      string
	MongoURI      string
	MongoDatabase string
	JWTSecret     string
	JWTTTL        time.Duration
	UserCountTick time.Duration
}

// Load reads configuration from environment variables, applying defaults
// where sensible. JWT_SECRET is required because a default secret would be
// a security hole.
func Load() (Config, error) {
	cfg := Config{
		HTTPPort:      getEnv("HTTP_PORT", "8080"),
		GRPCPort:      getEnv("GRPC_PORT", "9090"),
		MongoURI:      getEnv("MONGO_URI", "mongodb://localhost:27017"),
		MongoDatabase: getEnv("MONGO_DATABASE", "user_management"),
		JWTSecret:     os.Getenv("JWT_SECRET"),
		UserCountTick: 10 * time.Second,
	}

	if cfg.JWTSecret == "" {
		return Config{}, errors.New("JWT_SECRET is required")
	}

	ttl, err := time.ParseDuration(getEnv("JWT_TTL", "24h"))
	if err != nil {
		return Config{}, fmt.Errorf("invalid JWT_TTL: %w", err)
	}
	if ttl <= 0 {
		return Config{}, errors.New("JWT_TTL must be positive")
	}
	cfg.JWTTTL = ttl

	return cfg, nil
}

func getEnv(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return fallback
}
