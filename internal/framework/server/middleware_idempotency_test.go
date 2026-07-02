package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestIdempotency_ReplayAndConflict(t *testing.T) {
	api := New(":0", nil)
	api.UseIdempotency(24*time.Hour, nil)
	api.POST("/echo", func(ctx Context) error {
		payload := map[string]any{}
		if err := ctx.Bind(&payload); err != nil {
			return ctx.JSON(http.StatusBadRequest, map[string]any{"error": "bad payload"})
		}
		return ctx.JSON(http.StatusOK, map[string]any{"payload": payload})
	})

	srv := httptest.NewServer(api.HTTPHandler())
	defer srv.Close()

	key := "7bf41af5-70ae-4e79-9b28-a8fa75c3ac53"
	first := doPostJSON(t, srv.URL+"/echo", key, `{"value":1}`)
	if first.StatusCode != http.StatusOK {
		t.Fatalf("first status = %d", first.StatusCode)
	}

	replay := doPostJSON(t, srv.URL+"/echo", key, `{"value":1}`)
	if replay.StatusCode != http.StatusOK {
		t.Fatalf("replay status = %d", replay.StatusCode)
	}
	if replay.Header.Get("X-Idempotent-Replay") != "true" {
		t.Fatalf("expected replay header")
	}

	conflict := doPostJSON(t, srv.URL+"/echo", key, `{"value":2}`)
	if conflict.StatusCode != http.StatusConflict {
		t.Fatalf("conflict status = %d", conflict.StatusCode)
	}
}

func TestIdempotency_RequestInProcessAndValidation(t *testing.T) {
	api := New(":0", nil)
	api.UseIdempotency(24*time.Hour, nil)
	api.POST("/slow", func(ctx Context) error {
		time.Sleep(120 * time.Millisecond)
		return ctx.JSON(http.StatusOK, map[string]any{"ok": true})
	})

	srv := httptest.NewServer(api.HTTPHandler())
	defer srv.Close()

	invalid := doPostJSON(t, srv.URL+"/slow", "not-a-uuid", `{"value":1}`)
	if invalid.StatusCode != http.StatusBadRequest {
		t.Fatalf("invalid key status = %d", invalid.StatusCode)
	}

	key := "7bf41af5-70ae-4e79-9b28-a8fa75c3ac53"
	done := make(chan *http.Response, 1)
	go func() {
		done <- doPostJSON(t, srv.URL+"/slow", key, `{"value":1}`)
	}()
	time.Sleep(25 * time.Millisecond)

	inProcess := doPostJSON(t, srv.URL+"/slow", key, `{"value":1}`)
	if inProcess.StatusCode != http.StatusConflict {
		t.Fatalf("in-process status = %d", inProcess.StatusCode)
	}
	<-done
}

func TestAPIKeyAuth(t *testing.T) {
	api := New(":0", nil)
	api.UseAPIKeyAuth("test-key")
	api.POST("/secure", func(ctx Context) error {
		return ctx.JSON(http.StatusOK, map[string]any{"ok": true})
	})
	srv := httptest.NewServer(api.HTTPHandler())
	defer srv.Close()

	request, err := http.NewRequest(http.MethodPost, srv.URL+"/secure", bytes.NewBufferString(`{}`))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	request.Header.Set("Content-Type", "application/json")
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatalf("perform request: %v", err)
	}
	defer func() { _ = response.Body.Close() }()

	if response.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", response.StatusCode)
	}
}

func doPostJSON(t *testing.T, endpoint string, idempotencyKey string, body string) *http.Response {
	t.Helper()
	request, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewBufferString(body))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	request.Header.Set("Content-Type", "application/json")
	if idempotencyKey != "" {
		request.Header.Set("X-Idempotency-Key", idempotencyKey)
	}
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatalf("perform request: %v", err)
	}
	defer func() {
		// fully drain body to avoid leaked connections while still returning the response object
		var sink map[string]any
		_ = json.NewDecoder(response.Body).Decode(&sink)
		_ = response.Body.Close()
	}()
	// recreate a closed body-safe response for callers only interested in status/headers.
	return &http.Response{StatusCode: response.StatusCode, Header: response.Header}
}
