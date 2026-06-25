package main

import (
	"context"
	"log"
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

	logger := log.New(os.Stdout, "notification-app ", log.LstdFlags|log.Lmsgprefix)

	db, err := storage.NewSQLiteStorage(cfg.DatabasePath)
	if err != nil {
		logger.Fatalf("failed to initialize database: %v", err)
	}
	defer db.Close()

	if err := db.Migrate(); err != nil {
		logger.Fatalf("failed to run migrations: %v", err)
	}

	providerClient := provider.NewWebhookProvider(cfg.WebhookURL, logger)
	metricsCollector := metrics.NewCollector()
	queueManager := queue.NewManager(db, providerClient, metricsCollector, logger)
	queueManager.StartWorkers(context.Background())

	router := api.NewRouter(db, queueManager, metricsCollector, logger)

	srv := &http.Server{
		Addr:    cfg.ServerAddress,
		Handler: router,
	}

	go func() {
		logger.Printf("server listening on %s", cfg.ServerAddress)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatalf("server error: %v", err)
		}
	}()

	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, syscall.SIGINT, syscall.SIGTERM)
	<-shutdown

	logger.Println("shutdown requested, stopping server...")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		logger.Printf("server shutdown error: %v", err)
	}
}
