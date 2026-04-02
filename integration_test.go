package dojogokit_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Dispute-Dojo/dojo-go-kit/auth"
	"github.com/Dispute-Dojo/dojo-go-kit/health"
	"github.com/Dispute-Dojo/dojo-go-kit/httpkit"
	"github.com/Dispute-Dojo/dojo-go-kit/logging"
	"github.com/go-chi/chi/v5"
	"github.com/golang-jwt/jwt/v5"
)

func setupRouter(secret []byte) http.Handler {
	logger := logging.NewLoggerWithWriter("integration-test", io.Discard)

	r := chi.NewRouter()
	r.Use(httpkit.RequestIDMiddleware)
	r.Use(logging.RequestLoggerMiddleware(logger))
	r.Get("/health", health.NewHandler())
	r.Route("/api", func(r chi.Router) {
		r.Use(auth.JWTMiddleware(secret))
		r.Get("/me", func(w http.ResponseWriter, r *http.Request) {
			httpkit.RespondJSON(w, http.StatusOK, map[string]string{
				"user_id": auth.GetUserID(r.Context()),
				"email":   auth.GetEmail(r.Context()),
			})
		})
	})

	return r
}

func signToken(t *testing.T, secret []byte, claims *auth.Claims) string {
	t.Helper()
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString(secret)
	if err != nil {
		t.Fatalf("failed to sign token: %v", err)
	}
	return signed
}

func TestFullMiddlewareStack(t *testing.T) {
	secret := []byte("test-secret-key-for-integration")
	router := setupRouter(secret)

	t.Run("health_no_auth", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/health", nil)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", rec.Code)
		}

		var body map[string]string
		if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		if body["status"] != "healthy" {
			t.Fatalf("expected status=healthy, got %q", body["status"])
		}
	})

	t.Run("protected_no_token", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/me", nil)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401, got %d", rec.Code)
		}

		var body httpkit.ErrorResponse
		if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		if body.Code != "UNAUTHORIZED" {
			t.Fatalf("expected code=UNAUTHORIZED, got %q", body.Code)
		}
		if body.RequestID == "" {
			t.Fatal("expected non-empty request_id in error response")
		}
	})

	t.Run("protected_valid_token", func(t *testing.T) {
		claims := &auth.Claims{
			UserID: "user-123",
			Email:  "dale@dojo.com",
			RegisteredClaims: jwt.RegisteredClaims{
				ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
				IssuedAt:  jwt.NewNumericDate(time.Now()),
			},
		}
		token := signToken(t, secret, claims)

		req := httptest.NewRequest(http.MethodGet, "/api/me", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", rec.Code)
		}

		var body map[string]string
		if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		if body["user_id"] != "user-123" {
			t.Fatalf("expected user_id=user-123, got %q", body["user_id"])
		}
		if body["email"] != "dale@dojo.com" {
			t.Fatalf("expected email=dale@dojo.com, got %q", body["email"])
		}
	})

	t.Run("request_id_propagation", func(t *testing.T) {
		customID := "req-abc-123"
		req := httptest.NewRequest(http.MethodGet, "/health", nil)
		req.Header.Set("X-Request-ID", customID)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		got := rec.Header().Get("X-Request-ID")
		if got != customID {
			t.Fatalf("expected X-Request-ID=%q, got %q", customID, got)
		}
	})
}
