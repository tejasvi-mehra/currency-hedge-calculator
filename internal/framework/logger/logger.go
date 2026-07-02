package logger

import (
	"fmt"
	"strings"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// Config controls logger construction.
type Config struct {
	Mode     string
	Level    string
	Encoding string
	Name     string
}

// Build creates a configured zap SugaredLogger.
func Build(cfg Config) (*zap.SugaredLogger, error) {
	zapCfg := zap.NewProductionConfig()
	if strings.EqualFold(strings.TrimSpace(cfg.Mode), "development") {
		zapCfg = zap.NewDevelopmentConfig()
	}

	level, err := zapcore.ParseLevel(strings.ToLower(strings.TrimSpace(cfg.Level)))
	if err != nil {
		return nil, fmt.Errorf("parse log level: %w", err)
	}
	zapCfg.Level = zap.NewAtomicLevelAt(level)

	encoding := strings.ToLower(strings.TrimSpace(cfg.Encoding))
	if encoding == "" {
		encoding = "console"
	}
	if encoding != "json" && encoding != "console" {
		return nil, fmt.Errorf("unsupported log encoding: %s", cfg.Encoding)
	}
	zapCfg.Encoding = encoding

	logger, err := zapCfg.Build(zap.AddCallerSkip(1))
	if err != nil {
		return nil, fmt.Errorf("build zap logger: %w", err)
	}

	name := strings.TrimSpace(cfg.Name)
	if name == "" {
		name = "currency-hedge-calculator"
	}
	return logger.Named(name).Sugar(), nil
}
