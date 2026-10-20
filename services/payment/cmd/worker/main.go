// Command worker is the entry point for the PayFlow background job processor.
// It shares all internal packages with the payment API binary — only the
// startup and runtime loop differ. This is a deliberate design choice:
// one codebase, two deployment units, zero duplication.
package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/joho/godotenv"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/krishsingh120/payflow/services/payment/internal/config"
)

func main() {
	// Load .env file if present (dev convenience; not used in Docker).
	_ = godotenv.Load()

	// ── Config ──────────────────────────────────────────────────────────────
	cfg, err := config.Load()
	if err != nil {
		log.Fatal().Err(err).Msg("failed to load configuration")
	}

	// ── Logger ──────────────────────────────────────────────────────────────
	logger := buildLogger(cfg)
	logger.Info().
		Str("env", cfg.Env).
		Msg("starting payment-worker")

	// ── Graceful Shutdown ───────────────────────────────────────────────────
	ctx, cancel := context.WithCancel(context.Background())

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	// Worker processing loop will be connected in Phase 4 (RabbitMQ + provider).
	// For now, we just block until a shutdown signal is received.
	logger.Info().Msg("payment-worker running — awaiting jobs (RabbitMQ wired in Phase 4)")

	<-quit
	logger.Info().Msg("shutdown signal received — stopping worker")
	cancel()

	// Allow in-flight jobs up to 30 seconds to complete.
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	// Phase 4: wait for worker goroutines to drain using shutdownCtx.
	_ = shutdownCtx

	_ = ctx
	logger.Info().Msg("payment-worker stopped")
}

// buildLogger returns a zerolog logger configured for the current environment.
func buildLogger(cfg *config.Config) zerolog.Logger {
	level, err := zerolog.ParseLevel(cfg.LogLevel)
	if err != nil {
		level = zerolog.InfoLevel
	}

	if cfg.Env == "development" {
		return zerolog.New(zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: time.RFC3339}).
			Level(level).
			With().Timestamp().
			Str("service", "payment-worker").
			Logger()
	}

	return zerolog.New(os.Stderr).
		Level(level).
		With().Timestamp().
		Str("service", "payment-worker").
		Logger()
}
