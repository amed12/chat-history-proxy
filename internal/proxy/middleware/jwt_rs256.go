// Package middleware holds HTTP middleware specific to this proxy: RS256
// JWT verification, where the public key is supplied by the client's own
// auth backend via the JWT_PUBLIC_KEY environment variable (see
// internal/proxy/config). internal/middleware (the repo-level package) holds
// generic middleware with no proxy-specific logic.
package middleware

import (
	"context"
	"crypto/rsa"
	"net/http"
	"strings"

	"github.com/golang-jwt/jwt/v5"
)

type contextKey string

// UserIDKey is the context key the verified JWT's `sub` claim is stored
// under.
const UserIDKey contextKey = "chat_history_user_id"

// JWTAuthRS256 returns middleware that validates a Bearer token signed with
// RS256 against publicKey, and injects its `sub` claim into the request
// context. This is the identity the rest of the proxy trusts — handlers must
// read the user id from the context, never from a request parameter.
func JWTAuthRS256(publicKey *rsa.PublicKey) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if !strings.HasPrefix(authHeader, "Bearer ") {
				http.Error(w, "missing or invalid token", http.StatusUnauthorized)
				return
			}

			tokenStr := strings.TrimPrefix(authHeader, "Bearer ")
			token, err := jwt.Parse(tokenStr, func(t *jwt.Token) (interface{}, error) {
				if _, ok := t.Method.(*jwt.SigningMethodRSA); !ok {
					return nil, jwt.ErrSignatureInvalid
				}
				return publicKey, nil
			})
			if err != nil || !token.Valid {
				http.Error(w, "invalid token", http.StatusUnauthorized)
				return
			}

			claims, ok := token.Claims.(jwt.MapClaims)
			if !ok {
				http.Error(w, "invalid token claims", http.StatusUnauthorized)
				return
			}

			userID, ok := claims["sub"].(string)
			if !ok || userID == "" {
				http.Error(w, "token missing sub claim", http.StatusUnauthorized)
				return
			}

			ctx := context.WithValue(r.Context(), UserIDKey, userID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// UserIDFromContext returns the authenticated user id injected by
// JWTAuthRS256, and whether it was present.
func UserIDFromContext(ctx context.Context) (string, bool) {
	userID, ok := ctx.Value(UserIDKey).(string)
	return userID, ok
}
