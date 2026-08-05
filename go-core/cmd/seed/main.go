package main

import (
	"context"
	"log/slog"
	"os"
	"time"

	"tinder-core/internal/config"
	"tinder-core/internal/platform/logging"
	"tinder-core/internal/platform/postgres"
	"tinder-core/internal/seed"
)

func main() {
	cfg := config.Load()
	logger := logging.New(cfg.Env, cfg.LogLevel)
	slog.SetDefault(logger)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	db, err := postgres.New(cfg.PostgresDSN)
	if err != nil {
		logger.Error("connect to Postgres", "error", err)
		os.Exit(1)
	}
	defer db.Close()

	if err := postgres.ApplyMigrations(ctx, db); err != nil {
		logger.Error("apply database migrations", "error", err)
		os.Exit(1)
	}
	if err := seed.Load(ctx, db); err != nil {
		logger.Error("load seed data", "error", err)
		os.Exit(1)
	}

	logger.Info("seed data loaded", "demo_email", "demo@example.com")
}
