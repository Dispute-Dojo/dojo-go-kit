package httpkit

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/google/uuid"
)

// ErrorResponse is the standard error shape returned by all Dojo services.
type ErrorResponse struct {
	Error     string `json:"error"`
	Code      string `json:"code"`
	RequestID string `json:"request_id,omitempty"`
}

type contextKey int

const requestIDKey contextKey = iota

// RespondJSON writes data as JSON with the given status code.
// If data is nil, only the status code is written.
func RespondJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if data != nil {
		json.NewEncoder(w).Encode(data)
	}
}

// RespondError writes a standardized error response. It auto-extracts the
// request ID from context and maps the HTTP status to a machine-readable code.
func RespondError(w http.ResponseWriter, r *http.Request, status int, message string) {
	RespondJSON(w, status, ErrorResponse{
		Error:     message,
		Code:      ErrorCodeFromStatus(status),
		RequestID: GetRequestID(r.Context()),
	})
}

// ErrorCodeFromStatus maps an HTTP status to a machine-readable error code.
func ErrorCodeFromStatus(status int) string {
	switch status {
	case http.StatusBadRequest:
		return "VALIDATION_ERROR"
	case http.StatusUnauthorized:
		return "UNAUTHORIZED"
	case http.StatusForbidden:
		return "FORBIDDEN"
	case http.StatusNotFound:
		return "NOT_FOUND"
	case http.StatusConflict:
		return "CONFLICT"
	case http.StatusTooManyRequests:
		return "RATE_LIMITED"
	case http.StatusServiceUnavailable:
		return "SERVICE_UNAVAILABLE"
	case http.StatusGatewayTimeout:
		return "TIMEOUT"
	default:
		return "INTERNAL_ERROR"
	}
}

// RequestIDMiddleware reads X-Request-ID from the incoming request header.
// If missing, it generates a new UUID. The ID is stored in the request context
// and echoed back in the response header.
func RequestIDMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get("X-Request-ID")
		if id == "" {
			id = uuid.New().String()
		}
		w.Header().Set("X-Request-ID", id)
		ctx := context.WithValue(r.Context(), requestIDKey, id)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// GetRequestID returns the request ID from context, or empty string if not set.
func GetRequestID(ctx context.Context) string {
	if v, ok := ctx.Value(requestIDKey).(string); ok {
		return v
	}
	return ""
}

// SetRequestID stores a request ID in context. Primarily for testing.
func SetRequestID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, requestIDKey, id)
}
