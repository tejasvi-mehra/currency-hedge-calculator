package exposure

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/tejasvi-mehra/currency-hedge-calculator/internal/service/rates"
)

type stubRatesProvider struct {
	quotes map[string]rates.Quote
	err    error
}

func (s *stubRatesProvider) GetPresentmentPerSettlementRate(_ context.Context, settlementCurrency string, presentmentCurrency string) (rates.Quote, error) {
	if s.err != nil {
		return rates.Quote{}, s.err
	}
	key := settlementCurrency + ":" + presentmentCurrency
	return s.quotes[key], nil
}

type stubTransactionSource struct {
	transactions []PendingTransaction
}

func (s *stubTransactionSource) ListPending(_ context.Context) ([]PendingTransaction, error) {
	result := make([]PendingTransaction, len(s.transactions))
	copy(result, s.transactions)
	return result, nil
}

func TestServiceCalculateExposure_MixedResults(t *testing.T) {
	provider := &stubRatesProvider{
		quotes: map[string]rates.Quote{
			"EUR:BRL": {SettlementCurrency: "EUR", PresentmentCurrency: "BRL", Rate: 6.3, Timestamp: time.Now().UTC(), Source: "test-live"},
			"USD:MXN": {SettlementCurrency: "USD", PresentmentCurrency: "MXN", Rate: 17.5, Timestamp: time.Now().UTC(), Source: "test-live"},
		},
	}

	service := NewService(provider, nil, 2, nil)
	response, err := service.CalculateExposure(context.Background(), CalculateExposureRequest{
		Transactions: []PendingTransaction{
			{
				TransactionID:          "TX-1",
				AuthorizationTimestamp: time.Now().Add(-10 * 24 * time.Hour).UTC(),
				AuthorizedAmount:       14400,
				PresentmentCurrency:    "BRL",
				SettlementCurrency:     "EUR",
				AuthorizationRate:      6.0,
			},
			{
				TransactionID:          "TX-2",
				AuthorizationTimestamp: time.Now().Add(-4 * 24 * time.Hour).UTC(),
				AuthorizedAmount:       17000,
				PresentmentCurrency:    "MXN",
				SettlementCurrency:     "USD",
				AuthorizationRate:      18.0,
			},
		},
	})
	if err != nil {
		t.Fatalf("CalculateExposure() unexpected error: %v", err)
	}

	if response.Summary.TotalExposureAmount != -87.3016 {
		t.Fatalf("unexpected total exposure: got %v", response.Summary.TotalExposureAmount)
	}
	if response.Summary.LossCount != 1 || response.Summary.GainCount != 1 {
		t.Fatalf("unexpected gain/loss counts: %+v", response.Summary)
	}
	if len(response.Transactions) != 2 {
		t.Fatalf("unexpected transaction count: %d", len(response.Transactions))
	}
	if response.Transactions[0].TransactionID != "TX-1" {
		t.Fatalf("expected TX-1 to be worst (sorted first), got %s", response.Transactions[0].TransactionID)
	}
	if !response.Transactions[0].IsHighRisk {
		t.Fatalf("expected TX-1 to be high risk")
	}
}

func TestServiceCalculateExposure_UsesSeedSourceWhenRequested(t *testing.T) {
	service := NewService(
		&stubRatesProvider{
			quotes: map[string]rates.Quote{
				"USD:ARS": {SettlementCurrency: "USD", PresentmentCurrency: "ARS", Rate: 1000, Timestamp: time.Now().UTC(), Source: "test-live"},
			},
		},
		&stubTransactionSource{
			transactions: []PendingTransaction{
				{
					TransactionID:          "TX-SEED",
					AuthorizationTimestamp: time.Now().Add(-24 * time.Hour).UTC(),
					AuthorizedAmount:       100000,
					PresentmentCurrency:    "ARS",
					SettlementCurrency:     "USD",
					AuthorizationRate:      950,
				},
			},
		},
		2,
		nil,
	)

	response, err := service.CalculateExposure(context.Background(), CalculateExposureRequest{
		UseDefaultTestDataWhenEmptyInput: true,
	})
	if err != nil {
		t.Fatalf("CalculateExposure() unexpected error: %v", err)
	}
	if len(response.Transactions) != 1 {
		t.Fatalf("expected one transaction, got %d", len(response.Transactions))
	}
	if response.Transactions[0].TransactionID != "TX-SEED" {
		t.Fatalf("unexpected transaction returned: %+v", response.Transactions[0])
	}
}

func TestServiceCalculateExposure_ValidationError(t *testing.T) {
	service := NewService(&stubRatesProvider{}, nil, 2, nil)

	_, err := service.CalculateExposure(context.Background(), CalculateExposureRequest{
		Transactions: []PendingTransaction{
			{
				TransactionID:          "TX-BAD",
				AuthorizationTimestamp: time.Now().UTC(),
				AuthorizedAmount:       100,
				PresentmentCurrency:    "BRL",
				SettlementCurrency:     "EUR",
				AuthorizationRate:      0,
			},
		},
	})
	if err == nil {
		t.Fatalf("expected validation error")
	}
	if !errors.Is(err, ErrValidation) {
		t.Fatalf("expected ErrValidation, got %v", err)
	}
}
