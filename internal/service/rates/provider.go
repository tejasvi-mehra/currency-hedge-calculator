package rates

import (
	"context"
	"errors"
	"time"
)

var (
	// ErrUnsupportedCurrencyPair indicates the external API does not support the requested pair.
	ErrUnsupportedCurrencyPair = errors.New("unsupported currency pair")
	// ErrRateUnavailable indicates no live or fallback rate is available.
	ErrRateUnavailable = errors.New("rate unavailable")
)

// Quote represents a single FX quote with provenance information.
type Quote struct {
	SettlementCurrency  string
	PresentmentCurrency string
	Rate                float64
	Timestamp           time.Time
	Source              string
	Stale               bool
}

// Provider defines live exchange-rate retrieval for exposure calculations.
type Provider interface {
	GetPresentmentPerSettlementRate(ctx context.Context, settlementCurrency string, presentmentCurrency string) (Quote, error)
}
