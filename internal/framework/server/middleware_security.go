package server

import (
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/tejasvi-mehra/currency-hedge-calculator/internal/framework/metrics"
	"golang.org/x/time/rate"
)

type rateLimiterEntry struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

// UseBodyLimit enforces a maximum request body size.
func (s *Server) UseBodyLimit(maxBytes int64) {
	if maxBytes <= 0 {
		return
	}
	s.echo.Use(middleware.BodyLimit(strings.TrimSpace(int64ToString(maxBytes))))
}

// UseRequestTimeout enforces an end-to-end request budget.
func (s *Server) UseRequestTimeout(timeout time.Duration) {
	if timeout <= 0 {
		return
	}
	s.echo.Use(middleware.TimeoutWithConfig(middleware.TimeoutConfig{
		Timeout:      timeout,
		ErrorMessage: `{"type":"timeout_error","code":"REQUEST_TIMEOUT","message":"request timed out"}`,
	}))
}

// UseAPIKeyAuth protects API routes with X-API-Key.
func (s *Server) UseAPIKeyAuth(apiKey string) {
	key := strings.TrimSpace(apiKey)
	if key == "" {
		return
	}
	s.echo.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			if c.Path() == "/healthz" {
				return next(c)
			}
			if c.Path() == "/metrics" {
				return next(c)
			}
			if strings.TrimSpace(c.Request().Header.Get("X-API-Key")) != key {
				return c.JSON(http.StatusUnauthorized, map[string]any{
					"type":    "auth_error",
					"code":    "UNAUTHORIZED",
					"message": "invalid API key",
				})
			}
			return next(c)
		}
	})
}

// UseRateLimiting enforces per-client request budgets.
func (s *Server) UseRateLimiting(maxRequests int, window time.Duration) {
	if maxRequests <= 0 || window <= 0 {
		return
	}
	entries := map[string]*rateLimiterEntry{}
	var mu sync.Mutex

	limit := rate.Every(window / time.Duration(maxRequests))
	if limit <= 0 {
		limit = rate.Every(time.Second)
	}
	burst := maxRequests

	s.echo.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			clientKey := c.RealIP()
			if clientKey == "" {
				clientKey = "unknown"
			}

			now := time.Now().UTC()
			mu.Lock()
			entry, ok := entries[clientKey]
			if !ok {
				entry = &rateLimiterEntry{
					limiter:  rate.NewLimiter(limit, burst),
					lastSeen: now,
				}
				entries[clientKey] = entry
			}
			entry.lastSeen = now
			for key, current := range entries {
				if now.Sub(current.lastSeen) > 2*window {
					delete(entries, key)
				}
			}
			allowed := entry.limiter.Allow()
			mu.Unlock()

			if !allowed {
				return c.JSON(http.StatusTooManyRequests, map[string]any{
					"type":    "rate_limit_error",
					"code":    "RATE_LIMITED",
					"message": "too many requests",
				})
			}
			return next(c)
		}
	})
}

// UseObservability tracks request counts/errors/latencies.
func (s *Server) UseObservability(collector *metrics.Collector) {
	if collector == nil {
		return
	}

	s.echo.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			collector.AddRequest()
			started := time.Now().UTC()
			err := next(c)
			collector.AddLatency(time.Since(started))
			if err != nil || c.Response().Status >= 400 {
				collector.AddError()
			}
			return err
		}
	})
}

func int64ToString(value int64) string {
	return strconv.FormatInt(value, 10)
}
