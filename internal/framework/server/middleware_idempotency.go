package server

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/tejasvi-mehra/currency-hedge-calculator/internal/framework/metrics"
)

var uuidPattern = regexp.MustCompile(`^[a-fA-F0-9]{8}-[a-fA-F0-9]{4}-[1-5][a-fA-F0-9]{3}-[89abAB][a-fA-F0-9]{3}-[a-fA-F0-9]{12}$`)

type idempotencyRecord struct {
	key        string
	path       string
	bodyHash   string
	inFlight   bool
	statusCode int
	response   []byte
	headers    map[string]string
	createdAt  time.Time
}

type idempotencyStore struct {
	mu      sync.Mutex
	ttl     time.Duration
	records map[string]*idempotencyRecord
}

func newIdempotencyStore(ttl time.Duration) *idempotencyStore {
	if ttl <= 0 {
		ttl = 24 * time.Hour
	}
	return &idempotencyStore{
		ttl:     ttl,
		records: map[string]*idempotencyRecord{},
	}
}

func (s *Server) UseIdempotency(ttl time.Duration, collector *metrics.Collector) {
	store := newIdempotencyStore(ttl)

	s.echo.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			if c.Request().Method != http.MethodPost {
				return next(c)
			}

			key := strings.TrimSpace(c.Request().Header.Get("X-Idempotency-Key"))
			if key == "" {
				return c.JSON(http.StatusBadRequest, map[string]any{
					"type":    "validation_error",
					"code":    "MISSING_IDEMPOTENCY_KEY",
					"message": "X-Idempotency-Key is required for POST requests",
				})
			}
			if !uuidPattern.MatchString(key) {
				return c.JSON(http.StatusBadRequest, map[string]any{
					"type":    "validation_error",
					"code":    "INVALID_IDEMPOTENCY_KEY",
					"message": "X-Idempotency-Key must be a valid UUID",
				})
			}

			bodyBytes, err := io.ReadAll(c.Request().Body)
			if err != nil {
				return c.JSON(http.StatusBadRequest, map[string]any{
					"type":    "validation_error",
					"code":    "INVALID_REQUEST_BODY",
					"message": "unable to read request body",
				})
			}
			c.Request().Body = io.NopCloser(bytes.NewReader(bodyBytes))
			hash := hashPayload(bodyBytes)
			recordID := c.Request().Method + ":" + c.Path() + ":" + key
			now := time.Now().UTC()

			store.mu.Lock()
			store.evictExpired(now)
			existing, found := store.records[recordID]
			if found {
				if existing.bodyHash != hash {
					store.mu.Unlock()
					if collector != nil {
						collector.AddIdempotencyConflict()
					}
					return c.JSON(http.StatusConflict, map[string]any{
						"type":    "validation_error",
						"code":    "IDEMPOTENCY_DUPLICATED",
						"message": "idempotency key already used with a different payload",
					})
				}
				if existing.inFlight {
					store.mu.Unlock()
					if collector != nil {
						collector.AddIdempotencyConflict()
					}
					return c.JSON(http.StatusConflict, map[string]any{
						"type":    "validation_error",
						"code":    "REQUEST_IN_PROCESS",
						"message": "there is already an open call for this X-Idempotency-Key",
					})
				}

				for name, value := range existing.headers {
					if name == "" || value == "" {
						continue
					}
					c.Response().Header().Set(name, value)
				}
				c.Response().Header().Set("X-Idempotent-Replay", "true")
				store.mu.Unlock()
				if collector != nil {
					collector.AddIdempotencyReplay()
				}
				return c.Blob(existing.statusCode, existing.headers[echo.HeaderContentType], existing.response)
			}

			store.records[recordID] = &idempotencyRecord{
				key:       key,
				path:      c.Path(),
				bodyHash:  hash,
				inFlight:  true,
				createdAt: now,
			}
			store.mu.Unlock()

			recorder := newResponseRecorder(c.Response().Writer)
			c.Response().Writer = recorder
			handlerErr := next(c)

			store.mu.Lock()
			record := store.records[recordID]
			if record != nil {
				statusCode := recorder.statusCode
				if statusCode == 0 {
					statusCode = c.Response().Status
				}
				if statusCode >= 200 && statusCode < 400 {
					record.statusCode = statusCode
					record.response = recorder.body.Bytes()
					record.headers = map[string]string{
						echo.HeaderContentType: c.Response().Header().Get(echo.HeaderContentType),
					}
					record.inFlight = false
				} else {
					delete(store.records, recordID)
				}
			}
			store.mu.Unlock()

			return handlerErr
		}
	})
}

func hashPayload(payload []byte) string {
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:])
}

func (s *idempotencyStore) evictExpired(now time.Time) {
	for key, record := range s.records {
		if now.Sub(record.createdAt) > s.ttl {
			delete(s.records, key)
		}
	}
}

type responseRecorder struct {
	writer     http.ResponseWriter
	statusCode int
	body       bytes.Buffer
}

func newResponseRecorder(w http.ResponseWriter) *responseRecorder {
	return &responseRecorder{writer: w}
}

func (r *responseRecorder) Header() http.Header {
	return r.writer.Header()
}

func (r *responseRecorder) WriteHeader(code int) {
	r.statusCode = code
	r.writer.WriteHeader(code)
}

func (r *responseRecorder) Write(data []byte) (int, error) {
	if r.statusCode == 0 {
		r.statusCode = http.StatusOK
	}
	_, _ = r.body.Write(data)
	return r.writer.Write(data)
}

func (r *responseRecorder) Flush() {
	if flusher, ok := r.writer.(http.Flusher); ok {
		flusher.Flush()
	}
}

func (r *responseRecorder) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	if hijacker, ok := r.writer.(http.Hijacker); ok {
		return hijacker.Hijack()
	}
	return nil, nil, http.ErrNotSupported
}

func (r *responseRecorder) Push(target string, opts *http.PushOptions) error {
	if pusher, ok := r.writer.(http.Pusher); ok {
		return pusher.Push(target, opts)
	}
	return http.ErrNotSupported
}
