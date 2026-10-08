package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"whatsapp-ai-autentication/internal/adapters/backend"
	"whatsapp-ai-autentication/internal/adapters/cryptography"
	"whatsapp-ai-autentication/internal/adapters/persistence"
	authhttp "whatsapp-ai-autentication/internal/adapters/transport/http"
	"whatsapp-ai-autentication/internal/config"
	"whatsapp-ai-autentication/internal/usecase"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	cfg, err := config.Load()
	if err != nil {
		logger.Error("invalid configuration", "error", err)
		os.Exit(1)
	}

	migrator, err := migrate.New("file://migrations", cfg.DatabaseURL)
	if err != nil {
		logger.Error("create database migrator", "error", err)
		os.Exit(1)
	}
	if err := migrator.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		logger.Error("apply database migrations", "error", err)
		os.Exit(1)
	}
	if sourceErr, databaseErr := migrator.Close(); sourceErr != nil || databaseErr != nil {
		logger.Error("close database migrator", "source_error", sourceErr, "database_error", databaseErr)
		os.Exit(1)
	}

	db, err := gorm.Open(postgres.Open(cfg.DatabaseURL), &gorm.Config{})
	if err != nil {
		logger.Error("connect to PostgreSQL", "error", err)
		os.Exit(1)
	}

	accountRepo := persistence.NewAccountRepository(db)
	signer, err := cryptography.NewTokenSigner(cfg.PrivateKeyPath, cfg.KeyID, cfg.Issuer, cfg.Audience, cfg.AccessTokenTTL)
	if err != nil {
		logger.Error("load signing key", "error", err)
		os.Exit(1)
	}
	backendClient := backend.NewClient(cfg.BackendURL, cfg.InternalServiceToken, cfg.BackendTimeout)
	service := usecase.NewService(
		accountRepo,
		cryptography.NewPasswordHasher(),
		signer,
		backendClient,
		cfg.RefreshTokenTTL,
		cfg.LoginLockBase,
		cfg.LoginLockMaximum,
		logger,
	)

	server := &http.Server{
		Addr:              cfg.ListenAddress,
		Handler:           authhttp.NewHandler(service, signer, cfg, db),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go service.RetryPendingOnboarding(ctx, cfg.OnboardingRetryInterval)
	go func() {
		logger.Info("authentication service listening", "address", cfg.ListenAddress)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("HTTP server failed", "error", err)
			stop()
		}
	}()

	<-ctx.Done()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.Error("HTTP server shutdown failed", "error", err)
	}
}
