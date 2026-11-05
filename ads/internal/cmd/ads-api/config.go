package main

import (
	"os"
	"strconv"
	"time"
)

// Config holds everything the API needs to start.
type Config struct {
	// HTTPAddr is the address the API listens on, for example ":8080".
	HTTPAddr string
	// DatabaseURL is the PostgreSQL connection string.
	DatabaseURL string
	// IdentityURL is the base URL of the identity service.
	IdentityURL string
	// IdentityTimeout bounds every call to the identity service.
	IdentityTimeout time.Duration
	// UserCacheTTL is how long a username is served from memory before being fetched again.
	UserCacheTTL time.Duration
	// UserCacheSize caps the number of usernames kept in memory.
	UserCacheSize int
}

// Load reads the configuration from the environment.
func Load() Config {
	return Config{
		HTTPAddr:        text("HTTP_ADDR", ":8080"),
		DatabaseURL:     text("DATABASE_URL", "postgres://ads:ads@localhost:5432/ads?sslmode=disable"),
		IdentityURL:     text("IDENTITY_URL", "http://localhost:8081"),
		IdentityTimeout: duration("IDENTITY_TIMEOUT", 300*time.Millisecond),
		UserCacheTTL:    duration("USER_CACHE_TTL", 5*time.Minute),
		UserCacheSize:   integer("USER_CACHE_SIZE", 10_000),
	}
}

func text(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}

	return fallback
}

func duration(name string, fallback time.Duration) time.Duration {
	value, err := time.ParseDuration(os.Getenv(name))
	if err != nil {
		return fallback
	}

	return value
}

func integer(name string, fallback int) int {
	value, err := strconv.Atoi(os.Getenv(name))
	if err != nil || value <= 0 {
		return fallback
	}

	return value
}
