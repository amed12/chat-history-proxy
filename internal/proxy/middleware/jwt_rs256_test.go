package middleware

import (
	"crypto/rand"
	"crypto/rsa"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func generateTestKeyPair(t *testing.T) (*rsa.PrivateKey, *rsa.PublicKey) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("failed to generate test RSA key: %v", err)
	}
	return key, &key.PublicKey
}

func signToken(t *testing.T, key *rsa.PrivateKey, claims jwt.MapClaims) string {
	t.Helper()
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	signed, err := token.SignedString(key)
	if err != nil {
		t.Fatalf("failed to sign test token: %v", err)
	}
	return signed
}

func newProtectedHandler() (http.Handler, *string) {
	var capturedUserID string
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userID, _ := UserIDFromContext(r.Context())
		capturedUserID = userID
		w.WriteHeader(http.StatusOK)
	})
	return handler, &capturedUserID
}

func TestJWTAuthRS256_ValidToken(t *testing.T) {
	privateKey, publicKey := generateTestKeyPair(t)
	inner, capturedUserID := newProtectedHandler()
	wrapped := JWTAuthRS256(publicKey)(inner)

	token := signToken(t, privateKey, jwt.MapClaims{
		"sub": "user-1",
		"exp": time.Now().Add(time.Hour).Unix(),
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	wrapped.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if *capturedUserID != "user-1" {
		t.Errorf("captured user id = %q, want %q", *capturedUserID, "user-1")
	}
}

func TestJWTAuthRS256_MissingHeader(t *testing.T) {
	_, publicKey := generateTestKeyPair(t)
	inner, _ := newProtectedHandler()
	wrapped := JWTAuthRS256(publicKey)(inner)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	wrapped.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
}

func TestJWTAuthRS256_WrongKey(t *testing.T) {
	wrongPrivateKey, _ := generateTestKeyPair(t)
	_, realPublicKey := generateTestKeyPair(t)
	inner, _ := newProtectedHandler()
	wrapped := JWTAuthRS256(realPublicKey)(inner)

	token := signToken(t, wrongPrivateKey, jwt.MapClaims{
		"sub": "user-1",
		"exp": time.Now().Add(time.Hour).Unix(),
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	wrapped.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401 for a token signed by a different key", rec.Code)
	}
}

func TestJWTAuthRS256_MissingSubClaim(t *testing.T) {
	privateKey, publicKey := generateTestKeyPair(t)
	inner, _ := newProtectedHandler()
	wrapped := JWTAuthRS256(publicKey)(inner)

	token := signToken(t, privateKey, jwt.MapClaims{
		"exp": time.Now().Add(time.Hour).Unix(),
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	wrapped.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401 when sub claim is missing", rec.Code)
	}
}
