package server

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"go.uber.org/zap"
)

// Context is a framework-owned request abstraction for handlers.
type Context interface {
	RequestContext() context.Context
	Header(name string) string
	Path() string
	Bind(target any) error
	JSON(code int, value any) error
	String(code int, value string) error
}

// HandlerFunc is a framework-agnostic HTTP handler signature.
type HandlerFunc func(Context) error

type contextAdapter struct {
	echo.Context
}

func (c contextAdapter) RequestContext() context.Context { return c.Request().Context() }
func (c contextAdapter) Header(name string) string       { return c.Request().Header.Get(name) }
func (c contextAdapter) Path() string                    { return c.Request().URL.Path }
func (c contextAdapter) Bind(target any) error           { return c.Context.Bind(target) }
func (c contextAdapter) JSON(code int, value any) error  { return c.Context.JSON(code, value) }
func (c contextAdapter) String(code int, value string) error {
	return c.Context.String(code, value)
}

// Server wraps Echo setup and lifecycle concerns.
type Server struct {
	echo   *echo.Echo
	server *http.Server
	logger *zap.SugaredLogger
}

// New constructs the HTTP server with baseline middleware.
func New(address string, logger *zap.SugaredLogger) *Server {
	if logger == nil {
		logger = zap.NewNop().Sugar()
	}

	e := echo.New()
	e.HideBanner = true
	e.Use(middleware.Recover())
	e.Use(middleware.RequestID())

	return &Server{
		echo: e,
		server: &http.Server{
			Addr:    address,
			Handler: e,
		},
		logger: logger,
	}
}

// UseCORS applies CORS middleware for local and deployment scenarios.
func (s *Server) UseCORS(allowedOrigins []string) {
	origins := make([]string, 0, len(allowedOrigins))
	for _, origin := range allowedOrigins {
		trimmed := strings.TrimSpace(origin)
		if trimmed == "" {
			continue
		}
		origins = append(origins, trimmed)
	}
	if len(origins) == 0 {
		origins = []string{"*"}
	}

	s.echo.Use(middleware.CORSWithConfig(middleware.CORSConfig{
		AllowOrigins: origins,
		AllowMethods: []string{http.MethodGet, http.MethodPost, http.MethodOptions},
		AllowHeaders: []string{
			echo.HeaderOrigin,
			echo.HeaderContentType,
			echo.HeaderAccept,
			echo.HeaderAuthorization,
			"X-API-Key",
			"X-Idempotency-Key",
		},
	}))
}

// UseRequestLogging installs structured request logging.
func (s *Server) UseRequestLogging() {
	s.echo.Use(middleware.RequestLoggerWithConfig(middleware.RequestLoggerConfig{
		LogMethod:   true,
		LogURI:      true,
		LogStatus:   true,
		LogLatency:  true,
		LogError:    true,
		LogRemoteIP: true,
		LogValuesFunc: func(c echo.Context, values middleware.RequestLoggerValues) error {
			fields := []any{
				"method", values.Method,
				"uri", values.URI,
				"status", values.Status,
				"latency", values.Latency.String(),
				"remote_ip", values.RemoteIP,
			}
			if requestID := c.Response().Header().Get(echo.HeaderXRequestID); requestID != "" {
				fields = append(fields, "request_id", requestID)
			}
			if idempotencyKey := c.Request().Header.Get("X-Idempotency-Key"); idempotencyKey != "" {
				fields = append(fields, "idempotency_key", idempotencyKey)
			}
			if values.Error != nil {
				fields = append(fields, "error", values.Error)
				s.logger.Warnw("http request failed", fields...)
				return nil
			}
			s.logger.Infow("http request", fields...)
			return nil
		},
	}))
}

// GET registers a GET endpoint.
func (s *Server) GET(path string, handler HandlerFunc) {
	s.echo.GET(path, func(c echo.Context) error {
		return handler(contextAdapter{Context: c})
	})
}

// POST registers a POST endpoint.
func (s *Server) POST(path string, handler HandlerFunc) {
	s.echo.POST(path, func(c echo.Context) error {
		return handler(contextAdapter{Context: c})
	})
}

// StartAsync starts the HTTP server and reports start/runtime failures.
func (s *Server) StartAsync(errCh chan<- error) {
	go func() {
		s.logger.Infow("http server starting", "address", s.server.Addr)
		if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
		close(errCh)
	}()
}

// Shutdown gracefully stops the HTTP server with a timeout.
func (s *Server) Shutdown(ctx context.Context) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	return s.server.Shutdown(timeoutCtx)
}

// HTTPHandler exposes the underlying HTTP handler for integration tests.
func (s *Server) HTTPHandler() http.Handler {
	return s.echo
}
