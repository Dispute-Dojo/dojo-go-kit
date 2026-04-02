# dojo-go-kit

Shared Go toolkit for Dojo microservices. Provides standardized JWT auth, HTTP responses, health checks, and structured logging.

## Packages

- `httpkit` — Response helpers, error codes, request ID middleware
- `auth` — JWT validation middleware and context helpers
- `health` — Composable health check handler
- `logging` — zerolog configuration and request logger middleware

## Install

```bash
go get github.com/Dispute-Dojo/dojo-go-kit
```

## Usage

```go
import (
    "github.com/Dispute-Dojo/dojo-go-kit/auth"
    "github.com/Dispute-Dojo/dojo-go-kit/httpkit"
    "github.com/Dispute-Dojo/dojo-go-kit/health"
    "github.com/Dispute-Dojo/dojo-go-kit/logging"
)

// Set up middleware stack
logger := logging.NewLogger("my-service")
r := chi.NewRouter()
r.Use(httpkit.RequestIDMiddleware)
r.Use(logging.RequestLoggerMiddleware(logger))

// Public health check
r.Get("/health", health.NewHandler(
    health.PingChecker("postgres", db.PingContext),
))

// Protected routes
r.Route("/api", func(r chi.Router) {
    r.Use(auth.JWTMiddleware([]byte(os.Getenv("JWT_SECRET"))))
    r.Get("/me", func(w http.ResponseWriter, r *http.Request) {
        httpkit.RespondJSON(w, 200, map[string]string{
            "user_id": auth.GetUserID(r.Context()),
        })
    })
})
```
