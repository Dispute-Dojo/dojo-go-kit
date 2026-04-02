package logging

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Dispute-Dojo/dojo-go-kit/httpkit"
)

// logEntry represents a single JSON log line for test assertions.
type logEntry struct {
	Level     string  `json:"level"`
	Service   string  `json:"service"`
	Method    string  `json:"method"`
	Path      string  `json:"path"`
	Status    int     `json:"status"`
	Duration  float64 `json:"duration_ms"`
	RequestID string  `json:"request_id"`
	Message   string  `json:"message"`
	Time      int64   `json:"time"`
}

func TestNewLoggerWithWriter_IncludesServiceName(t *testing.T) {
	var buf bytes.Buffer
	logger := NewLoggerWithWriter("test-service", &buf)

	logger.Info().Msg("boot")

	var entry logEntry
	if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
		t.Fatalf("failed to decode log line: %v\nbody: %s", err, buf.String())
	}

	if entry.Service != "test-service" {
		t.Errorf("service = %q; want %q", entry.Service, "test-service")
	}
	if entry.Message != "boot" {
		t.Errorf("message = %q; want %q", entry.Message, "boot")
	}
	if entry.Level != "info" {
		t.Errorf("level = %q; want %q", entry.Level, "info")
	}
	if entry.Time == 0 {
		t.Error("expected timestamp to be set")
	}
}

func TestRequestLoggerMiddleware_LogsRequestFields(t *testing.T) {
	var buf bytes.Buffer
	logger := NewLoggerWithWriter("api", &buf)

	handler := RequestLoggerMiddleware(logger)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
	}))

	req := httptest.NewRequest(http.MethodPost, "/v1/items", nil)
	// Manually set request ID in context so the middleware can read it.
	ctx := httpkit.SetRequestID(req.Context(), "req-abc-123")
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	var entry logEntry
	if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
		t.Fatalf("failed to decode log line: %v\nbody: %s", err, buf.String())
	}

	if entry.Method != "POST" {
		t.Errorf("method = %q; want %q", entry.Method, "POST")
	}
	if entry.Path != "/v1/items" {
		t.Errorf("path = %q; want %q", entry.Path, "/v1/items")
	}
	if entry.Status != http.StatusCreated {
		t.Errorf("status = %d; want %d", entry.Status, http.StatusCreated)
	}
	if entry.RequestID != "req-abc-123" {
		t.Errorf("request_id = %q; want %q", entry.RequestID, "req-abc-123")
	}
	if entry.Duration < 0 {
		t.Errorf("duration_ms = %f; want >= 0", entry.Duration)
	}
	if entry.Message != "request" {
		t.Errorf("message = %q; want %q", entry.Message, "request")
	}
}

func TestRequestLoggerMiddleware_ComposesWithRequestIDMiddleware(t *testing.T) {
	var buf bytes.Buffer
	logger := NewLoggerWithWriter("api", &buf)

	// Compose: RequestIDMiddleware -> RequestLoggerMiddleware -> handler
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	chain := httpkit.RequestIDMiddleware(RequestLoggerMiddleware(logger)(inner))

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	req.Header.Set("X-Request-ID", "from-header-xyz")

	rr := httptest.NewRecorder()
	chain.ServeHTTP(rr, req)

	var entry logEntry
	if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
		t.Fatalf("failed to decode log line: %v\nbody: %s", err, buf.String())
	}

	if entry.RequestID != "from-header-xyz" {
		t.Errorf("request_id = %q; want %q", entry.RequestID, "from-header-xyz")
	}
	if entry.Method != "GET" {
		t.Errorf("method = %q; want %q", entry.Method, "GET")
	}
	if entry.Path != "/healthz" {
		t.Errorf("path = %q; want %q", entry.Path, "/healthz")
	}
	if entry.Status != http.StatusOK {
		t.Errorf("status = %d; want %d", entry.Status, http.StatusOK)
	}

	// Verify response also has the echoed header from RequestIDMiddleware.
	if got := rr.Header().Get("X-Request-ID"); got != "from-header-xyz" {
		t.Errorf("X-Request-ID header = %q; want %q", got, "from-header-xyz")
	}
}
