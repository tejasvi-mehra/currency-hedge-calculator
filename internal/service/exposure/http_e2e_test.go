package exposure

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	frameworkserver "github.com/tejasvi-mehra/currency-hedge-calculator/internal/framework/server"
	"github.com/tejasvi-mehra/currency-hedge-calculator/internal/service/rates"
)

type e2eRatesProvider struct {
	live       map[string]rates.Quote
	historical map[string]rates.Quote
}

func (p *e2eRatesProvider) GetPresentmentPerSettlementRate(_ context.Context, settlementCurrency string, presentmentCurrency string) (rates.Quote, error) {
	return p.live[settlementCurrency+":"+presentmentCurrency], nil
}

func (p *e2eRatesProvider) GetHistoricalPresentmentPerSettlementRate(_ context.Context, settlementCurrency string, presentmentCurrency string, _ time.Time) (rates.Quote, error) {
	return p.historical[settlementCurrency+":"+presentmentCurrency], nil
}

func TestCalculateEndpointE2E_WithAuthAndIdempotency(t *testing.T) {
	service := NewService(
		&e2eRatesProvider{
			live: map[string]rates.Quote{
				"EUR:BRL": {SettlementCurrency: "EUR", PresentmentCurrency: "BRL", Rate: 6.3, Timestamp: time.Now().UTC(), Source: "test-live"},
				"USD:MXN": {SettlementCurrency: "USD", PresentmentCurrency: "MXN", Rate: 17.4, Timestamp: time.Now().UTC(), Source: "test-live"},
			},
			historical: map[string]rates.Quote{
				"EUR:BRL": {SettlementCurrency: "EUR", PresentmentCurrency: "BRL", Rate: 6.0, Timestamp: time.Now().UTC(), Source: "test-historical"},
				"USD:MXN": {SettlementCurrency: "USD", PresentmentCurrency: "MXN", Rate: 18.0, Timestamp: time.Now().UTC(), Source: "test-historical"},
			},
		},
		nil,
		2,
		[]string{"EUR", "USD", "BRL", "MXN"},
		500,
		10*time.Minute,
		0,
		0,
		nil,
		nil,
	)
	handler := NewHandler(service, nil)
	server := frameworkserver.New(":0", nil)
	server.UseAPIKeyAuth("test-key")
	server.UseIdempotency(24*time.Hour, nil)
	handler.Register(server, "/healthz")

	testServer := httptest.NewServer(server.HTTPHandler())
	defer testServer.Close()

	requestPayload := `{"transactions":[{"account_id":"acc_1","payment_id":"pay_1","transaction_id":"txn_1","merchant_order_id":"mo_1","country":"BR","provider":"provider_a","payment_method_type":"card","payment_status":"authorized","transaction_status":"authorized","authorized_at":"2026-06-20T10:30:00Z","authorization_expires_at":"2026-07-03T10:30:00Z","authorized_amount":14400,"capture_amount":14400,"presentment_currency":"BRL","settlement_currency":"EUR","authorization_rate":6.0}]}`

	req, err := http.NewRequest(http.MethodPost, testServer.URL+"/v1/exposure/calculate", bytes.NewBufferString(requestPayload))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", "test-key")
	req.Header.Set("X-Idempotency-Key", "7bf41af5-70ae-4e79-9b28-a8fa75c3ac53")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("send request: %v", err)
	}
	defer func() { _ = res.Body.Close() }()

	if res.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", res.StatusCode)
	}

	response := CalculateExposureResponse{}
	if err := json.NewDecoder(res.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(response.Transactions) != 1 {
		t.Fatalf("expected 1 transaction, got %d", len(response.Transactions))
	}
	if response.Transactions[0].NextAction == "" {
		t.Fatalf("expected next_action")
	}
}

func TestCalculateEndpoint_AuthError(t *testing.T) {
	service := NewService(&e2eRatesProvider{}, nil, 2, []string{"USD"}, 10, 10*time.Minute, 0, 0, nil, nil)
	handler := NewHandler(service, nil)
	server := frameworkserver.New(":0", nil)
	server.UseAPIKeyAuth("test-key")
	server.UseIdempotency(24*time.Hour, nil)
	handler.Register(server, "/healthz")

	testServer := httptest.NewServer(server.HTTPHandler())
	defer testServer.Close()

	req, _ := http.NewRequest(http.MethodPost, testServer.URL+"/v1/exposure/calculate", bytes.NewBufferString(`{"transactions":[]}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Idempotency-Key", "7bf41af5-70ae-4e79-9b28-a8fa75c3ac53")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("send request: %v", err)
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", res.StatusCode)
	}
}

func TestCalculateEndpoint_UnsupportedCurrency(t *testing.T) {
	service := NewService(
		&e2eRatesProvider{
			live: map[string]rates.Quote{"USD:USD": {SettlementCurrency: "USD", PresentmentCurrency: "USD", Rate: 1, Timestamp: time.Now().UTC(), Source: "identity"}},
		},
		nil,
		2,
		[]string{"USD", "EUR"},
		500,
		10*time.Minute,
		0,
		0,
		nil,
		nil,
	)
	handler := NewHandler(service, nil)
	server := frameworkserver.New(":0", nil)
	server.UseAPIKeyAuth("test-key")
	server.UseIdempotency(24*time.Hour, nil)
	handler.Register(server, "/healthz")
	testServer := httptest.NewServer(server.HTTPHandler())
	defer testServer.Close()

	payload := `{"transactions":[{"account_id":"acc_1","payment_id":"pay_1","transaction_id":"txn_1","merchant_order_id":"mo_1","country":"BR","provider":"provider_a","payment_method_type":"card","payment_status":"authorized","transaction_status":"authorized","authorized_at":"2026-06-20T10:30:00Z","authorization_expires_at":"2026-07-03T10:30:00Z","authorized_amount":14400,"capture_amount":14400,"presentment_currency":"ZZZ","settlement_currency":"USD","authorization_rate":1.0}]}`
	req, _ := http.NewRequest(http.MethodPost, testServer.URL+"/v1/exposure/calculate", bytes.NewBufferString(payload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", "test-key")
	req.Header.Set("X-Idempotency-Key", "7bf41af5-70ae-4e79-9b28-a8fa75c3ac53")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("send request: %v", err)
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", res.StatusCode)
	}
}

func TestOpenAPIExamplesContract(t *testing.T) {
	requestBytes, err := os.ReadFile("../../../docs/example-request.json")
	if err != nil {
		t.Fatalf("read request example: %v", err)
	}
	var request CalculateExposureRequest
	if err := json.Unmarshal(requestBytes, &request); err != nil {
		t.Fatalf("unmarshal request example: %v", err)
	}
	if len(request.Transactions) == 0 {
		t.Fatalf("example request has no transactions")
	}

	responseBytes, err := os.ReadFile("../../../docs/example-response.json")
	if err != nil {
		t.Fatalf("read response example: %v", err)
	}
	var response CalculateExposureResponse
	if err := json.Unmarshal(responseBytes, &response); err != nil {
		t.Fatalf("unmarshal response example: %v", err)
	}
	if len(response.Ranking) == 0 {
		t.Fatalf("example response has no ranking entries")
	}
}
