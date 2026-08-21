package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/wyw14/cry-072/internal/application"
	"github.com/wyw14/cry-072/internal/config"
	"github.com/wyw14/cry-072/internal/domain"
	"github.com/wyw14/cry-072/internal/platform"
	"github.com/wyw14/cry-072/internal/repository/memory"
	"github.com/wyw14/cry-072/internal/repository/postgres"
	httptransport "github.com/wyw14/cry-072/internal/transport/http"
	"go.uber.org/zap"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	logger, err := buildLogger(cfg.Environment)
	if err != nil {
		return fmt.Errorf("build logger: %w", err)
	}
	defer func() { _ = logger.Sync() }()

	rootContext, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	repository, closeRepository, err := openRepository(rootContext, cfg, logger)
	if err != nil {
		return err
	}
	defer closeRepository()

	clock := platform.NewUTCSource()
	ids := platform.RandomIDGenerator{}
	if cfg.DatabaseURL == "" {
		if err := application.SeedDemo(rootContext, repository, clock.Now()); err != nil {
			return err
		}
	}
	notifier := platform.NewLocalNotificationAdapter()
	attachments, err := platform.NewLocalAttachmentStore(cfg.AttachmentDir, cfg.AttachmentMaxBytes)
	if err != nil {
		return fmt.Errorf("open attachment store: %w", err)
	}
	catalog := application.NewCatalogService(repository, clock, ids)
	rules := application.NewRuleService(repository, clock, ids)
	hazards := application.NewHazardService(repository, clock, ids)
	workflow := application.NewWorkflowService(repository, clock, ids)
	remediation := application.NewRemediationService(repository, clock, ids)
	notifications := application.NewNotificationService(repository, clock, ids, notifier)
	analytics := application.NewAnalyticsService(repository, clock)
	reports := application.NewReportService(repository)
	api := httptransport.NewAPI(catalog, rules, hazards, workflow, remediation, notifications, analytics, reports, attachments)
	role := domain.ParseRole(cfg.DemoOperatorRole)
	router := httptransport.NewRouter(api, repository, logger, cfg.DemoOperatorID, role)
	server := &http.Server{
		Addr: cfg.HTTPAddress, Handler: router, ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout: 15 * time.Second, WriteTimeout: 30 * time.Second, IdleTimeout: 60 * time.Second,
	}

	schedulerDone := make(chan struct{})
	go runScheduler(rootContext, logger, notifications, schedulerDone)
	serverError := make(chan error, 1)
	go func() {
		logger.Info("http_server_started", zap.String("address", cfg.HTTPAddress), zap.String("environment", cfg.Environment))
		serverError <- server.ListenAndServe()
	}()

	select {
	case <-rootContext.Done():
		logger.Info("shutdown_requested")
	case err := <-serverError:
		if !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("serve HTTP: %w", err)
		}
	}
	shutdownContext, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()
	if err := server.Shutdown(shutdownContext); err != nil {
		return fmt.Errorf("shutdown HTTP server: %w", err)
	}
	stop()
	select {
	case <-schedulerDone:
	case <-shutdownContext.Done():
		return fmt.Errorf("stop scheduler: %w", shutdownContext.Err())
	}
	return nil
}

func openRepository(ctx context.Context, cfg config.Config, logger *zap.Logger) (application.Repository, func(), error) {
	if cfg.DatabaseURL == "" {
		logger.Warn("database_url_empty_using_memory_repository")
		return memory.New(), func() {}, nil
	}
	repository, err := postgres.Open(ctx, cfg.DatabaseURL, cfg.DatabaseTimeout)
	if err != nil {
		return nil, nil, err
	}
	return repository, repository.Close, nil
}

func runScheduler(ctx context.Context, logger *zap.Logger, notifications *application.NotificationService, done chan<- struct{}) {
	defer close(done)
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			processed, err := notifications.ProcessDue(ctx, 100)
			if err != nil {
				logger.Error("notification_cycle_failed", zap.Error(err))
				continue
			}
			if processed > 0 {
				logger.Info("notification_cycle_completed", zap.Int("processed", processed))
			}
		}
	}
}

func buildLogger(environment string) (*zap.Logger, error) {
	if environment == "production" {
		return zap.NewProduction()
	}
	return zap.NewDevelopment()
}
