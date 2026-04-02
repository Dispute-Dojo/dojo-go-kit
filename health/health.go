package health

import (
	"context"
	"encoding/json"
	"net/http"
)

// Checker runs a health check against a dependency.
type Checker interface {
	Name() string
	Check(ctx context.Context) error
}

type pingChecker struct {
	name string
	ping func(ctx context.Context) error
}

func (c *pingChecker) Name() string                    { return c.name }
func (c *pingChecker) Check(ctx context.Context) error { return c.ping(ctx) }

// PingChecker creates a Checker from any func(ctx) error.
func PingChecker(name string, ping func(ctx context.Context) error) Checker {
	return &pingChecker{name: name, ping: ping}
}

// NewHandler returns an http.HandlerFunc that runs all checkers.
// Zero checkers: 200 {"status":"healthy"}.
// All pass: 200 {"status":"healthy","checks":{...}}.
// Any fail: 503 {"status":"degraded","checks":{...}}.
func NewHandler(checkers ...Checker) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if len(checkers) == 0 {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]string{"status": "healthy"})
			return
		}

		checks := make(map[string]string, len(checkers))
		healthy := true

		for _, c := range checkers {
			if err := c.Check(r.Context()); err != nil {
				checks[c.Name()] = err.Error()
				healthy = false
			} else {
				checks[c.Name()] = "ok"
			}
		}

		status := "healthy"
		code := http.StatusOK
		if !healthy {
			status = "degraded"
			code = http.StatusServiceUnavailable
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(code)
		json.NewEncoder(w).Encode(map[string]any{
			"status": status,
			"checks": checks,
		})
	}
}
