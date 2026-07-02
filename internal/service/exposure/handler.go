package exposure

import (
	"errors"
	"io"
	"net/http"

	"github.com/tejasvi-mehra/currency-hedge-calculator/internal/framework/server"
	"github.com/tejasvi-mehra/currency-hedge-calculator/internal/service/rates"
	"go.uber.org/zap"
)

// Handler exposes HTTP handlers for exposure use cases.
type Handler struct {
	service *Service
	logger  *zap.SugaredLogger
}

// NewHandler creates an exposure API handler.
func NewHandler(service *Service, logger *zap.SugaredLogger) *Handler {
	if logger == nil {
		logger = zap.NewNop().Sugar()
	}
	return &Handler{
		service: service,
		logger:  logger,
	}
}

// Register binds exposure routes into the shared framework server.
func (h *Handler) Register(apiServer *server.Server, healthPath string) {
	apiServer.POST("/v1/exposure/calculate", h.calculate)
	apiServer.POST("/v1/exposure/calculate/test", h.calculateWithDefaultTestData)
	apiServer.GET(healthPath, h.healthz)
}

// calculate handles exposure calculation requests.
func (h *Handler) calculate(ctx server.Context) error {
	request := CalculateExposureRequest{}
	if err := ctx.Bind(&request); err != nil {
		if errors.Is(err, io.EOF) {
			request = CalculateExposureRequest{}
		} else {
			h.logger.Warnw("invalid calculate request payload", "error", err)
			return ctx.JSON(http.StatusBadRequest, ErrorResponse{
				Type:    "validation_error",
				Code:    "INVALID_REQUEST_BODY",
				Message: "request body must be a valid JSON object",
				Details: map[string]any{"error": err.Error()},
			})
		}
	}

	response, err := h.service.CalculateExposure(ctx.RequestContext(), request)
	if err != nil {
		return h.writeServiceError(ctx, err)
	}

	return ctx.JSON(http.StatusOK, response)
}

// calculateWithDefaultTestData handles test-only exposure runs backed by configured test data.
func (h *Handler) calculateWithDefaultTestData(ctx server.Context) error {
	request := CalculateExposureRequest{}
	if err := ctx.Bind(&request); err != nil {
		if !errors.Is(err, io.EOF) {
			h.logger.Warnw("invalid test calculate request payload", "error", err)
			return ctx.JSON(http.StatusBadRequest, ErrorResponse{
				Type:    "validation_error",
				Code:    "INVALID_REQUEST_BODY",
				Message: "request body must be a valid JSON object",
				Details: map[string]any{"error": err.Error()},
			})
		}
	}

	// Test endpoint always ignores request transactions and loads from test data source.
	request.Transactions = nil
	request.UseDefaultTestDataWhenEmptyInput = true

	response, err := h.service.CalculateExposure(ctx.RequestContext(), request)
	if err != nil {
		return h.writeServiceError(ctx, err)
	}

	return ctx.JSON(http.StatusOK, response)
}

// healthz confirms the API process is alive.
func (h *Handler) healthz(ctx server.Context) error {
	return ctx.String(http.StatusOK, "ok")
}

func (h *Handler) writeServiceError(ctx server.Context, err error) error {
	var statusCode int
	payload := ErrorResponse{
		Type:    "internal_error",
		Code:    "INTERNAL_ERROR",
		Message: "failed to calculate exposure",
	}

	switch {
	case errors.Is(err, ErrNoTransactions):
		statusCode = http.StatusBadRequest
		payload.Type = "validation_error"
		payload.Code = "NO_TRANSACTIONS"
		payload.Message = err.Error()
	case errors.Is(err, ErrValidation):
		statusCode = http.StatusBadRequest
		payload.Type = "validation_error"
		payload.Code = "INVALID_TRANSACTION_INPUT"
		payload.Message = err.Error()
	case errors.Is(err, rates.ErrUnsupportedCurrencyPair):
		statusCode = http.StatusBadRequest
		payload.Type = "validation_error"
		payload.Code = "UNSUPPORTED_CURRENCY_PAIR"
		payload.Message = err.Error()
	case errors.Is(err, rates.ErrRateUnavailable):
		statusCode = http.StatusBadGateway
		payload.Type = "upstream_error"
		payload.Code = "FX_RATE_UNAVAILABLE"
		payload.Message = err.Error()
	default:
		statusCode = http.StatusInternalServerError
		payload.Message = "internal processing error"
	}

	if statusCode >= http.StatusInternalServerError {
		h.logger.Errorw("calculate exposure failed", "error", err)
	} else {
		h.logger.Warnw("calculate exposure request failed", "error", err)
	}
	return ctx.JSON(statusCode, payload)
}
