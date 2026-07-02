package main

import (
	"context"
	"errors"
	"fmt"

	"github.com/tejasvi-mehra/currency-hedge-calculator/internal/config"
	"github.com/tejasvi-mehra/currency-hedge-calculator/internal/framework/backoff"
	frameworkhttp "github.com/tejasvi-mehra/currency-hedge-calculator/internal/framework/http_connector"
	frameworkserver "github.com/tejasvi-mehra/currency-hedge-calculator/internal/framework/server"
	"github.com/tejasvi-mehra/currency-hedge-calculator/internal/service/exposure"
	"github.com/tejasvi-mehra/currency-hedge-calculator/internal/service/rates"
	"github.com/tejasvi-mehra/currency-hedge-calculator/internal/service/transactions"
	"go.uber.org/zap"
)

// App is the composition root for runtime dependencies.
type App struct {
	server *frameworkserver.Server
	logger *zap.SugaredLogger
}

// NewApp wires framework and domain components.
func NewApp(cfg config.Config, logger *zap.SugaredLogger) (*App, error) {
	if logger == nil {
		logger = zap.NewNop().Sugar()
	}

	httpConnector := frameworkhttp.New(cfg.FX.Timeout, logger.Named("framework.http_connector"))
	rateCache := rates.NewMemoryCache(cfg.FX.CacheTTL)
	retryStrategy := backoff.Exponential{
		Initial: cfg.FX.RetryInitial,
		Max:     cfg.FX.RetryMax,
	}
	ratesProvider := rates.NewLiveProvider(
		cfg.FX,
		httpConnector,
		rateCache,
		retryStrategy,
		logger.Named("service.rates"),
	)

	store, err := transactions.NewFileStore(cfg.Data.TestDataPath)
	if err != nil {
		return nil, fmt.Errorf("build transaction store: %w", err)
	}

	exposureService := exposure.NewService(
		ratesProvider,
		store,
		cfg.Exposure.DefaultRiskThresholdPercentage,
		logger.Named("service.exposure"),
	)
	exposureHandler := exposure.NewHandler(exposureService, logger.Named("handler.exposure"))

	server := frameworkserver.New(cfg.Server.ListenAddr, logger.Named("framework.server"))
	server.UseCORS(cfg.Server.AllowedOrigins)
	server.UseRequestLogging()
	exposureHandler.Register(server, cfg.Server.HealthPath)

	return &App{
		server: server,
		logger: logger,
	}, nil
}

// Run starts the HTTP server and blocks until shutdown.
func (a *App) Run(ctx context.Context) error {
	errCh := make(chan error, 1)
	a.server.StartAsync(errCh)

	select {
	case <-ctx.Done():
		a.logger.Infow("shutdown signal received")
		if err := a.server.Shutdown(context.Background()); err != nil {
			return fmt.Errorf("shutdown server: %w", err)
		}
		return nil
	case err, ok := <-errCh:
		if !ok {
			return nil
		}
		if errors.Is(err, context.Canceled) {
			return nil
		}
		return fmt.Errorf("http server failed: %w", err)
	}
}
