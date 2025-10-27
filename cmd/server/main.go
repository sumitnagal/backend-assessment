package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"backend-assessment/internal/api"
	"backend-assessment/internal/config"
    "backend-assessment/internal/observability"
	"backend-assessment/internal/datastore"

	log "github.com/sirupsen/logrus"
)

func main() {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Setup logging
	setupLogging(cfg)

	// Connect to database
	db, err := datastore.NewPostgresConnection(cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

    // Initialize tracing
    shutdownTracing, err := observability.InitTracing(context.Background(), cfg)
    if err != nil {
        log.Fatalf("Failed to init tracing: %v", err)
    }
    defer func() {
        if err := shutdownTracing(context.Background()); err != nil {
            log.Errorf("tracing shutdown error: %v", err)
        }
    }()

	// Create API server
	server := api.NewServer(cfg, db)

	// Setup HTTP server
	httpServer := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.Port),
		Handler:      server.Router(),
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start server in goroutine
	go func() {
		log.Infof("Starting server on port %d", cfg.Port)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Failed to start server: %v", err)
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Info("Shutting down server...")

	// Create shutdown context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Attempt graceful shutdown
	if err := httpServer.Shutdown(ctx); err != nil {
		log.Errorf("Server forced to shutdown: %v", err)
	}

	log.Info("Server exited")
}

func setupLogging(cfg *config.Config) {
	if cfg.LogLevel == "debug" {
		log.SetLevel(log.DebugLevel)
	} else if cfg.LogLevel == "info" {
		log.SetLevel(log.InfoLevel)
	} else {
		log.SetLevel(log.WarnLevel)
	}

	log.SetFormatter(&log.JSONFormatter{})
}