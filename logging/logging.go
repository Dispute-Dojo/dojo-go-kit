package logging

import (
	"io"
	"net/http"
	"os"
	"time"

	"github.com/Dispute-Dojo/dojo-go-kit/httpkit"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/rs/zerolog"
)

// NewLogger returns a zerolog.Logger configured for the given service.
// Uses JSON output in production (ENV=production), pretty console otherwise.
func NewLogger(serviceName string) zerolog.Logger {
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix

	var writer io.Writer = os.Stderr
	if os.Getenv("ENV") != "production" {
		writer = zerolog.ConsoleWriter{Out: os.Stderr}
	}

	return zerolog.New(writer).With().
		Timestamp().
		Str("service", serviceName).
		Logger()
}

// NewLoggerWithWriter returns a logger that writes to w. For testing.
func NewLoggerWithWriter(serviceName string, w io.Writer) zerolog.Logger {
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	return zerolog.New(w).With().
		Timestamp().
		Str("service", serviceName).
		Logger()
}

// RequestLoggerMiddleware logs every HTTP request with method, path, status,
// duration, and request_id. Requires httpkit.RequestIDMiddleware upstream.
func RequestLoggerMiddleware(logger zerolog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)

			defer func() {
				logger.Info().
					Str("method", r.Method).
					Str("path", r.URL.Path).
					Int("status", ww.Status()).
					Float64("duration_ms", float64(time.Since(start).Microseconds())/1000.0).
					Str("request_id", httpkit.GetRequestID(r.Context())).
					Msg("request")
			}()

			next.ServeHTTP(ww, r)
		})
	}
}
