// Command api is the entry point for the PayFlow HTTP server.
// It loads configuration, sets up structured logging, wires the router,
// and starts a gracefully-shutting-down HTTP server.
package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/joho/godotenv"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/krishsingh120/payflow/services/payment/internal/config"
	svchttp "github.com/krishsingh120/payflow/services/payment/internal/http"
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
		Int("port", cfg.Port).
		Msg("starting payment-api")

	// ── Router ──────────────────────────────────────────────────────────────
	router := svchttp.NewRouter(logger)

	// ── HTTP Server ─────────────────────────────────────────────────────────
	srv := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.Port),
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// ── Graceful Shutdown ───────────────────────────────────────────────────
	// Run server in a goroutine so the main goroutine can wait for OS signals.
	go func() {
		logger.Info().Str("addr", srv.Addr).Msg("payment-api listening")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal().Err(err).Msg("server error")
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info().Msg("shutdown signal received — draining connections")

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		logger.Error().Err(err).Msg("server shutdown error")
	}

	logger.Info().Msg("payment-api stopped")
}

// buildLogger returns a zerolog logger configured for the current environment.
// In development it outputs pretty-printed console logs; in production it
// outputs JSON for log aggregation.
func buildLogger(cfg *config.Config) zerolog.Logger {
	level, err := zerolog.ParseLevel(cfg.LogLevel)
	if err != nil {
		level = zerolog.InfoLevel
	}

	if cfg.Env == "development" {
		return zerolog.New(zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: time.RFC3339}).
			Level(level).
			With().Timestamp().
			Str("service", "payment-api").
			Logger()
	}

	return zerolog.New(os.Stderr).
		Level(level).
		With().Timestamp().
		Str("service", "payment-api").
		Logger()
}
