package exposure

import "time"

// PendingTransaction models an authorized but not captured payment.
type PendingTransaction struct {
	AccountID               string            `json:"account_id,omitempty"`
	PaymentID               string            `json:"payment_id,omitempty"`
	TransactionID           string            `json:"transaction_id"`
	MerchantOrderID         string            `json:"merchant_order_id,omitempty"`
	Country                 string            `json:"country,omitempty"`
	Provider                string            `json:"provider,omitempty"`
	PaymentMethodType       string            `json:"payment_method_type,omitempty"`
	PaymentStatus           string            `json:"payment_status,omitempty"`
	TransactionStatus       string            `json:"transaction_status,omitempty"`
	AuthorizationTimestamp  time.Time         `json:"authorization_timestamp,omitempty"` // Backward compatible alias.
	AuthorizedAt            time.Time         `json:"authorized_at,omitempty"`
	AuthorizationExpiresAt  time.Time         `json:"authorization_expires_at,omitempty"`
	AuthorizedAmount        float64           `json:"authorized_amount"`
	CaptureAmount           float64           `json:"capture_amount,omitempty"`
	PresentmentCurrency     string            `json:"presentment_currency"`
	SettlementCurrency      string            `json:"settlement_currency"`
	AuthorizationRate       float64           `json:"authorization_rate"`
	AuthorizationRateSource string            `json:"authorization_rate_source,omitempty"`
	Metadata                map[string]string `json:"metadata,omitempty"`
}

// CalculateExposureRequest is the API payload for exposure calculations.
type CalculateExposureRequest struct {
	Transactions                     []PendingTransaction `json:"transactions"`
	RiskThresholdPercentage          *float64             `json:"risk_threshold_percentage,omitempty"`
	UseDefaultTestDataWhenEmptyInput bool                 `json:"use_default_test_data_when_empty_input,omitempty"`
}

// CalculateExposureResponse contains per-transaction and aggregate exposure output.
type CalculateExposureResponse struct {
	Summary      ExposureSummary       `json:"summary"`
	Transactions []TransactionExposure `json:"transactions"`
	Ranking      []RiskRankingItem     `json:"ranking"`
}

// ExposureSummary aggregates risk metrics over all transactions.
type ExposureSummary struct {
	GeneratedAt               time.Time                   `json:"generated_at"`
	TotalExposureAmount       float64                     `json:"total_exposure_amount"`
	GainCount                 int                         `json:"gain_count"`
	LossCount                 int                         `json:"loss_count"`
	NeutralCount              int                         `json:"neutral_count"`
	HighRiskCount             int                         `json:"high_risk_count"`
	HighRiskExposureTotal     float64                     `json:"high_risk_exposure_total"`
	StaleRateTransactionCount int                         `json:"stale_rate_transaction_count"`
	CurrencyBreakdown         []CurrencyExposureBreakdown `json:"currency_breakdown"`
	RiskyCurrencyPairs        []CurrencyPairRiskInsight   `json:"risky_currency_pairs"`
	BestTransaction           *TransactionExposure        `json:"best_transaction,omitempty"`
	WorstTransaction          *TransactionExposure        `json:"worst_transaction,omitempty"`
}

// CurrencyPairRiskInsight summarizes risk for a presentment/settlement pair.
type CurrencyPairRiskInsight struct {
	PresentmentCurrency    string  `json:"presentment_currency"`
	SettlementCurrency     string  `json:"settlement_currency"`
	TransactionCount       int     `json:"transaction_count"`
	HighRiskCount          int     `json:"high_risk_count"`
	TotalExposureAmount    float64 `json:"total_exposure_amount"`
	AverageExposurePercent float64 `json:"average_exposure_percentage"`
	Trend                  string  `json:"trend"`
}

// CurrencyExposureBreakdown summarizes exposure grouped by presentment currency.
type CurrencyExposureBreakdown struct {
	PresentmentCurrency string  `json:"presentment_currency"`
	TotalExposureAmount float64 `json:"total_exposure_amount"`
	TransactionCount    int     `json:"transaction_count"`
	GainCount           int     `json:"gain_count"`
	LossCount           int     `json:"loss_count"`
}

// TransactionExposure describes exposure metrics for one transaction.
type TransactionExposure struct {
	AccountID                string            `json:"account_id,omitempty"`
	PaymentID                string            `json:"payment_id,omitempty"`
	MerchantOrderID          string            `json:"merchant_order_id,omitempty"`
	TransactionID            string            `json:"transaction_id"`
	AuthorizationTimestamp   time.Time         `json:"authorization_timestamp"`
	AuthorizationExpiresAt   time.Time         `json:"authorization_expires_at,omitempty"`
	PresentmentCurrency      string            `json:"presentment_currency"`
	SettlementCurrency       string            `json:"settlement_currency"`
	AuthorizedAmount         float64           `json:"authorized_amount"`
	CaptureAmount            float64           `json:"capture_amount"`
	OriginalSettlementAmount float64           `json:"original_settlement_amount"`
	CurrentSettlementAmount  float64           `json:"current_settlement_amount"`
	ExposureAmount           float64           `json:"exposure_amount"`
	ExposurePercentage       float64           `json:"exposure_percentage"`
	AuthorizationRate        float64           `json:"authorization_rate"`
	AuthorizationRateSource  string            `json:"authorization_rate_source,omitempty"`
	CurrentRate              float64           `json:"current_rate"`
	CurrentRateTimestamp     time.Time         `json:"current_rate_timestamp"`
	QuoteAgeSeconds          float64           `json:"quote_age_seconds"`
	CurrentRateSource        string            `json:"current_rate_source"`
	IsStaleRate              bool              `json:"is_stale_rate"`
	IsHighRisk               bool              `json:"is_high_risk"`
	EligibleToCapture        bool              `json:"eligible_to_capture"`
	CaptureEndpointHint      string            `json:"capture_endpoint_hint"`
	CaptureType              string            `json:"capture_type"`
	PartialCaptureSupported  bool              `json:"partial_capture_supported"`
	AuthorizationExpiryRisk  string            `json:"authorization_expiry_risk"`
	BlockingReason           string            `json:"blocking_reason,omitempty"`
	ExpectedLossAvoided      float64           `json:"expected_loss_avoided"`
	UrgencyScore             float64           `json:"urgency_score"`
	ExpiryScore              float64           `json:"expiry_score"`
	FXSeverityScore          float64           `json:"fx_severity_score"`
	NextAction               string            `json:"next_action"`
	Recommendation           string            `json:"recommendation"` // Backward compatible alias.
	Metadata                 map[string]string `json:"metadata,omitempty"`
}

// RiskRankingItem is a concise ranking projection for immediate actioning.
type RiskRankingItem struct {
	Rank                int     `json:"rank"`
	TransactionID       string  `json:"transaction_id"`
	PresentmentCurrency string  `json:"presentment_currency"`
	SettlementCurrency  string  `json:"settlement_currency"`
	ExposureAmount      float64 `json:"exposure_amount"`
	ExposurePercentage  float64 `json:"exposure_percentage"`
	IsHighRisk          bool    `json:"is_high_risk"`
	Recommendation      string  `json:"recommendation"`
}

// ErrorResponse is the shared API error envelope.
type ErrorResponse struct {
	Type    string `json:"type"`
	Code    string `json:"code"`
	Message string `json:"message"`
	Details any    `json:"details,omitempty"`
}
