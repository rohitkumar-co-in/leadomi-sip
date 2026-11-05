// Package main is the entry point for the Leadomi SIP application
package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/rohitkumar-co-in/leadomi-sip/internal/api"
	"github.com/rohitkumar-co-in/leadomi-sip/internal/config"
	"github.com/rohitkumar-co-in/leadomi-sip/internal/db"
	"github.com/rohitkumar-co-in/leadomi-sip/internal/twilio"
	"github.com/rohitkumar-co-in/leadomi-sip/pkg/sip"
)

func main() {
	// Initialize structured logging
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	slog.Info("Starting Leadomi SIP", "version", "1.0.0")

	// Load configuration
	cfg := config.Load()

	// Ensure data directories exist
	if err := cfg.EnsureDirectories(); err != nil {
		slog.Error("Failed to create data directories", "error", err)
		os.Exit(1)
	}

	// Initialize database
	database, err := db.New(cfg.DBPath())
	if err != nil {
		slog.Error("Failed to initialize database", "error", err)
		os.Exit(1)
	}
	defer database.Close()

	// Run migrations
	if err := database.Migrate(); err != nil {
		slog.Error("Failed to run database migrations", "error", err)
		os.Exit(1)
	}
	if err := cfg.LoadTwilioEnv(); err != nil {
		slog.Error("Failed to load Twilio environment file")
		os.Exit(1)
	}
	if err := os.Remove(filepath.Join(cfg.DataDir, "twilio-accounts.json")); err != nil && !os.IsNotExist(err) {
		slog.Error("Failed to remove legacy Twilio account credentials")
		os.Exit(1)
	}
	if err := database.Config.MigrateTwilioEnv(cfg); err != nil {
		slog.Error("Failed to migrate Twilio credentials to environment file")
		os.Exit(1)
	}

	// Create context with cancellation for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Initialize SIP server
	sipServer, err := sip.NewServer(sip.Config{
		Port:      cfg.SIPPort,
		UserAgent: config.DefaultUserAgent,
		MOHPath:   cfg.MOHFilePath(""),
	}, database)
	if err != nil {
		slog.Error("Failed to initialize SIP server", "error", err)
		os.Exit(1)
	}

	// Start SIP server
	if err := sipServer.Start(ctx); err != nil {
		slog.Error("Failed to start SIP server", "error", err)
		os.Exit(1)
	}
	slog.Info("SIP server started", "port", cfg.SIPPort)

	// Initialize Twilio client
	twilioClient := twilio.NewClient(cfg)

	twilioClient.Start(ctx)
	defer twilioClient.Stop()
	slog.Info("Twilio client initialized")

	// Initialize and start HTTP server
	router := api.NewRouter(&api.Dependencies{
		Config: cfg,
		DB:     database,
		SIP:    sipServer,
		Twilio: twilioClient,
	})

	httpServer := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.HTTPPort),
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start HTTP server in goroutine
	go func() {
		slog.Info("HTTP server started", "port", cfg.HTTPPort)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("HTTP server error", "error", err)
			cancel()
		}
	}()

	// Wait for interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	<-sigChan
	slog.Info("Shutdown signal received, initiating graceful shutdown...")

	// Graceful shutdown with timeout
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	// Shutdown HTTP server
	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		slog.Error("HTTP server shutdown error", "error", err)
	}

	// Stop SIP server
	sipServer.Stop()

	slog.Info("Leadomi SIP shutdown complete")
}
