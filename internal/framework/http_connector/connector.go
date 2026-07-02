package http_connector

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"go.uber.org/zap"
)

// HTTPError captures non-2xx responses from upstream HTTP dependencies.
type HTTPError struct {
	StatusCode int
	Body       string
}

// Error returns an HTTP error summary string.
func (e *HTTPError) Error() string {
	return fmt.Sprintf("upstream returned status %d", e.StatusCode)
}

// Connector wraps HTTP operations used by external service adapters.
type Connector struct {
	client *http.Client
	logger *zap.SugaredLogger
}

// New builds an HTTP connector with timeout and structured logging support.
func New(timeout time.Duration, logger *zap.SugaredLogger) *Connector {
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	if logger == nil {
		logger = zap.NewNop().Sugar()
	}
	return &Connector{
		client: &http.Client{Timeout: timeout},
		logger: logger,
	}
}

// GetJSON issues a GET request and decodes JSON response to target.
func (c *Connector) GetJSON(ctx context.Context, endpoint string, headers map[string]string, target any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	for name, value := range headers {
		if strings.TrimSpace(name) == "" {
			continue
		}
		req.Header.Set(name, value)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("send request: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read response body: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return &HTTPError{
			StatusCode: resp.StatusCode,
			Body:       truncate(string(body), 2048),
		}
	}

	if err := json.Unmarshal(body, target); err != nil {
		return fmt.Errorf("decode response json: %w", err)
	}
	return nil
}

func truncate(value string, max int) string {
	if max <= 0 || len(value) <= max {
		return value
	}
	return value[:max]
}
