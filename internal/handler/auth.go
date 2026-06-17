package handler

import (
    "encoding/json"
    "net/http"
    "time"

    "github.com/golang-jwt/jwt/v5"
)

// AuthHandler handles POST /api/v1/auth/token
// Sample only — issues a JWT for any userId/password combination.
// In production: validate against your user database.
type AuthHandler struct {
    jwtSecret      string
    jwtExpiryHours int
}

// NewAuthHandler creates an AuthHandler.
func NewAuthHandler(jwtSecret string, jwtExpiryHours int) *AuthHandler {
    return &AuthHandler{jwtSecret: jwtSecret, jwtExpiryHours: jwtExpiryHours}
}

type authRequest struct {
    UserID string `json:"user_id"`
}

type authResponse struct {
    Token     string `json:"token"`
    ExpiresAt int64  `json:"expires_at"`
}

func (h *AuthHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
    var req authRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.UserID == "" {
        http.Error(w, "user_id is required", http.StatusBadRequest)
        return
    }

    expiresAt := time.Now().Add(time.Duration(h.jwtExpiryHours) * time.Hour)

    claims := jwt.MapClaims{
        "sub": req.UserID,
        "exp": expiresAt.Unix(),
        "iat": time.Now().Unix(),
    }
    token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
    signed, err := token.SignedString([]byte(h.jwtSecret))
    if err != nil {
        http.Error(w, "token generation failed", http.StatusInternalServerError)
        return
    }

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(authResponse{Token: signed, ExpiresAt: expiresAt.Unix()})
}
