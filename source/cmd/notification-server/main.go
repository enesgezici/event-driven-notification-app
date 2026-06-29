package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/yourusername/event-driven-notification-app/config"
	"github.com/yourusername/event-driven-notification-app/internal/api"
	"github.com/yourusername/event-driven-notification-app/internal/metrics"
	"github.com/yourusername/event-driven-notification-app/internal/provider"
	"github.com/yourusername/event-driven-notification-app/internal/queue"
	"github.com/yourusername/event-driven-notification-app/internal/storage"
)

func main() {
	cfg := config.LoadConfig()

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	if cfg.WebhookURL == "" {
		logger.Error("missing required configuration", "env", "WEBHOOK_URL")
		os.Exit(1)
	}

	db, err := storage.NewPostgresStorage(cfg.DatabaseURL)
	if err != nil {
		logger.Error("database initialization failed", "error", err)
		os.Exit(1)
	}
	defer db.Close()

	if err := db.Migrate(); err != nil {
		logger.Error("database migration failed", "error", err)
		os.Exit(1)
	}

	providerClient := provider.NewWebhookProvider(cfg.WebhookURL, logger)
	metricsCollector := metrics.NewCollector()
	queueManager := queue.NewManager(db, providerClient, metricsCollector, logger)
	workerCtx, stopWorkers := context.WithCancel(context.Background())
	defer stopWorkers()
	queueManager.StartWorkers(workerCtx)

	router := api.NewRouter(db, queueManager, metricsCollector, logger)

	srv := &http.Server{
		Addr:    cfg.ServerAddress,
		Handler: router,
	}

	go func() {
		logger.Info("server listening", "address", cfg.ServerAddress)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("server failed", "error", err)
			os.Exit(1)
		}
	}()

	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, syscall.SIGINT, syscall.SIGTERM)
	<-shutdown

	logger.Info("shutdown requested")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		logger.Error("server shutdown failed", "error", err)
	}
	stopWorkers()
	queueManager.StopWorkers()
}
