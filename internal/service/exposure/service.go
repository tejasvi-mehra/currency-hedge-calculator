package exposure

import (
	"context"
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/tejasvi-mehra/currency-hedge-calculator/internal/service/rates"
	"go.uber.org/zap"
)

var (
	// ErrNoTransactions is returned when no pending transactions are available.
	ErrNoTransactions = errors.New("no pending transactions provided")
	// ErrValidation indicates malformed transaction payload data.
	ErrValidation = errors.New("validation error")
)

// TransactionSource loads pending transactions when the API request body is empty.
type TransactionSource interface {
	ListPending(ctx context.Context) ([]PendingTransaction, error)
}

// Clock abstracts current time for deterministic tests.
type Clock interface {
	Now() time.Time
}

type systemClock struct{}

func (systemClock) Now() time.Time { return time.Now().UTC() }

// Service encapsulates exposure calculations and ranking logic.
type Service struct {
	ratesProvider        rates.Provider
	transactionSource    TransactionSource
	defaultRiskThreshold float64
	clock                Clock
	logger               *zap.SugaredLogger
}

// NewService creates a new exposure service.
func NewService(
	ratesProvider rates.Provider,
	transactionSource TransactionSource,
	defaultRiskThreshold float64,
	logger *zap.SugaredLogger,
) *Service {
	if logger == nil {
		logger = zap.NewNop().Sugar()
	}
	if defaultRiskThreshold < 0 {
		defaultRiskThreshold = 0
	}

	return &Service{
		ratesProvider:        ratesProvider,
		transactionSource:    transactionSource,
		defaultRiskThreshold: defaultRiskThreshold,
		clock:                systemClock{},
		logger:               logger,
	}
}

// CalculateExposure computes transaction-level and aggregate exposure metrics.
func (s *Service) CalculateExposure(ctx context.Context, request CalculateExposureRequest) (CalculateExposureResponse, error) {
	transactions := request.Transactions
	if len(transactions) == 0 && request.UseSeedDataWhenEmptyInput {
		if s.transactionSource == nil {
			return CalculateExposureResponse{}, fmt.Errorf("%w: seed data unavailable", ErrNoTransactions)
		}
		var err error
		transactions, err = s.transactionSource.ListPending(ctx)
		if err != nil {
			return CalculateExposureResponse{}, fmt.Errorf("load pending transactions: %w", err)
		}
	}
	if len(transactions) == 0 {
		return CalculateExposureResponse{}, ErrNoTransactions
	}

	threshold := s.defaultRiskThreshold
	if request.RiskThresholdPercentage != nil {
		if *request.RiskThresholdPercentage < 0 {
			return CalculateExposureResponse{}, fmt.Errorf("%w: risk_threshold_percentage must be >= 0", ErrValidation)
		}
		threshold = *request.RiskThresholdPercentage
	}

	exposures := make([]TransactionExposure, 0, len(transactions))
	breakdown := map[string]*CurrencyExposureBreakdown{}

	var totalExposure float64
	var gainCount int
	var lossCount int
	var neutralCount int
	var highRiskCount int
	var staleRateCount int

	for _, transaction := range transactions {
		if err := validateTransaction(transaction); err != nil {
			return CalculateExposureResponse{}, err
		}

		quote, err := s.ratesProvider.GetPresentmentPerSettlementRate(ctx, transaction.SettlementCurrency, transaction.PresentmentCurrency)
		if err != nil {
			return CalculateExposureResponse{}, fmt.Errorf("fetch current rate for transaction %s: %w", transaction.TransactionID, err)
		}

		originalSettlement := transaction.AuthorizedAmount / transaction.AuthorizationRate
		currentSettlement := transaction.AuthorizedAmount / quote.Rate
		exposureAmount := currentSettlement - originalSettlement

		exposurePercentage := 0.0
		if originalSettlement != 0 {
			exposurePercentage = (exposureAmount / originalSettlement) * 100
		}

		isHighRisk := exposurePercentage <= (-1 * threshold)
		recommendation := "monitor"
		if isHighRisk {
			recommendation = "capture_now"
			highRiskCount++
		}

		if exposureAmount > 0 {
			gainCount++
		} else if exposureAmount < 0 {
			lossCount++
		} else {
			neutralCount++
		}
		if quote.Stale {
			staleRateCount++
		}

		totalExposure += exposureAmount

		currency := strings.ToUpper(strings.TrimSpace(transaction.PresentmentCurrency))
		group, exists := breakdown[currency]
		if !exists {
			group = &CurrencyExposureBreakdown{PresentmentCurrency: currency}
			breakdown[currency] = group
		}
		group.TotalExposureAmount += exposureAmount
		group.TransactionCount++
		if exposureAmount > 0 {
			group.GainCount++
		}
		if exposureAmount < 0 {
			group.LossCount++
		}

		exposureItem := TransactionExposure{
			TransactionID:            transaction.TransactionID,
			AuthorizationTimestamp:   transaction.AuthorizationTimestamp,
			PresentmentCurrency:      currency,
			SettlementCurrency:       strings.ToUpper(strings.TrimSpace(transaction.SettlementCurrency)),
			AuthorizedAmount:         transaction.AuthorizedAmount,
			OriginalSettlementAmount: round(originalSettlement),
			CurrentSettlementAmount:  round(currentSettlement),
			ExposureAmount:           round(exposureAmount),
			ExposurePercentage:       round(exposurePercentage),
			AuthorizationRate:        transaction.AuthorizationRate,
			CurrentRate:              quote.Rate,
			CurrentRateTimestamp:     quote.Timestamp,
			CurrentRateSource:        quote.Source,
			IsStaleRate:              quote.Stale,
			IsHighRisk:               isHighRisk,
			Recommendation:           recommendation,
			Metadata:                 transaction.Metadata,
		}
		exposures = append(exposures, exposureItem)
	}

	sortedExposures := make([]TransactionExposure, len(exposures))
	copy(sortedExposures, exposures)
	sort.Slice(sortedExposures, func(i, j int) bool {
		if sortedExposures[i].ExposureAmount == sortedExposures[j].ExposureAmount {
			return sortedExposures[i].TransactionID < sortedExposures[j].TransactionID
		}
		return sortedExposures[i].ExposureAmount < sortedExposures[j].ExposureAmount
	})

	currencyBreakdown := make([]CurrencyExposureBreakdown, 0, len(breakdown))
	for _, item := range breakdown {
		item.TotalExposureAmount = round(item.TotalExposureAmount)
		currencyBreakdown = append(currencyBreakdown, *item)
	}
	sort.Slice(currencyBreakdown, func(i, j int) bool {
		return currencyBreakdown[i].PresentmentCurrency < currencyBreakdown[j].PresentmentCurrency
	})

	ranking := make([]RiskRankingItem, 0, len(sortedExposures))
	for index, item := range sortedExposures {
		ranking = append(ranking, RiskRankingItem{
			Rank:                index + 1,
			TransactionID:       item.TransactionID,
			PresentmentCurrency: item.PresentmentCurrency,
			SettlementCurrency:  item.SettlementCurrency,
			ExposureAmount:      item.ExposureAmount,
			ExposurePercentage:  item.ExposurePercentage,
			IsHighRisk:          item.IsHighRisk,
			Recommendation:      item.Recommendation,
		})
	}

	response := CalculateExposureResponse{
		Summary: ExposureSummary{
			GeneratedAt:               s.clock.Now(),
			TotalExposureAmount:       round(totalExposure),
			GainCount:                 gainCount,
			LossCount:                 lossCount,
			NeutralCount:              neutralCount,
			HighRiskCount:             highRiskCount,
			StaleRateTransactionCount: staleRateCount,
			CurrencyBreakdown:         currencyBreakdown,
		},
		Transactions: sortedExposures,
		Ranking:      ranking,
	}

	if len(sortedExposures) > 0 {
		worst := sortedExposures[0]
		best := sortedExposures[len(sortedExposures)-1]
		response.Summary.WorstTransaction = &worst
		response.Summary.BestTransaction = &best
	}

	return response, nil
}

func validateTransaction(transaction PendingTransaction) error {
	transactionID := strings.TrimSpace(transaction.TransactionID)
	if transactionID == "" {
		return fmt.Errorf("%w: transaction_id is required", ErrValidation)
	}
	if transaction.AuthorizationTimestamp.IsZero() {
		return fmt.Errorf("%w: authorization_timestamp is required for transaction %s", ErrValidation, transactionID)
	}
	if transaction.AuthorizedAmount <= 0 {
		return fmt.Errorf("%w: authorized_amount must be > 0 for transaction %s", ErrValidation, transactionID)
	}
	if strings.TrimSpace(transaction.PresentmentCurrency) == "" {
		return fmt.Errorf("%w: presentment_currency is required for transaction %s", ErrValidation, transactionID)
	}
	if strings.TrimSpace(transaction.SettlementCurrency) == "" {
		return fmt.Errorf("%w: settlement_currency is required for transaction %s", ErrValidation, transactionID)
	}
	if transaction.AuthorizationRate <= 0 {
		return fmt.Errorf("%w: authorization_rate must be > 0 for transaction %s", ErrValidation, transactionID)
	}
	return nil
}

func round(value float64) float64 {
	return math.Round(value*10000) / 10000
}
