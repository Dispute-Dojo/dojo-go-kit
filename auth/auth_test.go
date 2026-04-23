package auth

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Dispute-Dojo/dojo-go-kit/httpkit"
	"github.com/golang-jwt/jwt/v5"
)

var testSecret = []byte("test-secret-key-32-bytes-long!!!")

func makeToken(t *testing.T, secret []byte, claims Claims) string {
	t.Helper()
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	s, err := token.SignedString(secret)
	if err != nil {
		t.Fatalf("failed to sign token: %v", err)
	}
	return s
}

func validClaims() Claims {
	return Claims{
		UserID:        "usr_123",
		Email:         "dale@dojo.com",
		Name:          "Dale Yarborough",
		Picture:       "https://example.com/pic.jpg",
		VerifiedEmail: true,
		OrgID:         "org_redlobster",
		CustomerID:    "4",
		Role:          "org_admin",
		Scope:         "all_stores",
		StoreIDs:      []string{"store_1", "store_2"},
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
}

// ---------------------------------------------------------------------------
// ValidateToken tests
// ---------------------------------------------------------------------------

func TestValidateToken_ValidToken(t *testing.T) {
	tokenStr := makeToken(t, testSecret, validClaims())

	claims, err := ValidateToken(testSecret, tokenStr)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if claims.UserID != "usr_123" {
		t.Errorf("UserID = %q, want %q", claims.UserID, "usr_123")
	}
	if claims.Email != "dale@dojo.com" {
		t.Errorf("Email = %q, want %q", claims.Email, "dale@dojo.com")
	}
	if claims.Name != "Dale Yarborough" {
		t.Errorf("Name = %q, want %q", claims.Name, "Dale Yarborough")
	}
	if !claims.VerifiedEmail {
		t.Error("VerifiedEmail = false, want true")
	}
	if claims.CustomerID != "4" {
		t.Errorf("CustomerID = %q, want %q", claims.CustomerID, "4")
	}
	if claims.OrgID != "org_redlobster" {
		t.Errorf("OrgID = %q, want %q", claims.OrgID, "org_redlobster")
	}
	if claims.Role != "org_admin" {
		t.Errorf("Role = %q, want %q", claims.Role, "org_admin")
	}
}

func TestValidateToken_ExpiredToken(t *testing.T) {
	c := validClaims()
	c.ExpiresAt = jwt.NewNumericDate(time.Now().Add(-time.Hour))
	tokenStr := makeToken(t, testSecret, c)

	_, err := ValidateToken(testSecret, tokenStr)
	if err == nil {
		t.Fatal("expected error for expired token, got nil")
	}
}

func TestValidateToken_WrongSecret(t *testing.T) {
	tokenStr := makeToken(t, testSecret, validClaims())
	wrongSecret := []byte("wrong-secret-key-32-bytes-long!!")

	_, err := ValidateToken(wrongSecret, tokenStr)
	if err == nil {
		t.Fatal("expected error for wrong secret, got nil")
	}
}

func TestValidateToken_NonHMACSigningMethod(t *testing.T) {
	// Create a token signed with ECDSA (non-HMAC) to verify the alg check.
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("failed to generate ECDSA key: %v", err)
	}
	token := jwt.NewWithClaims(jwt.SigningMethodES256, validClaims())
	tokenStr, err := token.SignedString(key)
	if err != nil {
		t.Fatalf("failed to sign token: %v", err)
	}

	_, err = ValidateToken(testSecret, tokenStr)
	if err == nil {
		t.Fatal("expected error for non-HMAC signing method, got nil")
	}
}

// ---------------------------------------------------------------------------
// JWTMiddleware tests
// ---------------------------------------------------------------------------

func TestJWTMiddleware_ValidToken(t *testing.T) {
	tokenStr := makeToken(t, testSecret, validClaims())

	var capturedClaims *Claims
	handler := JWTMiddleware(testSecret)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedClaims = GetClaims(r.Context())
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+tokenStr)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if capturedClaims == nil {
		t.Fatal("expected claims in context, got nil")
	}
	if capturedClaims.UserID != "usr_123" {
		t.Errorf("UserID = %q, want %q", capturedClaims.UserID, "usr_123")
	}
	if capturedClaims.Email != "dale@dojo.com" {
		t.Errorf("Email = %q, want %q", capturedClaims.Email, "dale@dojo.com")
	}
}

func TestJWTMiddleware_MissingAuthHeader(t *testing.T) {
	handler := JWTMiddleware(testSecret)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("next handler should not be called")
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}

	var errResp httpkit.ErrorResponse
	if err := json.NewDecoder(rec.Body).Decode(&errResp); err != nil {
		t.Fatalf("failed to decode error response: %v", err)
	}
	if errResp.Code != "UNAUTHORIZED" {
		t.Errorf("Code = %q, want %q", errResp.Code, "UNAUTHORIZED")
	}
}

func TestJWTMiddleware_InvalidToken(t *testing.T) {
	handler := JWTMiddleware(testSecret)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("next handler should not be called")
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer invalid.token.here")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}

	var errResp httpkit.ErrorResponse
	if err := json.NewDecoder(rec.Body).Decode(&errResp); err != nil {
		t.Fatalf("failed to decode error response: %v", err)
	}
	if errResp.Code != "UNAUTHORIZED" {
		t.Errorf("Code = %q, want %q", errResp.Code, "UNAUTHORIZED")
	}
}

func TestJWTMiddleware_MalformedAuthHeader(t *testing.T) {
	handler := JWTMiddleware(testSecret)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("next handler should not be called")
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Token some-value")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

// ---------------------------------------------------------------------------
// Context helper tests
// ---------------------------------------------------------------------------

func TestGetClaims_NilWhenNoClaims(t *testing.T) {
	ctx := context.Background()
	if c := GetClaims(ctx); c != nil {
		t.Errorf("expected nil, got %+v", c)
	}
}

func TestGetUserID_EmptyWhenNoClaims(t *testing.T) {
	ctx := context.Background()
	if id := GetUserID(ctx); id != "" {
		t.Errorf("expected empty string, got %q", id)
	}
}

func TestGetEmail_EmptyWhenNoClaims(t *testing.T) {
	ctx := context.Background()
	if email := GetEmail(ctx); email != "" {
		t.Errorf("expected empty string, got %q", email)
	}
}

func TestGetCustomerID_EmptyWhenNoClaims(t *testing.T) {
	ctx := context.Background()
	if id := GetCustomerID(ctx); id != "" {
		t.Errorf("expected empty string, got %q", id)
	}
}

func TestGetCustomerID_FromClaims(t *testing.T) {
	c := validClaims()
	ctx := context.WithValue(context.Background(), claimsKey, &c)
	if id := GetCustomerID(ctx); id != "4" {
		t.Errorf("GetCustomerID = %q, want %q", id, "4")
	}
}

func TestGetOrgID_FromClaims(t *testing.T) {
	c := validClaims()
	ctx := context.WithValue(context.Background(), claimsKey, &c)
	if id := GetOrgID(ctx); id != "org_redlobster" {
		t.Errorf("GetOrgID = %q, want %q", id, "org_redlobster")
	}
}

func TestGetRole_FromClaims(t *testing.T) {
	c := validClaims()
	ctx := context.WithValue(context.Background(), claimsKey, &c)
	if r := GetRole(ctx); r != "org_admin" {
		t.Errorf("GetRole = %q, want %q", r, "org_admin")
	}
}
