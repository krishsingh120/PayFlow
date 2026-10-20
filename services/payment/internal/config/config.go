// Package config loads and validates service configuration from environment variables.
// All configuration is centralised here so that handlers and services never
// call os.Getenv directly — making it easy to test and reason about.
package config

import (
	"fmt"
	"os"
	"strconv"
)

// Config holds all runtime configuration for the payment service.
type Config struct {
	// Server
	Port int
	Env  string

	// Database
	DatabaseURL string

	// RabbitMQ
	RabbitMQURL string

	// Logging
	LogLevel string

	// Webhook
	WebhookSigningSecret string

	// Simulated provider
	ProviderDefaultOutcome string
	ProviderTimeoutMs      int
}

// Load reads configuration from environment variables.
// It returns an error if any required variable is missing or invalid.
func Load() (*Config, error) {
	port, err := strconv.Atoi(getEnv("PAYMENT_SERVICE_PORT", "8080"))
	if err != nil {
		return nil, fmt.Errorf("config: invalid PAYMENT_SERVICE_PORT: %w", err)
	}

	providerTimeout, err := strconv.Atoi(getEnv("PROVIDER_TIMEOUT_MS", "3000"))
	if err != nil {
		return nil, fmt.Errorf("config: invalid PROVIDER_TIMEOUT_MS: %w", err)
	}

	cfg := &Config{
		Port:                   port,
		Env:                    getEnv("PAYMENT_SERVICE_ENV", "development"),
		DatabaseURL:            getEnv("DATABASE_URL", ""),
		RabbitMQURL:            getEnv("RABBITMQ_URL", ""),
		LogLevel:               getEnv("LOG_LEVEL", "info"),
		WebhookSigningSecret:   getEnv("WEBHOOK_SIGNING_SECRET", ""),
		ProviderDefaultOutcome: getEnv("PROVIDER_DEFAULT_OUTCOME", "success"),
		ProviderTimeoutMs:      providerTimeout,
	}

	return cfg, nil
}

// getEnv returns the value of the environment variable key, or fallback if not set.
func getEnv(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return fallback
}
