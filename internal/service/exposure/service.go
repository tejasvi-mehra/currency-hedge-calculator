package exposure

import (
	"context"
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/tejasvi-mehra/currency-hedge-calculator/internal/framework/metrics"
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
	supportedCurrencySet map[string]struct{}
	maxTransactions      int
	quoteFreshnessSLA    time.Duration
	settlementSpreadBPS  float64
	providerMarkupBPS    float64
	clock                Clock
	logger               *zap.SugaredLogger
	metrics              *metrics.Collector
}

type historicalRateProvider interface {
	GetHistoricalPresentmentPerSettlementRate(ctx context.Context, settlementCurrency string, presentmentCurrency string, at time.Time) (rates.Quote, error)
}

type pairAccumulator struct {
	PresentmentCurrency string
	SettlementCurrency  string
	TransactionCount    int
	HighRiskCount       int
	TotalExposureAmount float64
	ExposurePercentSum  float64
}

// NewService creates a new exposure service.
func NewService(
	ratesProvider rates.Provider,
	transactionSource TransactionSource,
	defaultRiskThreshold float64,
	supportedCurrencies []string,
	maxTransactions int,
	quoteFreshnessSLA time.Duration,
	settlementSpreadBPS float64,
	providerMarkupBPS float64,
	collector *metrics.Collector,
	logger *zap.SugaredLogger,
) *Service {
	if logger == nil {
		logger = zap.NewNop().Sugar()
	}
	if defaultRiskThreshold < 0 {
		defaultRiskThreshold = 0
	}
	if maxTransactions <= 0 {
		maxTransactions = 500
	}
	if quoteFreshnessSLA <= 0 {
		quoteFreshnessSLA = 10 * time.Minute
	}

	supportedCurrencySet := map[string]struct{}{}
	for _, code := range supportedCurrencies {
		normalized := strings.ToUpper(strings.TrimSpace(code))
		if normalized != "" {
			supportedCurrencySet[normalized] = struct{}{}
		}
	}

	return &Service{
		ratesProvider:        ratesProvider,
		transactionSource:    transactionSource,
		defaultRiskThreshold: defaultRiskThreshold,
		supportedCurrencySet: supportedCurrencySet,
		maxTransactions:      maxTransactions,
		quoteFreshnessSLA:    quoteFreshnessSLA,
		settlementSpreadBPS:  settlementSpreadBPS,
		providerMarkupBPS:    providerMarkupBPS,
		clock:                systemClock{},
		logger:               logger,
		metrics:              collector,
	}
}

// CalculateExposure computes transaction-level and aggregate exposure metrics.
func (s *Service) CalculateExposure(ctx context.Context, request CalculateExposureRequest) (CalculateExposureResponse, error) {
	transactions := request.Transactions
	if len(transactions) == 0 && request.UseDefaultTestDataWhenEmptyInput {
		if s.transactionSource == nil {
			return CalculateExposureResponse{}, fmt.Errorf("%w: default test data unavailable", ErrNoTransactions)
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
	if len(transactions) > s.maxTransactions {
		return CalculateExposureResponse{}, fmt.Errorf("%w: max %d transactions per request", ErrValidation, s.maxTransactions)
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
	pairBreakdown := map[string]*pairAccumulator{}

	var totalExposure float64
	var gainCount int
	var lossCount int
	var neutralCount int
	var highRiskCount int
	var highRiskExposureTotal float64
	var staleRateCount int
	now := s.clock.Now()

	for _, transaction := range transactions {
		if err := validateTransaction(transaction, s.supportedCurrencySet); err != nil {
			return CalculateExposureResponse{}, err
		}
		authorizedAt := resolveAuthorizedAt(transaction)
		authorizationExpiresAt := resolveAuthorizationExpiresAt(transaction, authorizedAt)
		captureAmount := transaction.CaptureAmount
		if captureAmount <= 0 {
			captureAmount = transaction.AuthorizedAmount
		}

		authorizationRate := transaction.AuthorizationRate
		authorizationRateSource := strings.TrimSpace(transaction.AuthorizationRateSource)
		if authorizationRate <= 0 {
			historical, ok := s.ratesProvider.(historicalRateProvider)
			if !ok {
				return CalculateExposureResponse{}, fmt.Errorf("%w: authorization_rate is required when historical provider is unavailable for transaction %s", ErrValidation, transaction.TransactionID)
			}
			historicalQuote, err := historical.GetHistoricalPresentmentPerSettlementRate(
				ctx,
				transaction.SettlementCurrency,
				transaction.PresentmentCurrency,
				authorizedAt,
			)
			if err != nil {
				return CalculateExposureResponse{}, fmt.Errorf("resolve authorization rate for transaction %s: %w", transaction.TransactionID, err)
			}
			authorizationRate = historicalQuote.Rate
			if authorizationRateSource == "" {
				authorizationRateSource = historicalQuote.Source
			}
		}
		if authorizationRateSource == "" {
			authorizationRateSource = "request_payload"
		}

		quote, err := s.ratesProvider.GetPresentmentPerSettlementRate(ctx, transaction.SettlementCurrency, transaction.PresentmentCurrency)
		if err != nil {
			return CalculateExposureResponse{}, fmt.Errorf("fetch current rate for transaction %s: %w", transaction.TransactionID, err)
		}

		effectiveCurrentRate := s.applyCurrentRateAssumptions(quote.Rate)
		originalSettlement := captureAmount / authorizationRate
		currentSettlement := captureAmount / effectiveCurrentRate
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
			if exposureAmount < 0 {
				highRiskExposureTotal += (-1 * exposureAmount)
			}
		}

		if exposureAmount > 0 {
			gainCount++
		} else if exposureAmount < 0 {
			lossCount++
		} else {
			neutralCount++
		}
		quoteAgeSeconds := now.Sub(quote.Timestamp).Seconds()
		if quoteAgeSeconds < 0 {
			quoteAgeSeconds = 0
		}
		isStaleQuote := quote.Stale || time.Duration(quoteAgeSeconds*float64(time.Second)) > s.quoteFreshnessSLA
		if isStaleQuote {
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

		pairKey := strings.ToUpper(strings.TrimSpace(transaction.PresentmentCurrency)) + ":" + strings.ToUpper(strings.TrimSpace(transaction.SettlementCurrency))
		pairGroup, exists := pairBreakdown[pairKey]
		if !exists {
			pairGroup = &pairAccumulator{
				PresentmentCurrency: strings.ToUpper(strings.TrimSpace(transaction.PresentmentCurrency)),
				SettlementCurrency:  strings.ToUpper(strings.TrimSpace(transaction.SettlementCurrency)),
			}
			pairBreakdown[pairKey] = pairGroup
		}
		pairGroup.TransactionCount++
		pairGroup.TotalExposureAmount += exposureAmount
		pairGroup.ExposurePercentSum += exposurePercentage
		if isHighRisk {
			pairGroup.HighRiskCount++
		}

		expiryRisk, expiryScore := calculateExpiryRisk(now, authorizationExpiresAt)
		fxSeverityScore := calculateFXSeverityScore(exposurePercentage)
		urgencyScore := calculateUrgencyScore(fxSeverityScore, expiryScore, exposureAmount)
		nextAction := recommendNextAction(isHighRisk, urgencyScore, expiryRisk)
		eligibleToCapture, blockingReason := captureEligibility(transaction, authorizationExpiresAt, now)
		captureType := "full"
		if captureAmount < transaction.AuthorizedAmount {
			captureType = "partial"
		}
		partialSupported := supportsPartialCapture(transaction.PaymentMethodType)
		captureEndpointHint := captureHint(transaction)

		exposureItem := TransactionExposure{
			AccountID:                transaction.AccountID,
			PaymentID:                transaction.PaymentID,
			MerchantOrderID:          transaction.MerchantOrderID,
			TransactionID:            transaction.TransactionID,
			AuthorizationTimestamp:   authorizedAt,
			AuthorizationExpiresAt:   authorizationExpiresAt,
			PresentmentCurrency:      currency,
			SettlementCurrency:       strings.ToUpper(strings.TrimSpace(transaction.SettlementCurrency)),
			AuthorizedAmount:         transaction.AuthorizedAmount,
			CaptureAmount:            captureAmount,
			OriginalSettlementAmount: round(originalSettlement),
			CurrentSettlementAmount:  round(currentSettlement),
			ExposureAmount:           round(exposureAmount),
			ExposurePercentage:       round(exposurePercentage),
			AuthorizationRate:        authorizationRate,
			AuthorizationRateSource:  authorizationRateSource,
			CurrentRate:              round(effectiveCurrentRate),
			CurrentRateTimestamp:     quote.Timestamp,
			QuoteAgeSeconds:          round(quoteAgeSeconds),
			CurrentRateSource:        quote.Source,
			IsStaleRate:              isStaleQuote,
			IsHighRisk:               isHighRisk,
			EligibleToCapture:        eligibleToCapture,
			CaptureEndpointHint:      captureEndpointHint,
			CaptureType:              captureType,
			PartialCaptureSupported:  partialSupported,
			AuthorizationExpiryRisk:  expiryRisk,
			BlockingReason:           blockingReason,
			ExpectedLossAvoided:      round(math.Max(0, -1*exposureAmount)),
			UrgencyScore:             round(urgencyScore),
			ExpiryScore:              round(expiryScore),
			FXSeverityScore:          round(fxSeverityScore),
			NextAction:               nextAction,
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

	riskyPairs := make([]CurrencyPairRiskInsight, 0, len(pairBreakdown))
	for _, pair := range pairBreakdown {
		averageExposurePct := pair.ExposurePercentSum / float64(pair.TransactionCount)
		trend := "stable_or_favorable"
		if pair.HighRiskCount > 0 || averageExposurePct <= (-1*threshold) {
			trend = "dangerous"
		}
		riskyPairs = append(riskyPairs, CurrencyPairRiskInsight{
			PresentmentCurrency:    pair.PresentmentCurrency,
			SettlementCurrency:     pair.SettlementCurrency,
			TransactionCount:       pair.TransactionCount,
			HighRiskCount:          pair.HighRiskCount,
			TotalExposureAmount:    round(pair.TotalExposureAmount),
			AverageExposurePercent: round(averageExposurePct),
			Trend:                  trend,
		})
	}
	sort.Slice(riskyPairs, func(i, j int) bool {
		if riskyPairs[i].HighRiskCount == riskyPairs[j].HighRiskCount {
			return riskyPairs[i].AverageExposurePercent < riskyPairs[j].AverageExposurePercent
		}
		return riskyPairs[i].HighRiskCount > riskyPairs[j].HighRiskCount
	})

	response := CalculateExposureResponse{
		Summary: ExposureSummary{
			GeneratedAt:               s.clock.Now(),
			TotalExposureAmount:       round(totalExposure),
			GainCount:                 gainCount,
			LossCount:                 lossCount,
			NeutralCount:              neutralCount,
			HighRiskCount:             highRiskCount,
			HighRiskExposureTotal:     round(highRiskExposureTotal),
			StaleRateTransactionCount: staleRateCount,
			CurrencyBreakdown:         currencyBreakdown,
			RiskyCurrencyPairs:        riskyPairs,
		},
		Transactions: sortedExposures,
		Ranking:      ranking,
	}
	if s.metrics != nil {
		s.metrics.AddHighRiskExposure(round(highRiskExposureTotal))
	}

	if len(sortedExposures) > 0 {
		worst := sortedExposures[0]
		best := sortedExposures[len(sortedExposures)-1]
		response.Summary.WorstTransaction = &worst
		response.Summary.BestTransaction = &best
	}

	return response, nil
}

func validateTransaction(transaction PendingTransaction, supportedSet map[string]struct{}) error {
	transactionID := strings.TrimSpace(transaction.TransactionID)
	if transactionID == "" {
		return fmt.Errorf("%w: transaction_id is required", ErrValidation)
	}
	if resolveAuthorizedAt(transaction).IsZero() {
		return fmt.Errorf("%w: authorized_at is required for transaction %s", ErrValidation, transactionID)
	}
	if transaction.AuthorizedAmount <= 0 {
		return fmt.Errorf("%w: authorized_amount must be > 0 for transaction %s", ErrValidation, transactionID)
	}
	if transaction.CaptureAmount < 0 {
		return fmt.Errorf("%w: capture_amount must be >= 0 for transaction %s", ErrValidation, transactionID)
	}
	if transaction.CaptureAmount > transaction.AuthorizedAmount {
		return fmt.Errorf("%w: capture_amount cannot exceed authorized_amount for transaction %s", ErrValidation, transactionID)
	}
	if strings.TrimSpace(transaction.PresentmentCurrency) == "" {
		return fmt.Errorf("%w: presentment_currency is required for transaction %s", ErrValidation, transactionID)
	}
	if strings.TrimSpace(transaction.SettlementCurrency) == "" {
		return fmt.Errorf("%w: settlement_currency is required for transaction %s", ErrValidation, transactionID)
	}
	if len(supportedSet) > 0 {
		if _, ok := supportedSet[strings.ToUpper(strings.TrimSpace(transaction.PresentmentCurrency))]; !ok {
			return fmt.Errorf("%w: unsupported presentment currency for transaction %s", ErrValidation, transactionID)
		}
		if _, ok := supportedSet[strings.ToUpper(strings.TrimSpace(transaction.SettlementCurrency))]; !ok {
			return fmt.Errorf("%w: unsupported settlement currency for transaction %s", ErrValidation, transactionID)
		}
	}
	return nil
}

func resolveAuthorizedAt(transaction PendingTransaction) time.Time {
	if !transaction.AuthorizedAt.IsZero() {
		return transaction.AuthorizedAt.UTC()
	}
	return transaction.AuthorizationTimestamp.UTC()
}

func resolveAuthorizationExpiresAt(transaction PendingTransaction, authorizedAt time.Time) time.Time {
	if !transaction.AuthorizationExpiresAt.IsZero() {
		return transaction.AuthorizationExpiresAt.UTC()
	}
	if authorizedAt.IsZero() {
		return time.Time{}
	}
	return authorizedAt.Add(7 * 24 * time.Hour).UTC()
}

func calculateExpiryRisk(now time.Time, expiresAt time.Time) (string, float64) {
	if expiresAt.IsZero() {
		return "unknown", 35
	}
	remaining := expiresAt.Sub(now)
	switch {
	case remaining <= 0:
		return "expired", 100
	case remaining <= 24*time.Hour:
		return "high", 90
	case remaining <= 72*time.Hour:
		return "medium", 65
	default:
		return "low", 30
	}
}

func calculateFXSeverityScore(exposurePct float64) float64 {
	if exposurePct >= 0 {
		return 5
	}
	score := math.Min(100, math.Abs(exposurePct)*8)
	if score < 5 {
		score = 5
	}
	return score
}

func calculateUrgencyScore(fxSeverityScore float64, expiryScore float64, exposureAmount float64) float64 {
	weightFX := 0.65
	if exposureAmount >= 0 {
		weightFX = 0.35
	}
	return (fxSeverityScore * weightFX) + (expiryScore * (1 - weightFX))
}

func recommendNextAction(isHighRisk bool, urgencyScore float64, expiryRisk string) string {
	if expiryRisk == "expired" {
		return "authorization_expired_reauthorize_required"
	}
	if isHighRisk && urgencyScore >= 75 {
		return "capture_immediately"
	}
	if isHighRisk {
		return "prioritize_capture_today"
	}
	if urgencyScore >= 70 {
		return "schedule_capture_soon"
	}
	return "monitor_and_capture_per_schedule"
}

func captureEligibility(transaction PendingTransaction, expiresAt time.Time, now time.Time) (bool, string) {
	if !expiresAt.IsZero() && !expiresAt.After(now) {
		return false, "authorization_expired"
	}
	paymentStatus := strings.ToLower(strings.TrimSpace(transaction.PaymentStatus))
	switch paymentStatus {
	case "captured", "cancelled", "refunded", "failed":
		return false, "payment_status_not_capturable"
	}
	transactionStatus := strings.ToLower(strings.TrimSpace(transaction.TransactionStatus))
	if transactionStatus != "" && transactionStatus != "authorized" && transactionStatus != "pending" {
		return false, "transaction_status_not_capturable"
	}
	return true, ""
}

func captureHint(transaction PendingTransaction) string {
	if strings.TrimSpace(transaction.PaymentID) != "" {
		return "/v1/payments/" + strings.TrimSpace(transaction.PaymentID) + "/capture"
	}
	return "/v1/payments/{payment_id}/capture"
}

func supportsPartialCapture(paymentMethodType string) bool {
	switch strings.ToLower(strings.TrimSpace(paymentMethodType)) {
	case "card", "credit_card":
		return true
	default:
		return false
	}
}

func (s *Service) applyCurrentRateAssumptions(rawRate float64) float64 {
	bps := s.settlementSpreadBPS + s.providerMarkupBPS
	if bps <= 0 {
		return rawRate
	}
	return rawRate * (1 + (bps / 10000))
}

func round(value float64) float64 {
	return math.Round(value*10000) / 10000
}
