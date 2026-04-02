package health_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Dispute-Dojo/dojo-go-kit/health"
)

func TestNewHandler_ZeroCheckers(t *testing.T) {
	t.Parallel()

	handler := health.NewHandler()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/health", nil)

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}

	ct := rec.Header().Get("Content-Type")
	if ct != "application/json" {
		t.Fatalf("expected Content-Type application/json, got %q", ct)
	}

	var body map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode response body: %v", err)
	}

	if body["status"] != "healthy" {
		t.Fatalf("expected status healthy, got %q", body["status"])
	}

	if _, exists := body["checks"]; exists {
		t.Fatal("expected no checks key when zero checkers provided")
	}
}

func TestNewHandler_AllCheckersPass(t *testing.T) {
	t.Parallel()

	pg := health.PingChecker("postgres", func(ctx context.Context) error { return nil })
	redis := health.PingChecker("redis", func(ctx context.Context) error { return nil })

	handler := health.NewHandler(pg, redis)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/health", nil)

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}

	var body map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode response body: %v", err)
	}

	if body["status"] != "healthy" {
		t.Fatalf("expected status healthy, got %q", body["status"])
	}

	checks, ok := body["checks"].(map[string]any)
	if !ok {
		t.Fatal("expected checks to be a map")
	}

	if checks["postgres"] != "ok" {
		t.Fatalf("expected postgres check ok, got %q", checks["postgres"])
	}
	if checks["redis"] != "ok" {
		t.Fatalf("expected redis check ok, got %q", checks["redis"])
	}
}

func TestNewHandler_OneCheckerFails(t *testing.T) {
	t.Parallel()

	pg := health.PingChecker("postgres", func(ctx context.Context) error { return nil })
	redis := health.PingChecker("redis", func(ctx context.Context) error {
		return errors.New("connection refused")
	})

	handler := health.NewHandler(pg, redis)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/health", nil)

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected status 503, got %d", rec.Code)
	}

	var body map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode response body: %v", err)
	}

	if body["status"] != "degraded" {
		t.Fatalf("expected status degraded, got %q", body["status"])
	}

	checks, ok := body["checks"].(map[string]any)
	if !ok {
		t.Fatal("expected checks to be a map")
	}

	if checks["postgres"] != "ok" {
		t.Fatalf("expected postgres check ok, got %q", checks["postgres"])
	}
	if checks["redis"] != "connection refused" {
		t.Fatalf("expected redis check 'connection refused', got %q", checks["redis"])
	}
}

func TestNewHandler_AllCheckersFail(t *testing.T) {
	t.Parallel()

	pg := health.PingChecker("postgres", func(ctx context.Context) error {
		return errors.New("timeout")
	})
	redis := health.PingChecker("redis", func(ctx context.Context) error {
		return errors.New("connection refused")
	})

	handler := health.NewHandler(pg, redis)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/health", nil)

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected status 503, got %d", rec.Code)
	}

	var body map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode response body: %v", err)
	}

	if body["status"] != "degraded" {
		t.Fatalf("expected status degraded, got %q", body["status"])
	}

	checks, ok := body["checks"].(map[string]any)
	if !ok {
		t.Fatal("expected checks to be a map")
	}

	if checks["postgres"] != "timeout" {
		t.Fatalf("expected postgres check 'timeout', got %q", checks["postgres"])
	}
	if checks["redis"] != "connection refused" {
		t.Fatalf("expected redis check 'connection refused', got %q", checks["redis"])
	}
}

func TestPingChecker_NameAndCheck(t *testing.T) {
	t.Parallel()

	called := false
	c := health.PingChecker("mydb", func(ctx context.Context) error {
		called = true
		return nil
	})

	if c.Name() != "mydb" {
		t.Fatalf("expected name mydb, got %q", c.Name())
	}

	if err := c.Check(context.Background()); err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	if !called {
		t.Fatal("expected ping function to be called")
	}
}

func TestPingChecker_ReturnsError(t *testing.T) {
	t.Parallel()

	want := errors.New("dial tcp: connection refused")
	c := health.PingChecker("cache", func(ctx context.Context) error {
		return want
	})

	got := c.Check(context.Background())
	if !errors.Is(got, want) {
		t.Fatalf("expected error %v, got %v", want, got)
	}
}
