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
