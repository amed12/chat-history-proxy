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
