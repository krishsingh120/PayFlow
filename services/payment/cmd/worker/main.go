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

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/krishsingh120/payflow/services/payment/internal/config"
	"github.com/krishsingh120/payflow/services/payment/internal/ledger"
	"github.com/krishsingh120/payflow/services/payment/internal/messaging"
	"github.com/krishsingh120/payflow/services/payment/internal/provider"
	"github.com/krishsingh120/payflow/services/payment/internal/storage"
	"github.com/krishsingh120/payflow/services/payment/internal/worker"
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
		logger.Warn().Msg("running with in-memory payment repository")
		repo = storage.NewMemoryPaymentRepository()
	}

	// ── Simulated Provider ──────────────────────────────────────────────────
	simProvider := provider.NewSimulatedProvider(cfg.ProviderDefaultOutcome, cfg.ProviderTimeoutMs)

	// ── Ledger Service ──────────────────────────────────────────────────────
	var ledgerRepo ledger.Repository
	if pool != nil {
		ledgerRepo = ledger.NewPostgresRepository(pool)
	} else {
		ledgerRepo = ledger.NewMemoryRepository()
	}
	ledgerService := ledger.NewService(ledgerRepo, logger)

	// ── Processor ───────────────────────────────────────────────────────────
	processor := worker.NewPaymentProcessor(repo, simProvider, logger).
		WithLedger(ledgerService)

	// ── Messaging Consumer ──────────────────────────────────────────────────
	var consumer messaging.Consumer

	if cfg.RabbitMQURL != "" {
		rmq, err := messaging.NewRabbitMQBroker(cfg.RabbitMQURL, logger)
		if err == nil {
			logger.Info().Msg("connected to RabbitMQ broker")
			consumer = rmq
		} else {
			logger.Warn().Err(err).Msg("RabbitMQ connection failed — falling back to memory broker")
		}
	}

	if consumer == nil {
		logger.Warn().Msg("running with in-memory message consumer")
		consumer = messaging.NewMemoryBroker()
	}

	// ── Start Consumption ───────────────────────────────────────────────────
	ctx, cancel := context.WithCancel(context.Background())

	err = consumer.Consume(ctx, messaging.QueuePaymentProcessing, processor.ProcessJob)
	if err != nil {
		logger.Fatal().Err(err).Msg("failed to start consuming payment jobs")
	}

	logger.Info().
		Str("queue", messaging.QueuePaymentProcessing).
		Msg("payment-worker actively listening for jobs")

	// ── Graceful Shutdown ───────────────────────────────────────────────────
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info().Msg("shutdown signal received — draining worker")
	cancel()

	_ = consumer.Close()
	if pool != nil {
		pool.Close()
	}

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
