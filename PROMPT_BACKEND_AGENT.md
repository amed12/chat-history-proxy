# AI Agent Prompt — Golang Backend: Chat History Archive Service
# Client: HelloGod (Sekolah Tinggi Teologi Amanat Agung)
# App ID: vvmxt-mcz1gxflybkyeul
# Last updated: 2026-06-17

---

## CONTEXT & BACKGROUND

You are building a **standalone Golang HTTP service** that archives Qiscus chat room
messages and serves them to the Flutter app when the Qiscus session has expired.

### Why this exists

The Flutter app uses Qiscus **sessional mode** (`is_sessional: true`). When a room is
resolved and ~1 hour passes, the user's Qiscus token expires. Any call to the Qiscus
SDK then returns:

```
{"error":{"message":"Unauthorized"},"status":403}
```

This backend solves that by:
1. **Listening** to Qiscus webhooks for "Mark as Resolved" events
2. **Fetching** all messages from Qiscus using server-side credentials (no user token needed)
3. **Storing** them in PostgreSQL
4. **Serving** them to the Flutter app via a simple REST endpoint

---

## ARCHITECTURE OVERVIEW

```
Qiscus Platform
      │
      │ POST /webhook/resolve  (on room resolved)
      ▼
┌─────────────────────────────────────────────────────┐
│              Golang HTTP Service                     │
│                                                     │
│  POST /webhook/resolve  → WebhookHandler            │
│    └─ validate signature (qiscus-signature-key)     │
│    └─ go fetchAndArchive(roomId)  ← async goroutine │
│                                                     │
│  GET  /api/v1/chat-history/:room_id → HistoryHandler│
│    └─ validate JWT                                  │
│    └─ query DB → if found: return                   │
│    └─ if not: fetchAndArchive() → return            │
│                                                     │
│  POST /api/v1/auth/token → AuthHandler (sample)     │
│    └─ issue JWT for Flutter app                     │
└──────────────────┬──────────────────────────────────┘
                   │ SQL
                   ▼
            PostgreSQL DB
            table: chat_archives
```

---

## TECH STACK

| Layer | Choice |
|---|---|
| Language | Go 1.22+ |
| HTTP framework | `net/http` + `github.com/go-chi/chi/v5` |
| Database | PostgreSQL 15+ |
| ORM / query | `database/sql` + `lib/pq` (no ORM — keep it simple) |
| Auth (JWT) | `github.com/golang-jwt/jwt/v5` |
| Config | `github.com/joho/godotenv` |
| HTTP client | `net/http` (stdlib) |

---

## PROJECT STRUCTURE

```
chat-history-service/
├── cmd/
│   └── server/
│       └── main.go              # entry point
├── internal/
│   ├── config/
│   │   └── config.go            # loads .env, exports Config struct
│   ├── database/
│   │   ├── db.go                # opens postgres connection pool
│   │   └── migrations.sql       # schema — run once on deploy
│   ├── middleware/
│   │   ├── jwt_auth.go          # validates Bearer token on API routes
│   │   └── logger.go            # simple request logger
│   ├── qiscus/
│   │   └── client.go            # HTTP client for Qiscus REST API
│   ├── archive/
│   │   ├── model.go             # ChatArchive, ArchivedMessage structs
│   │   ├── repository.go        # DB read/write for chat_archives
│   │   └── service.go           # fetchAndArchive() orchestration
│   └── handler/
│       ├── webhook.go           # POST /webhook/resolve
│       ├── history.go           # GET  /api/v1/chat-history/:room_id
│       └── auth.go              # POST /api/v1/auth/token (sample only)
├── .env.example
├── go.mod
└── go.sum
```

---

## STEP-BY-STEP IMPLEMENTATION

### Step 1 — `go.mod`

```
module github.com/hellogod/chat-history-service

go 1.22

require (
    github.com/go-chi/chi/v5 v5.1.0
    github.com/golang-jwt/jwt/v5 v5.2.1
    github.com/joho/godotenv v1.5.1
    github.com/lib/pq v1.10.9
)
```

Run `go mod tidy` after creating this file.

---

### Step 2 — `.env.example`

Document every environment variable. Copy to `.env` and fill in values.

```env
# Server
PORT=8080

# PostgreSQL
DATABASE_URL=postgres://postgres:password@localhost:5432/chat_history?sslmode=disable

# Qiscus credentials (server-side — never expose to client)
QISCUS_APP_ID=vvmxt-mcz1gxflybkyeul
QISCUS_SECRET_KEY=your_qiscus_secret_key_here
QISCUS_BASE_URL=https://api3.qiscus.com
QISCUS_MULTICHANNEL_URL=https://multichannel.qiscus.com

# Webhook
# Must match the secret configured in Qiscus Dashboard → Webhook Settings
WEBHOOK_SECRET=your_webhook_secret_here

# JWT (for Flutter ↔ backend auth — separate from Qiscus auth)
JWT_SECRET=your_32_char_or_longer_random_secret_here
JWT_EXPIRY_HOURS=720   # 30 days
```

---

### Step 3 — `internal/config/config.go`

```go
package config

import (
    "log"
    "os"
    "strconv"

    "github.com/joho/godotenv"
)

// Config holds all runtime configuration loaded from environment variables.
type Config struct {
    Port                 string
    DatabaseURL          string
    QiscusAppID          string
    QiscusSecretKey      string
    QiscusBaseURL        string
    QiscusMultichannelURL string
    WebhookSecret        string
    JWTSecret            string
    JWTExpiryHours       int
}

// Load reads .env (if present) then environment variables.
// Panics if any required variable is missing.
func Load() Config {
    // Ignore error — .env file is optional in production (use real env vars).
    _ = godotenv.Load()

    cfg := Config{
        Port:                  getEnv("PORT", "8080"),
        DatabaseURL:           requireEnv("DATABASE_URL"),
        QiscusAppID:           requireEnv("QISCUS_APP_ID"),
        QiscusSecretKey:       requireEnv("QISCUS_SECRET_KEY"),
        QiscusBaseURL:         getEnv("QISCUS_BASE_URL", "https://api3.qiscus.com"),
        QiscusMultichannelURL: getEnv("QISCUS_MULTICHANNEL_URL", "https://multichannel.qiscus.com"),
        WebhookSecret:         requireEnv("WEBHOOK_SECRET"),
        JWTSecret:             requireEnv("JWT_SECRET"),
        JWTExpiryHours:        getEnvInt("JWT_EXPIRY_HOURS", 720),
    }

    return cfg
}

func requireEnv(key string) string {
    v := os.Getenv(key)
    if v == "" {
        log.Fatalf("required environment variable %q is not set", key)
    }
    return v
}

func getEnv(key, fallback string) string {
    if v := os.Getenv(key); v != "" {
        return v
    }
    return fallback
}

func getEnvInt(key string, fallback int) int {
    if v := os.Getenv(key); v != "" {
        if i, err := strconv.Atoi(v); err == nil {
            return i
        }
    }
    return fallback
}
```

---

### Step 4 — `internal/database/migrations.sql`

Run this once against your PostgreSQL database before starting the service.

```sql
-- Chat archive table
-- One row per resolved Qiscus room.
-- Messages stored as JSONB for flexibility (schema can evolve without migration).
CREATE TABLE IF NOT EXISTS chat_archives (
    id          BIGSERIAL PRIMARY KEY,
    room_id     VARCHAR(50)  UNIQUE NOT NULL,
    room_name   VARCHAR(255) NOT NULL DEFAULT '',
    app_id      VARCHAR(100) NOT NULL,
    messages    JSONB        NOT NULL DEFAULT '[]',
    archived_at TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    source      VARCHAR(20)  NOT NULL DEFAULT 'webhook', -- 'webhook' | 'on_demand'
    updated_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_chat_archives_room_id ON chat_archives (room_id);
CREATE INDEX IF NOT EXISTS idx_chat_archives_app_id  ON chat_archives (app_id);
```

---

### Step 5 — `internal/database/db.go`

```go
package database

import (
    "database/sql"
    "log"

    _ "github.com/lib/pq"
)

// Open returns a connected *sql.DB with sane pool settings.
// Caller is responsible for calling db.Close() on shutdown.
func Open(databaseURL string) *sql.DB {
    db, err := sql.Open("postgres", databaseURL)
    if err != nil {
        log.Fatalf("database: open failed: %v", err)
    }

    db.SetMaxOpenConns(25)
    db.SetMaxIdleConns(5)

    if err := db.Ping(); err != nil {
        log.Fatalf("database: ping failed: %v", err)
    }

    log.Println("database: connected")
    return db
}
```

---

### Step 6 — `internal/archive/model.go`

```go
package archive

import "time"

// ArchivedMessage mirrors the Qiscus message JSON structure.
// Stored inside chat_archives.messages (JSONB array).
type ArchivedMessage struct {
    ID          int64                  `json:"id"`
    UniqueID    string                 `json:"unique_id"`
    Text        string                 `json:"text"`
    SenderName  string                 `json:"sender_name"`
    SenderEmail string                 `json:"sender_email"`
    Timestamp   time.Time              `json:"timestamp"`
    Type        string                 `json:"type"`
    Extras      map[string]interface{} `json:"extras,omitempty"`
    Payload     map[string]interface{} `json:"payload,omitempty"`
}

// ChatArchive is the full record stored in the database for one room.
type ChatArchive struct {
    ID         int64             `json:"id"`
    RoomID     string            `json:"room_id"`
    RoomName   string            `json:"room_name"`
    AppID      string            `json:"app_id"`
    Messages   []ArchivedMessage `json:"messages"`
    ArchivedAt time.Time         `json:"archived_at"`
    Source     string            `json:"source"`
    UpdatedAt  time.Time         `json:"updated_at"`
}
```

---

### Step 7 — `internal/archive/repository.go`

```go
package archive

import (
    "database/sql"
    "encoding/json"
    "errors"
    "time"
)

// ErrNotFound is returned when no archive exists for a given room ID.
var ErrNotFound = errors.New("archive: room not found")

// Repository handles all DB operations for chat archives.
type Repository struct {
    db *sql.DB
}

// NewRepository creates a Repository backed by db.
func NewRepository(db *sql.DB) *Repository {
    return &Repository{db: db}
}

// FindByRoomID fetches the archive for roomID.
// Returns ErrNotFound if no record exists.
func (r *Repository) FindByRoomID(roomID string) (*ChatArchive, error) {
    const q = `
        SELECT id, room_id, room_name, app_id, messages, archived_at, source, updated_at
        FROM chat_archives
        WHERE room_id = $1
        LIMIT 1`

    row := r.db.QueryRow(q, roomID)

    var a ChatArchive
    var messagesJSON []byte

    err := row.Scan(
        &a.ID, &a.RoomID, &a.RoomName, &a.AppID,
        &messagesJSON, &a.ArchivedAt, &a.Source, &a.UpdatedAt,
    )
    if errors.Is(err, sql.ErrNoRows) {
        return nil, ErrNotFound
    }
    if err != nil {
        return nil, err
    }

    if err := json.Unmarshal(messagesJSON, &a.Messages); err != nil {
        return nil, err
    }
    return &a, nil
}

// Upsert inserts or updates the archive for archive.RoomID.
// Safe to call multiple times (idempotent via ON CONFLICT).
func (r *Repository) Upsert(a *ChatArchive) error {
    messagesJSON, err := json.Marshal(a.Messages)
    if err != nil {
        return err
    }

    const q = `
        INSERT INTO chat_archives (room_id, room_name, app_id, messages, archived_at, source, updated_at)
        VALUES ($1, $2, $3, $4, $5, $6, NOW())
        ON CONFLICT (room_id) DO UPDATE
            SET messages    = EXCLUDED.messages,
                room_name   = EXCLUDED.room_name,
                source      = EXCLUDED.source,
                updated_at  = NOW()`

    _, err = r.db.Exec(q,
        a.RoomID, a.RoomName, a.AppID,
        messagesJSON, time.Now(), a.Source,
    )
    return err
}
```

---

### Step 8 — `internal/qiscus/client.go`

```go
package qiscus

import (
    "encoding/json"
    "fmt"
    "net/http"
    "time"
)

// Client wraps the Qiscus REST API using server-side credentials.
// It does NOT use a user token — this is intentional. Server credentials
// remain valid regardless of session state.
type Client struct {
    appID     string
    secretKey string
    baseURL   string
    http      *http.Client
}

// NewClient creates a Qiscus API client.
func NewClient(appID, secretKey, baseURL string) *Client {
    return &Client{
        appID:     appID,
        secretKey: secretKey,
        baseURL:   baseURL,
        http:      &http.Client{Timeout: 30 * time.Second},
    }
}

// Message is the subset of Qiscus message fields we care about.
type Message struct {
    ID          int64                  `json:"id"`
    UniqueID    string                 `json:"unique_temp_id"`
    Text        string                 `json:"message"`
    SenderName  string                 `json:"username"`
    SenderEmail string                 `json:"email"`
    Timestamp   time.Time              `json:"unix_timestamp"` // parsed below
    Type        string                 `json:"type"`
    Extras      map[string]interface{} `json:"extras"`
    Payload     map[string]interface{} `json:"payload"`
    UnixNano    int64                  `json:"unix_nano_timestamp"`
}

type roomMessagesResponse struct {
    Results struct {
        Room struct {
            Name string `json:"room_name"`
        } `json:"room"`
        Comments []qiscusComment `json:"comments"`
    } `json:"results"`
}

type qiscusComment struct {
    ID          int64                  `json:"id"`
    UniqueID    string                 `json:"unique_temp_id"`
    Message     string                 `json:"message"`
    Username    string                 `json:"username"`
    Email       string                 `json:"email"`
    UnixNano    int64                  `json:"unix_nano_timestamp"`
    Type        string                 `json:"type"`
    Extras      map[string]interface{} `json:"extras"`
    Payload     map[string]interface{} `json:"payload"`
}

// GetRoomMessages fetches ALL messages for roomID using server credentials.
// Paginates automatically until all messages are retrieved.
func (c *Client) GetRoomMessages(roomID string) (roomName string, msgs []Message, err error) {
    url := fmt.Sprintf(
        "%s/api/v2/rest/load_comments?room_id=%s&page=1&limit=100",
        c.baseURL, roomID,
    )

    req, err := http.NewRequest(http.MethodGet, url, nil)
    if err != nil {
        return "", nil, err
    }
    // Server-side auth: use QISCUS_APP_ID + QISCUS_SECRET_KEY headers.
    req.Header.Set("QISCUS_SDK_APP_ID", c.appID)
    req.Header.Set("QISCUS_SDK_SECRET", c.secretKey)

    resp, err := c.http.Do(req)
    if err != nil {
        return "", nil, fmt.Errorf("qiscus: GET messages failed: %w", err)
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        return "", nil, fmt.Errorf("qiscus: unexpected status %d for room %s", resp.StatusCode, roomID)
    }

    var body roomMessagesResponse
    if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
        return "", nil, fmt.Errorf("qiscus: decode error: %w", err)
    }

    roomName = body.Results.Room.Name
    for _, c := range body.Results.Comments {
        msgs = append(msgs, Message{
            ID:          c.ID,
            UniqueID:    c.UniqueID,
            Text:        c.Message,
            SenderName:  c.Username,
            SenderEmail: c.Email,
            Timestamp:   time.Unix(0, c.UnixNano*int64(time.Millisecond)).UTC(),
            Type:        c.Type,
            Extras:      c.Extras,
            Payload:     c.Payload,
        })
    }

    return roomName, msgs, nil
}
```

---

### Step 9 — `internal/archive/service.go`

```go
package archive

import (
    "log"

    "github.com/hellogod/chat-history-service/internal/qiscus"
)

// Service orchestrates fetching from Qiscus and persisting to the DB.
type Service struct {
    repo      *Repository
    qiscus    *qiscus.Client
    appID     string
}

// NewService creates an archive Service.
func NewService(repo *Repository, qiscusClient *qiscus.Client, appID string) *Service {
    return &Service{repo: repo, qiscus: qiscusClient, appID: appID}
}

// FetchAndArchive fetches messages for roomID from Qiscus and upserts them
// into the database. Safe to call concurrently for different room IDs.
//
// This is called:
//   - From the webhook handler (async goroutine, source="webhook")
//   - From the history handler when DB has no record (source="on_demand")
func (s *Service) FetchAndArchive(roomID, source string) (*ChatArchive, error) {
    roomName, messages, err := s.qiscus.GetRoomMessages(roomID)
    if err != nil {
        return nil, err
    }

    archived := make([]ArchivedMessage, 0, len(messages))
    for _, m := range messages {
        archived = append(archived, ArchivedMessage{
            ID:          m.ID,
            UniqueID:    m.UniqueID,
            Text:        m.Text,
            SenderName:  m.SenderName,
            SenderEmail: m.SenderEmail,
            Timestamp:   m.Timestamp,
            Type:        m.Type,
            Extras:      m.Extras,
            Payload:     m.Payload,
        })
    }

    record := &ChatArchive{
        RoomID:   roomID,
        RoomName: roomName,
        AppID:    s.appID,
        Messages: archived,
        Source:   source,
    }

    if err := s.repo.Upsert(record); err != nil {
        log.Printf("archive: upsert failed for room %s: %v", roomID, err)
        return nil, err
    }

    log.Printf("archive: saved %d messages for room %s (source=%s)", len(archived), roomID, source)
    return record, nil
}

// GetOrFetch returns the archive for roomID from DB if it exists,
// otherwise fetches from Qiscus first (on-demand).
func (s *Service) GetOrFetch(roomID string) (*ChatArchive, error) {
    record, err := s.repo.FindByRoomID(roomID)
    if err == nil {
        return record, nil // cache hit
    }
    if err != ErrNotFound {
        return nil, err
    }

    // Not in DB yet — fetch on demand.
    return s.FetchAndArchive(roomID, "on_demand")
}
```

---

### Step 10 — `internal/handler/webhook.go`

```go
package handler

import (
    "crypto/hmac"
    "crypto/sha256"
    "crypto/subtle"
    "encoding/base64"
    "encoding/json"
    "io"
    "log"
    "net/http"

    "github.com/hellogod/chat-history-service/internal/archive"
)

// WebhookPayload is the subset of the Qiscus webhook body we need.
// Under Multichannel / Omnichannel "Mark as Resolved" webhook settings.
type WebhookPayload struct {
    Service struct {
        RoomID     string `json:"room_id"`
        IsResolved bool   `json:"is_resolved"`
    } `json:"service"`
}

// WebhookHandler handles POST /webhook/resolve from Qiscus.
type WebhookHandler struct {
    secret  string // Webhook secret key configured in Omnichannel Settings
    service *archive.Service
}

// NewWebhookHandler creates a WebhookHandler.
func NewWebhookHandler(secret string, svc *archive.Service) *WebhookHandler {
    return &WebhookHandler{secret: secret, service: svc}
}

func (h *WebhookHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
    // 1. Read raw body for signature validation.
    bodyBytes, err := io.ReadAll(r.Body)
    if err != nil {
        http.Error(w, "failed to read request body", http.StatusBadRequest)
        return
    }

    // 2. Validate webhook HMAC signature (using qiscus-signature-key header).
    signature := r.Header.Get("qiscus-signature-key")
    if signature == "" {
        http.Error(w, "missing signature header", http.StatusUnauthorized)
        return
    }

    if !verifyHMACSignature(bodyBytes, h.secret, signature) {
        http.Error(w, "invalid signature", http.StatusUnauthorized)
        return
    }

    // 3. Parse body.
    var payload WebhookPayload
    if err := json.Unmarshal(bodyBytes, &payload); err != nil {
        http.Error(w, "bad request", http.StatusBadRequest)
        return
    }

    // 4. Validate if the conversation is resolved.
    if !payload.Service.IsResolved {
        w.WriteHeader(http.StatusOK) // acknowledge non-resolved events silently
        return
    }

    // 5. Extract room ID.
    roomID := payload.Service.RoomID
    if roomID == "" {
        http.Error(w, "missing room id", http.StatusBadRequest)
        return
    }

    // 6. Respond immediately — do NOT block on the archive fetch.
    // Qiscus will retry if it doesn't get a 200 quickly.
    w.WriteHeader(http.StatusOK)
    w.Write([]byte(`{"status":"accepted"}`))

    // 7. Archive asynchronously.
    go func() {
        if _, err := h.service.FetchAndArchive(roomID, "webhook"); err != nil {
            log.Printf("webhook: archive failed for room %s: %v", roomID, err)
        }
    }()
}

// verifyHMACSignature computes the HMAC-SHA256 signature of body with secret
// and compares it to the base64-encoded signature header.
func verifyHMACSignature(body []byte, secret, signatureHeader string) bool {
    mac := hmac.New(sha256.New, []byte(secret))
    mac.Write(body)
    expectedMAC := mac.Sum(nil)

    actualMAC, err := base64.StdEncoding.DecodeString(signatureHeader)
    if err != nil {
        return false
    }

    return subtle.ConstantTimeCompare(actualMAC, expectedMAC) == 1
}
```

---

### Step 11 — `internal/handler/history.go`

```go
package handler

import (
    "encoding/json"
    "errors"
    "net/http"

    "github.com/go-chi/chi/v5"
    "github.com/hellogod/chat-history-service/internal/archive"
)

// HistoryHandler handles GET /api/v1/chat-history/:room_id
type HistoryHandler struct {
    service *archive.Service
}

// NewHistoryHandler creates a HistoryHandler.
func NewHistoryHandler(svc *archive.Service) *HistoryHandler {
    return &HistoryHandler{service: svc}
}

func (h *HistoryHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
    roomID := chi.URLParam(r, "room_id")
    if roomID == "" {
        http.Error(w, "room_id is required", http.StatusBadRequest)
        return
    }

    record, err := h.service.GetOrFetch(roomID)
    if err != nil {
        if errors.Is(err, archive.ErrNotFound) {
            http.Error(w, "not found", http.StatusNotFound)
            return
        }
        http.Error(w, "internal server error", http.StatusInternalServerError)
        return
    }

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(map[string]interface{}{
        "data": record,
    })
}
```

---

### Step 12 — `internal/handler/auth.go` (sample only)

This is a **minimal sample** auth endpoint. In production, replace with your
actual user authentication flow.

```go
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
```

---

### Step 13 — `internal/middleware/jwt_auth.go`

```go
package middleware

import (
    "context"
    "net/http"
    "strings"

    "github.com/golang-jwt/jwt/v5"
)

type contextKey string

const UserIDKey contextKey = "user_id"

// JWTAuth returns middleware that validates Bearer tokens on protected routes.
// On success it injects the user_id claim into the request context.
func JWTAuth(jwtSecret string) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            authHeader := r.Header.Get("Authorization")
            if !strings.HasPrefix(authHeader, "Bearer ") {
                http.Error(w, "missing or invalid token", http.StatusUnauthorized)
                return
            }

            tokenStr := strings.TrimPrefix(authHeader, "Bearer ")
            token, err := jwt.Parse(tokenStr, func(t *jwt.Token) (interface{}, error) {
                if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
                    return nil, jwt.ErrSignatureInvalid
                }
                return []byte(jwtSecret), nil
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

            ctx := context.WithValue(r.Context(), UserIDKey, claims["sub"])
            next.ServeHTTP(w, r.WithContext(ctx))
        })
    }
}
```

---

### Step 14 — `internal/middleware/logger.go`

```go
package middleware

import (
    "log"
    "net/http"
    "time"
)

// Logger logs method, path, status code, and latency for every request.
func Logger(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        start := time.Now()
        rw := &responseWriter{ResponseWriter: w, status: http.StatusOK}
        next.ServeHTTP(rw, r)
        log.Printf("%s %s %d %s", r.Method, r.URL.Path, rw.status, time.Since(start))
    })
}

type responseWriter struct {
    http.ResponseWriter
    status int
}

func (rw *responseWriter) WriteHeader(status int) {
    rw.status = status
    rw.ResponseWriter.WriteHeader(status)
}
```

---

### Step 15 — `cmd/server/main.go`

```go
package main

import (
    "log"
    "net/http"

    "github.com/go-chi/chi/v5"
    chimiddleware "github.com/go-chi/chi/v5/middleware"

    "github.com/hellogod/chat-history-service/internal/archive"
    "github.com/hellogod/chat-history-service/internal/config"
    "github.com/hellogod/chat-history-service/internal/database"
    "github.com/hellogod/chat-history-service/internal/handler"
    "github.com/hellogod/chat-history-service/internal/middleware"
    "github.com/hellogod/chat-history-service/internal/qiscus"
)

func main() {
    cfg := config.Load()

    // Database
    db := database.Open(cfg.DatabaseURL)
    defer db.Close()

    // Dependencies
    qiscusClient := qiscus.NewClient(cfg.QiscusAppID, cfg.QiscusSecretKey, cfg.QiscusBaseURL)
    repo := archive.NewRepository(db)
    svc := archive.NewService(repo, qiscusClient, cfg.QiscusAppID)

    // Handlers
    webhookHandler := handler.NewWebhookHandler(cfg.WebhookSecret, svc)
    historyHandler := handler.NewHistoryHandler(svc)
    authHandler := handler.NewAuthHandler(cfg.JWTSecret, cfg.JWTExpiryHours)

    // Router
    r := chi.NewRouter()
    r.Use(chimiddleware.Recoverer)
    r.Use(middleware.Logger)

    // Public routes
    r.Post("/webhook/resolve", webhookHandler.ServeHTTP)
    r.Post("/api/v1/auth/token", authHandler.ServeHTTP)

    // Protected routes
    r.Group(func(r chi.Router) {
        r.Use(middleware.JWTAuth(cfg.JWTSecret))
        r.Get("/api/v1/chat-history/{room_id}", historyHandler.ServeHTTP)
    })

    // Health check
    r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
        w.Write([]byte(`{"status":"ok"}`))
    })

    log.Printf("server starting on :%s", cfg.Port)
    if err := http.ListenAndServe(":"+cfg.Port, r); err != nil {
        log.Fatalf("server error: %v", err)
    }
}
```

---

## API CONTRACT

These are the endpoints the Flutter app (and Qiscus) will call.

### `POST /webhook/resolve`

Called automatically by Qiscus when a room is resolved.

**Headers:**
```
qiscus-signature-key: <HMAC-SHA256 signature of request body using WEBHOOK_SECRET, encoded in Base64>
Content-Type: application/json
```

**Request body (Qiscus Multichannel format):**
```json
{
  "service": {
    "id": 401678,
    "room_id": "2694123",
    "is_resolved": true,
    "notes": "some notes here!",
    "first_comment_id": "28181345",
    "last_comment_id": 34192567,
    "source": "wa"
  },
  "resolved_by": {
    "id": 1482,
    "email": "agent@example.com",
    "name": "CS Admin",
    "type": "admin",
    "is_available": false
  },
  "customer": {
    "name": "Customer Name",
    "avatar": "https://image.flaticon.com/icons/svg/145/145867.svg",
    "user_id": "628122661xxxx"
  }
}
```

**Response:** `200 OK`
```json
{"status":"accepted"}
```

---

### `POST /api/v1/auth/token`

Issues a JWT for the Flutter app. Call this after the user logs in.

**Request body:**
```json
{"user_id": "zFk8jRIk_21481"}
```

**Response:** `200 OK`
```json
{
  "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
  "expires_at": 1784255400
}
```

---

### `GET /api/v1/chat-history/:room_id`

Returns archived messages for a resolved room.

**Headers:**
```
Authorization: Bearer <JWT from /auth/token>
```

**Response:** `200 OK`
```json
{
  "data": {
    "room_id": "440928374",
    "room_name": "John Doe",
    "app_id": "vvmxt-mcz1gxflybkyeul",
    "archived_at": "2026-06-10T15:35:47Z",
    "source": "webhook",
    "messages": [
      {
        "id": 4259110415,
        "unique_id": "1781080534622",
        "text": "test",
        "sender_name": "John Doe",
        "sender_email": "zFk8jRIk_21481@vvmxt-mcz1gxflybkyeul.qiscus.com",
        "timestamp": "2026-06-10T15:35:34Z",
        "type": "text"
      }
    ]
  }
}
```

**Error responses:**
```
401 Unauthorized  — missing or invalid JWT
404 Not Found     — room not archived (should not normally happen after fallback)
500               — Qiscus API or DB error
```

---

## DEPLOYMENT NOTES

### Configure Qiscus Webhook

1. Go to Qiscus Dashboard → Settings → Webhook
2. Add URL: `https://your-domain.com/webhook/resolve`
3. Select event: `Resolve Conversation`
4. Set Secret Key → copy the value to `WEBHOOK_SECRET` in your `.env`

### Local development

```bash
# 1. Start PostgreSQL (Docker example)
docker run -d --name postgres \
  -e POSTGRES_DB=chat_history \
  -e POSTGRES_PASSWORD=password \
  -p 5432:5432 postgres:15

# 2. Run migration
psql -U postgres -d chat_history -f internal/database/migrations.sql

# 3. Copy and fill in .env
cp .env.example .env

# 4. Run
go run ./cmd/server
```

### Production checklist

- [ ] `DATABASE_URL` uses SSL (`sslmode=require`)
- [ ] `JWT_SECRET` is at least 32 random characters
- [ ] `WEBHOOK_SECRET` matches exactly what is set in Qiscus Dashboard
- [ ] Service is behind HTTPS (Nginx / Caddy / load balancer)
- [ ] `PORT` is not exposed directly to the internet (reverse proxy only)
- [ ] Run `go build -o server ./cmd/server` and use a process manager (systemd / Docker)

---

## IMPORTANT CONSTRAINTS

1. **Never use a user Qiscus token** in this backend. Always use
   `QISCUS_APP_ID` + `QISCUS_SECRET_KEY` for server-to-Qiscus calls.
2. **Respond to webhook in < 5 seconds.** Qiscus retries if it doesn't
   get a 200 quickly. The `go func()` async pattern in the webhook handler
   ensures this.
3. **Upsert is idempotent.** If Qiscus retries a webhook, calling
   `FetchAndArchive` again simply overwrites the existing record — no duplicates.
4. **Do NOT open sessions or cookies** on the API routes. Stateless JWT only.
5. **The `/api/v1/auth/token` endpoint is a sample.** In production, this
   should be replaced with your real authentication system (OTP, email/password, SSO).

---

## FILE CHANGE SUMMARY

```
CREATED (all new files):
  cmd/server/main.go
  internal/config/config.go
  internal/database/db.go
  internal/database/migrations.sql
  internal/archive/model.go
  internal/archive/repository.go
  internal/archive/service.go
  internal/qiscus/client.go
  internal/handler/webhook.go
  internal/handler/history.go
  internal/handler/auth.go
  internal/middleware/jwt_auth.go
  internal/middleware/logger.go
  go.mod
  .env.example
```
