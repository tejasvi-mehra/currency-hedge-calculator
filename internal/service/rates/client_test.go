package rates

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/tejasvi-mehra/currency-hedge-calculator/internal/config"
	"github.com/tejasvi-mehra/currency-hedge-calculator/internal/framework/backoff"
	frameworkhttp "github.com/tejasvi-mehra/currency-hedge-calculator/internal/framework/http_connector"
)

func TestLiveProvider_RetryThenSuccess(t *testing.T) {
	var attempts int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		current := atomic.AddInt32(&attempts, 1)
		if current == 1 {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"error":"temporary"}`))
			return
		}

		_ = json.NewEncoder(w).Encode(map[string]any{
			"result":                "success",
			"time_last_update_unix": time.Now().Unix(),
			"rates": map[string]float64{
				"BRL": 6.2,
			},
		})
	}))
	defer server.Close()

	provider := NewLiveProvider(
		config.FXConfig{
			BaseURL:             server.URL,
			RetryMaxAttempts:    3,
			RetryInitial:        time.Millisecond,
			RetryMax:            time.Millisecond * 2,
			CacheTTL:            time.Minute,
			SupportedCurrencies: []string{"EUR", "BRL"},
		},
		frameworkhttp.New(2*time.Second, nil),
		NewMemoryCache(time.Minute),
		backoff.Exponential{Initial: time.Millisecond, Max: time.Millisecond * 2},
		nil,
	)

	quote, err := provider.GetPresentmentPerSettlementRate(context.Background(), "EUR", "BRL")
	if err != nil {
		t.Fatalf("GetPresentmentPerSettlementRate() error = %v", err)
	}
	if quote.Rate != 6.2 {
		t.Fatalf("expected rate 6.2, got %v", quote.Rate)
	}
	if atomic.LoadInt32(&attempts) != 2 {
		t.Fatalf("expected 2 attempts, got %d", attempts)
	}
}

func TestLiveProvider_StaleCacheFallback(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte(`{"error":"down"}`))
	}))
	defer server.Close()

	cache := NewMemoryCache(time.Nanosecond)
	cache.Set(Quote{
		SettlementCurrency:  "USD",
		PresentmentCurrency: "MXN",
		Rate:                17.9,
		Timestamp:           time.Now().Add(-10 * time.Minute).UTC(),
		Source:              "test-stale",
	})
	time.Sleep(2 * time.Nanosecond)

	provider := NewLiveProvider(
		config.FXConfig{
			BaseURL:             server.URL,
			RetryMaxAttempts:    1,
			RetryInitial:        time.Millisecond,
			RetryMax:            time.Millisecond,
			CacheTTL:            time.Nanosecond,
			SupportedCurrencies: []string{"USD", "MXN"},
		},
		frameworkhttp.New(2*time.Second, nil),
		cache,
		backoff.Exponential{Initial: time.Millisecond, Max: time.Millisecond},
		nil,
	)

	quote, err := provider.GetPresentmentPerSettlementRate(context.Background(), "USD", "MXN")
	if err != nil {
		t.Fatalf("GetPresentmentPerSettlementRate() fallback error = %v", err)
	}
	if !quote.Stale {
		t.Fatalf("expected stale fallback quote")
	}
}

func TestLiveProvider_UnsupportedCurrency(t *testing.T) {
	provider := NewLiveProvider(
		config.FXConfig{
			BaseURL:             "https://example.com",
			RetryMaxAttempts:    1,
			RetryInitial:        time.Millisecond,
			RetryMax:            time.Millisecond,
			CacheTTL:            time.Minute,
			SupportedCurrencies: []string{"USD", "EUR"},
		},
		frameworkhttp.New(2*time.Second, nil),
		NewMemoryCache(time.Minute),
		backoff.Exponential{Initial: time.Millisecond, Max: time.Millisecond},
		nil,
	)

	_, err := provider.GetPresentmentPerSettlementRate(context.Background(), "USD", "BRL")
	if err == nil {
		t.Fatalf("expected unsupported currency pair error")
	}
	if !strings.Contains(err.Error(), "unsupported currency pair") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLiveProvider_HistoricalRate(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/v6/latest/") {
			_ = json.NewEncoder(w).Encode(map[string]any{
				"result":                "success",
				"time_last_update_unix": time.Now().Unix(),
				"rates": map[string]float64{
					"BRL": 6.2,
				},
			})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"amount": 1,
			"base":   "EUR",
			"date":   "2026-06-20",
			"rates": map[string]float64{
				"BRL": 6.0,
			},
		})
	}))
	defer server.Close()

	provider := NewLiveProvider(
		config.FXConfig{
			BaseURL:             server.URL,
			HistoricalBaseURL:   server.URL,
			RetryMaxAttempts:    2,
			RetryInitial:        time.Millisecond,
			RetryMax:            time.Millisecond * 2,
			CacheTTL:            time.Minute,
			SupportedCurrencies: []string{"EUR", "BRL"},
		},
		frameworkhttp.New(2*time.Second, nil),
		NewMemoryCache(time.Minute),
		backoff.Exponential{Initial: time.Millisecond, Max: time.Millisecond * 2},
		nil,
	)

	quote, err := provider.GetHistoricalPresentmentPerSettlementRate(context.Background(), "EUR", "BRL", time.Date(2026, 6, 20, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("GetHistoricalPresentmentPerSettlementRate() error = %v", err)
	}
	if quote.Rate != 6.0 {
		t.Fatalf("expected historical rate 6.0, got %v", quote.Rate)
	}
}
