// Command api is the entry point for the PayFlow HTTP server.
// It loads configuration, sets up structured logging, connects to the database,
// wires the router, and starts a gracefully-shutting-down HTTP server.
package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/krishsingh120/payflow/services/payment/internal/config"
	svchttp "github.com/krishsingh120/payflow/services/payment/internal/http"
	"github.com/krishsingh120/payflow/services/payment/internal/messaging"
	"github.com/krishsingh120/payflow/services/payment/internal/payments"
	"github.com/krishsingh120/payflow/services/payment/internal/storage"
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

	// ── Database & Storage ──────────────────────────────────────────────────
	var repo storage.PaymentRepository
	var pool *pgxpool.Pool

	if cfg.DatabaseURL != "" {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		var dbErr error
		pool, dbErr = pgxpool.New(ctx, cfg.DatabaseURL)
		cancel()

		if dbErr == nil {
			pingCtx, pingCancel := context.WithTimeout(context.Background(), 3*time.Second)
			if err := pool.Ping(pingCtx); err == nil {
				logger.Info().Msg("connected to PostgreSQL database")
				repo = storage.NewPostgresPaymentRepository(pool)
			} else {
				logger.Warn().Err(err).Msg("PostgreSQL ping failed — falling back to memory repository")
				pool.Close()
				pool = nil
			}
			pingCancel()
		} else {
			logger.Warn().Err(dbErr).Msg("PostgreSQL connection failed — falling back to memory repository")
		}
	}

	if repo == nil {
		logger.Warn().Msg("running with in-memory payment repository (no persistent database configured)")
		repo = storage.NewMemoryPaymentRepository()
	}

	// ── Messaging Publisher ─────────────────────────────────────────────────
	var publisher messaging.Publisher
	var rmqBroker *messaging.RabbitMQBroker

	if cfg.RabbitMQURL != "" {
		var rmqErr error
		rmqBroker, rmqErr = messaging.NewRabbitMQBroker(cfg.RabbitMQURL, logger)
		if rmqErr == nil {
			logger.Info().Msg("connected to RabbitMQ broker")
			publisher = rmqBroker
		} else {
			logger.Warn().Err(rmqErr).Msg("RabbitMQ connection failed — falling back to memory broker")
		}
	}

	if publisher == nil {
		logger.Warn().Msg("running with in-memory message publisher")
		publisher = messaging.NewMemoryBroker()
	}

	// ── Payment Service ─────────────────────────────────────────────────────
	paymentService := payments.NewService(repo).WithPublisher(publisher)

	// ── Router ──────────────────────────────────────────────────────────────
	router := svchttp.NewRouter(logger, paymentService)

	// ── HTTP Server ─────────────────────────────────────────────────────────
	srv := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.Port),
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// ── Graceful Shutdown ───────────────────────────────────────────────────
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

	if rmqBroker != nil {
		_ = rmqBroker.Close()
		logger.Info().Msg("closed RabbitMQ connection")
	}

	if pool != nil {
		pool.Close()
		logger.Info().Msg("closed database connection pool")
	}

	logger.Info().Msg("payment-api stopped")
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
			Str("service", "payment-api").
			Logger()
	}

	return zerolog.New(os.Stderr).
		Level(level).
		With().Timestamp().
		Str("service", "payment-api").
		Logger()
}
