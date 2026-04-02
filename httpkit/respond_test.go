package httpkit

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRespondJSON_WithData(t *testing.T) {
	t.Parallel()

	w := httptest.NewRecorder()
	data := map[string]string{"name": "dojo"}

	RespondJSON(w, http.StatusOK, data)

	res := w.Result()
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		t.Errorf("status = %d; want %d", res.StatusCode, http.StatusOK)
	}

	ct := res.Header.Get("Content-Type")
	if ct != "application/json" {
		t.Errorf("Content-Type = %q; want %q", ct, "application/json")
	}

	var body map[string]string
	if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode body: %v", err)
	}
	if body["name"] != "dojo" {
		t.Errorf("body[name] = %q; want %q", body["name"], "dojo")
	}
}

func TestRespondJSON_NilData(t *testing.T) {
	t.Parallel()

	w := httptest.NewRecorder()

	RespondJSON(w, http.StatusNoContent, nil)

	res := w.Result()
	defer res.Body.Close()

	if res.StatusCode != http.StatusNoContent {
		t.Errorf("status = %d; want %d", res.StatusCode, http.StatusNoContent)
	}

	if w.Body.Len() != 0 {
		t.Errorf("body length = %d; want 0", w.Body.Len())
	}
}

func TestRespondError_ErrorCodes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		status   int
		wantCode string
	}{
		{http.StatusBadRequest, "VALIDATION_ERROR"},
		{http.StatusUnauthorized, "UNAUTHORIZED"},
		{http.StatusForbidden, "FORBIDDEN"},
		{http.StatusNotFound, "NOT_FOUND"},
		{http.StatusConflict, "CONFLICT"},
		{http.StatusTooManyRequests, "RATE_LIMITED"},
		{http.StatusInternalServerError, "INTERNAL_ERROR"},
		{http.StatusServiceUnavailable, "SERVICE_UNAVAILABLE"},
		{http.StatusGatewayTimeout, "TIMEOUT"},
	}

	for _, tt := range tests {
		t.Run(tt.wantCode, func(t *testing.T) {
			t.Parallel()

			w := httptest.NewRecorder()
			r := httptest.NewRequest(http.MethodGet, "/", nil)

			RespondError(w, r, tt.status, "something went wrong")

			res := w.Result()
			defer res.Body.Close()

			if res.StatusCode != tt.status {
				t.Errorf("status = %d; want %d", res.StatusCode, tt.status)
			}

			var body ErrorResponse
			if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
				t.Fatalf("failed to decode body: %v", err)
			}
			if body.Code != tt.wantCode {
				t.Errorf("code = %q; want %q", body.Code, tt.wantCode)
			}
			if body.Error != "something went wrong" {
				t.Errorf("error = %q; want %q", body.Error, "something went wrong")
			}
		})
	}
}

func TestRespondError_UnmappedStatusFallsBackToInternalError(t *testing.T) {
	t.Parallel()

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", nil)

	RespondError(w, r, http.StatusTeapot, "i am a teapot")

	res := w.Result()
	defer res.Body.Close()

	var body ErrorResponse
	if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode body: %v", err)
	}
	if body.Code != "INTERNAL_ERROR" {
		t.Errorf("code = %q; want %q", body.Code, "INTERNAL_ERROR")
	}
}

func TestRespondError_IncludesRequestIDFromContext(t *testing.T) {
	t.Parallel()

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	ctx := SetRequestID(r.Context(), "req-abc-123")
	r = r.WithContext(ctx)

	RespondError(w, r, http.StatusBadRequest, "bad input")

	res := w.Result()
	defer res.Body.Close()

	var body ErrorResponse
	if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode body: %v", err)
	}
	if body.RequestID != "req-abc-123" {
		t.Errorf("request_id = %q; want %q", body.RequestID, "req-abc-123")
	}
}

func TestRequestIDMiddleware_GeneratesUUID(t *testing.T) {
	t.Parallel()

	var capturedID string
	handler := RequestIDMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedID = GetRequestID(r.Context())
	}))

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	// No X-Request-ID header set.

	handler.ServeHTTP(w, r)

	if capturedID == "" {
		t.Fatal("expected generated request ID; got empty string")
	}
	if len(capturedID) < 32 {
		t.Errorf("generated ID %q looks too short for a UUID", capturedID)
	}
}

func TestRequestIDMiddleware_PreservesExistingHeader(t *testing.T) {
	t.Parallel()

	var capturedID string
	handler := RequestIDMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedID = GetRequestID(r.Context())
	}))

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set("X-Request-ID", "existing-id-456")

	handler.ServeHTTP(w, r)

	if capturedID != "existing-id-456" {
		t.Errorf("request ID = %q; want %q", capturedID, "existing-id-456")
	}
}

func TestRequestIDMiddleware_EchoesIDInResponseHeader(t *testing.T) {
	t.Parallel()

	handler := RequestIDMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set("X-Request-ID", "echo-me-789")

	handler.ServeHTTP(w, r)

	got := w.Result().Header.Get("X-Request-ID")
	if got != "echo-me-789" {
		t.Errorf("response X-Request-ID = %q; want %q", got, "echo-me-789")
	}
}

func TestGetRequestID_EmptyWhenNotInContext(t *testing.T) {
	t.Parallel()

	id := GetRequestID(context.Background())
	if id != "" {
		t.Errorf("GetRequestID = %q; want empty string", id)
	}
}
