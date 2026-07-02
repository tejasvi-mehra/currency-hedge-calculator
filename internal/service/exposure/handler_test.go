package exposure

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
)

type fakeContext struct {
	bindErr    error
	boundBody  CalculateExposureRequest
	statusCode int
	response   any
}

func (f *fakeContext) RequestContext() context.Context { return context.Background() }
func (f *fakeContext) Header(string) string            { return "" }
func (f *fakeContext) Path() string                    { return "/v1/exposure/calculate" }
func (f *fakeContext) Bind(target any) error {
	if f.bindErr != nil {
		return f.bindErr
	}
	request, ok := target.(*CalculateExposureRequest)
	if !ok {
		return errors.New("unexpected bind target")
	}
	*request = f.boundBody
	return nil
}
func (f *fakeContext) JSON(code int, value any) error {
	f.statusCode = code
	f.response = value
	return nil
}
func (f *fakeContext) String(code int, value string) error {
	f.statusCode = code
	f.response = value
	return nil
}

func TestHandlerCalculate_BadPayload(t *testing.T) {
	handler := NewHandler(nil, nil)
	ctx := &fakeContext{
		bindErr: errors.New("invalid json"),
	}

	if err := handler.calculate(ctx); err != nil {
		t.Fatalf("calculate() unexpected error: %v", err)
	}
	if ctx.statusCode != 400 {
		t.Fatalf("expected 400, got %d", ctx.statusCode)
	}

	response, ok := ctx.response.(ErrorResponse)
	if !ok {
		t.Fatalf("expected ErrorResponse, got %T", ctx.response)
	}
	if response.Code != "INVALID_REQUEST_BODY" {
		t.Fatalf("unexpected error code: %s", response.Code)
	}
}

func TestHandlerHealthz(t *testing.T) {
	handler := NewHandler(nil, nil)
	ctx := &fakeContext{}

	if err := handler.healthz(ctx); err != nil {
		t.Fatalf("healthz() unexpected error: %v", err)
	}
	if ctx.statusCode != 200 || ctx.response != "ok" {
		t.Fatalf("unexpected healthz response: %d, %v", ctx.statusCode, ctx.response)
	}
}

func TestErrorResponseJSONShape(t *testing.T) {
	payload := ErrorResponse{
		Type:    "validation_error",
		Code:    "INVALID_REQUEST_BODY",
		Message: "invalid body",
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal error response: %v", err)
	}
	if len(raw) == 0 {
		t.Fatalf("expected non-empty json payload")
	}
}
