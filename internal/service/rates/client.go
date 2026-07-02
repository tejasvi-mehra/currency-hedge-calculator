package rates

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/tejasvi-mehra/currency-hedge-calculator/internal/config"
	"github.com/tejasvi-mehra/currency-hedge-calculator/internal/framework/backoff"
	frameworkhttp "github.com/tejasvi-mehra/currency-hedge-calculator/internal/framework/http_connector"
	"go.uber.org/zap"
)

type erAPIResponse struct {
	Result             string             `json:"result"`
	ErrorType          string             `json:"error-type"`
	TimeLastUpdateUnix int64              `json:"time_last_update_unix"`
	Rates              map[string]float64 `json:"rates"`
}

// LiveProvider fetches FX rates from a public API with retry and stale-cache fallback.
type LiveProvider struct {
	cfg           config.FXConfig
	httpConnector *frameworkhttp.Connector
	cache         *MemoryCache
	retryStrategy backoff.Strategy
	supportedSet  map[string]struct{}
	logger        *zap.SugaredLogger
}

// NewLiveProvider constructs a live rates provider.
func NewLiveProvider(
	cfg config.FXConfig,
	httpConnector *frameworkhttp.Connector,
	cache *MemoryCache,
	retryStrategy backoff.Strategy,
	logger *zap.SugaredLogger,
) *LiveProvider {
	if logger == nil {
		logger = zap.NewNop().Sugar()
	}
	if cache == nil {
		cache = NewMemoryCache(cfg.CacheTTL)
	}

	supportedSet := map[string]struct{}{}
	for _, currency := range cfg.SupportedCurrencies {
		supportedSet[strings.ToUpper(strings.TrimSpace(currency))] = struct{}{}
	}

	return &LiveProvider{
		cfg:           cfg,
		httpConnector: httpConnector,
		cache:         cache,
		retryStrategy: retryStrategy,
		supportedSet:  supportedSet,
		logger:        logger,
	}
}

// GetPresentmentPerSettlementRate retrieves a rate with cache-first + retry semantics.
func (p *LiveProvider) GetPresentmentPerSettlementRate(ctx context.Context, settlementCurrency string, presentmentCurrency string) (Quote, error) {
	settlement := normalizeCurrency(settlementCurrency)
	presentment := normalizeCurrency(presentmentCurrency)

	if settlement == "" || presentment == "" {
		return Quote{}, fmt.Errorf("%w: empty currency code", ErrUnsupportedCurrencyPair)
	}
	if settlement == presentment {
		return Quote{
			SettlementCurrency:  settlement,
			PresentmentCurrency: presentment,
			Rate:                1,
			Timestamp:           time.Now().UTC(),
			Source:              "identity",
			Stale:               false,
		}, nil
	}
	if err := p.validateCurrencyPair(settlement, presentment); err != nil {
		return Quote{}, err
	}

	if quote, ok := p.cache.GetFresh(settlement, presentment); ok {
		return quote, nil
	}

	quote, err := p.fetchWithRetry(ctx, settlement, presentment)
	if err == nil {
		p.cache.Set(quote)
		return quote, nil
	}

	if cachedQuote, ok := p.cache.GetAny(settlement, presentment); ok {
		cachedQuote.Stale = true
		cachedQuote.Source = cachedQuote.Source + "-stale-cache"
		p.logger.Warnw("using stale FX quote fallback",
			"settlement_currency", settlement,
			"presentment_currency", presentment,
			"error", err,
		)
		return cachedQuote, nil
	}

	return Quote{}, fmt.Errorf("%w: %v", ErrRateUnavailable, err)
}

func (p *LiveProvider) fetchWithRetry(ctx context.Context, settlement string, presentment string) (Quote, error) {
	attempts := p.cfg.RetryMaxAttempts
	if attempts < 1 {
		attempts = 1
	}

	var lastErr error
	for attempt := 1; attempt <= attempts; attempt++ {
		quote, err := p.fetchOnce(ctx, settlement, presentment)
		if err == nil {
			return quote, nil
		}
		lastErr = err

		if !isRetryable(err) || attempt == attempts {
			break
		}
		p.logger.Warnw("live FX fetch failed, retrying",
			"attempt", attempt,
			"max_attempts", attempts,
			"settlement_currency", settlement,
			"presentment_currency", presentment,
			"error", err,
		)
		if waitErr := backoff.Sleep(ctx, p.retryStrategy, attempt); waitErr != nil {
			return Quote{}, waitErr
		}
	}

	return Quote{}, lastErr
}

func (p *LiveProvider) fetchOnce(ctx context.Context, settlement string, presentment string) (Quote, error) {
	endpoint := fmt.Sprintf("%s/v6/latest/%s", p.cfg.BaseURL, settlement)
	response := erAPIResponse{}

	if err := p.httpConnector.GetJSON(ctx, endpoint, nil, &response); err != nil {
		return Quote{}, err
	}
	if !strings.EqualFold(response.Result, "success") {
		return Quote{}, fmt.Errorf("fx api result=%q error_type=%q", response.Result, response.ErrorType)
	}

	rate, ok := response.Rates[presentment]
	if !ok || rate <= 0 {
		return Quote{}, fmt.Errorf("%w: %s/%s not found in response", ErrUnsupportedCurrencyPair, settlement, presentment)
	}

	timestamp := time.Now().UTC()
	if response.TimeLastUpdateUnix > 0 {
		timestamp = time.Unix(response.TimeLastUpdateUnix, 0).UTC()
	}

	return Quote{
		SettlementCurrency:  settlement,
		PresentmentCurrency: presentment,
		Rate:                rate,
		Timestamp:           timestamp,
		Source:              "open-er-api",
		Stale:               false,
	}, nil
}

func (p *LiveProvider) validateCurrencyPair(settlement string, presentment string) error {
	if len(p.supportedSet) == 0 {
		return nil
	}
	if _, ok := p.supportedSet[settlement]; !ok {
		return fmt.Errorf("%w: settlement currency %s", ErrUnsupportedCurrencyPair, settlement)
	}
	if _, ok := p.supportedSet[presentment]; !ok {
		return fmt.Errorf("%w: presentment currency %s", ErrUnsupportedCurrencyPair, presentment)
	}
	return nil
}

func normalizeCurrency(code string) string {
	return strings.ToUpper(strings.TrimSpace(code))
}

func isRetryable(err error) bool {
	var httpErr *frameworkhttp.HTTPError
	if errors.As(err, &httpErr) {
		return httpErr.StatusCode == http.StatusTooManyRequests || httpErr.StatusCode >= 500
	}
	return true
}
