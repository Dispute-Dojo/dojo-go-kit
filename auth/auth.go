package auth

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/Dispute-Dojo/dojo-go-kit/httpkit"
	"github.com/golang-jwt/jwt/v5"
)

// Claims is the standard JWT claims struct across all Dojo services.
type Claims struct {
	UserID        string `json:"user_id"`
	Email         string `json:"email"`
	Name          string `json:"name"`
	Picture       string `json:"picture"`
	VerifiedEmail bool   `json:"verified_email"`
	jwt.RegisteredClaims
}

type contextKey int

const claimsKey contextKey = iota

// GetClaims returns the full JWT claims from context, or nil if not present.
func GetClaims(ctx context.Context) *Claims {
	if c, ok := ctx.Value(claimsKey).(*Claims); ok {
		return c
	}
	return nil
}

// GetUserID returns the user_id from JWT claims in context, or empty string.
func GetUserID(ctx context.Context) string {
	if c := GetClaims(ctx); c != nil {
		return c.UserID
	}
	return ""
}

// GetEmail returns the email from JWT claims in context, or empty string.
func GetEmail(ctx context.Context) string {
	if c := GetClaims(ctx); c != nil {
		return c.Email
	}
	return ""
}

// ValidateToken parses and validates a JWT token string.
// Use this for non-HTTP flows (WebSocket, background jobs, etc).
func ValidateToken(secret []byte, tokenString string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return secret, nil
	})
	if err != nil {
		return nil, err
	}

	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, fmt.Errorf("invalid token claims")
	}

	return claims, nil
}

// JWTMiddleware returns chi-compatible middleware that validates JWT tokens.
func JWTMiddleware(secret []byte) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tokenString := extractBearer(r)
			if tokenString == "" {
				httpkit.RespondError(w, r, http.StatusUnauthorized, "missing authorization header")
				return
			}

			claims, err := ValidateToken(secret, tokenString)
			if err != nil {
				httpkit.RespondError(w, r, http.StatusUnauthorized, "invalid token")
				return
			}

			ctx := context.WithValue(r.Context(), claimsKey, claims)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func extractBearer(r *http.Request) string {
	h := r.Header.Get("Authorization")
	if h == "" {
		return ""
	}
	parts := strings.SplitN(h, " ", 2)
	if len(parts) != 2 || parts[0] != "Bearer" {
		return ""
	}
	return parts[1]
}
