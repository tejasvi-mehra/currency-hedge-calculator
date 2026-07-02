package config

import (
	"fmt"
	"strings"
	"time"

	"github.com/caarlos0/env/v11"
)

var defaultSupportedCurrencies = []string{"USD", "EUR", "BRL", "MXN", "COP", "ARS", "CLP"}

// Config defines all runtime configuration loaded from environment variables.
type Config struct {
	AppName     string `env:"APP_NAME" envDefault:"currency-hedge-calculator"`
	AppEnv      string `env:"APP_ENV" envDefault:"development"`
	LogLevel    string `env:"LOG_LEVEL" envDefault:"info"`
	LogEncoding string `env:"LOG_ENCODING" envDefault:"console"`
	Server      ServerConfig
	FX          FXConfig
	Exposure    ExposureConfig
	Data        DataConfig
}

// ServerConfig controls the HTTP API server.
type ServerConfig struct {
	ListenAddr     string   `env:"SERVER_LISTEN_ADDR" envDefault:":8080"`
	HealthPath     string   `env:"SERVER_HEALTH_PATH" envDefault:"/healthz"`
	AllowedOrigins []string `env:"SERVER_ALLOWED_ORIGINS" envDefault:"*" envSeparator:","`
}

// FXConfig controls live rate fetching and resilience.
type FXConfig struct {
	BaseURL             string        `env:"FX_BASE_URL" envDefault:"https://open.er-api.com"`
	HistoricalBaseURL   string        `env:"FX_HISTORICAL_BASE_URL" envDefault:"https://api.frankfurter.app"`
	Timeout             time.Duration `env:"FX_TIMEOUT" envDefault:"5s"`
	RetryMaxAttempts    int           `env:"FX_RETRY_MAX_ATTEMPTS" envDefault:"3"`
	RetryInitial        time.Duration `env:"FX_RETRY_INITIAL" envDefault:"200ms"`
	RetryMax            time.Duration `env:"FX_RETRY_MAX" envDefault:"2s"`
	CacheTTL            time.Duration `env:"FX_CACHE_TTL" envDefault:"5m"`
	SupportedCurrencies []string      `env:"FX_SUPPORTED_CURRENCIES" envDefault:"USD,EUR,BRL,MXN,COP,ARS,CLP" envSeparator:","`
}

// ExposureConfig controls exposure ranking defaults.
type ExposureConfig struct {
	DefaultRiskThresholdPercentage float64 `env:"EXPOSURE_DEFAULT_RISK_THRESHOLD_PERCENTAGE" envDefault:"2"`
}

// DataConfig controls default test data sources.
type DataConfig struct {
	TestDataPath string `env:"DATA_TEST_DATA_PATH" envDefault:"data/analytics_test_transactions.json"`
}

// LoadFromEnv parses and validates runtime configuration.
func LoadFromEnv() (Config, error) {
	cfg := Config{}
	if err := env.Parse(&cfg); err != nil {
		return Config{}, fmt.Errorf("parse env: %w", err)
	}

	cfg.Server.ListenAddr = strings.TrimSpace(cfg.Server.ListenAddr)
	cfg.Server.HealthPath = strings.TrimSpace(cfg.Server.HealthPath)
	cfg.FX.BaseURL = strings.TrimRight(strings.TrimSpace(cfg.FX.BaseURL), "/")
	cfg.FX.HistoricalBaseURL = strings.TrimRight(strings.TrimSpace(cfg.FX.HistoricalBaseURL), "/")
	cfg.Data.TestDataPath = strings.TrimSpace(cfg.Data.TestDataPath)
	cfg.normalizeSupportedCurrencies()

	if cfg.Server.ListenAddr == "" {
		return Config{}, fmt.Errorf("SERVER_LISTEN_ADDR must not be empty")
	}
	if cfg.Server.HealthPath == "" {
		return Config{}, fmt.Errorf("SERVER_HEALTH_PATH must not be empty")
	}
	if cfg.FX.BaseURL == "" {
		return Config{}, fmt.Errorf("FX_BASE_URL must not be empty")
	}
	if cfg.FX.HistoricalBaseURL == "" {
		return Config{}, fmt.Errorf("FX_HISTORICAL_BASE_URL must not be empty")
	}
	if cfg.FX.RetryMaxAttempts < 1 {
		return Config{}, fmt.Errorf("FX_RETRY_MAX_ATTEMPTS must be >= 1")
	}
	if cfg.FX.RetryInitial <= 0 || cfg.FX.RetryMax <= 0 || cfg.FX.CacheTTL <= 0 {
		return Config{}, fmt.Errorf("FX retry/cache durations must be > 0")
	}
	if cfg.FX.RetryInitial > cfg.FX.RetryMax {
		return Config{}, fmt.Errorf("FX_RETRY_INITIAL must be <= FX_RETRY_MAX")
	}
	if cfg.Exposure.DefaultRiskThresholdPercentage < 0 {
		return Config{}, fmt.Errorf("EXPOSURE_DEFAULT_RISK_THRESHOLD_PERCENTAGE must be >= 0")
	}
	if cfg.Data.TestDataPath == "" {
		return Config{}, fmt.Errorf("DATA_TEST_DATA_PATH must not be empty")
	}

	return cfg, nil
}

func (c *Config) normalizeSupportedCurrencies() {
	if len(c.FX.SupportedCurrencies) == 0 {
		c.FX.SupportedCurrencies = append([]string{}, defaultSupportedCurrencies...)
		return
	}
	normalized := make([]string, 0, len(c.FX.SupportedCurrencies))
	seen := map[string]struct{}{}
	for _, code := range c.FX.SupportedCurrencies {
		trimmed := strings.ToUpper(strings.TrimSpace(code))
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		normalized = append(normalized, trimmed)
		seen[trimmed] = struct{}{}
	}
	if len(normalized) == 0 {
		c.FX.SupportedCurrencies = append([]string{}, defaultSupportedCurrencies...)
		return
	}
	c.FX.SupportedCurrencies = normalized
}
