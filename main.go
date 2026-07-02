package main

import (
	"context"
	"log"

	"github.com/tejasvi-mehra/currency-hedge-calculator/internal/config"
	frameworkcontext "github.com/tejasvi-mehra/currency-hedge-calculator/internal/framework/context"
	frameworklogger "github.com/tejasvi-mehra/currency-hedge-calculator/internal/framework/logger"
)

// main loads config, wires dependencies, and starts the API runtime.
func main() {
	cfg, err := config.LoadFromEnv()
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	logger, err := frameworklogger.Build(frameworklogger.Config{
		Mode:     cfg.AppEnv,
		Level:    cfg.LogLevel,
		Encoding: cfg.LogEncoding,
		Name:     cfg.AppName,
	})
	if err != nil {
		log.Fatalf("build logger: %v", err)
	}
	defer func() {
		_ = logger.Sync()
	}()

	app, err := NewApp(cfg, logger)
	if err != nil {
		log.Fatalf("build app: %v", err)
	}

	ctx, cancel := frameworkcontext.WithShutdownSignals(context.Background())
	defer cancel()

	if err := app.Run(ctx); err != nil {
		logger.Errorw("service stopped with error", "error", err)
		log.Fatalf("run app: %v", err)
	}
}
