package handler

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	megamiddleware "github.com/qiscus-community/chat-history-proxy/internal/proxy/middleware"
	"github.com/qiscus-community/chat-history-proxy/internal/proxy/qiscus"
)

// MessageFetcher fetches every message in a room. Satisfied by
// *qiscus.Client and by test doubles.
type MessageFetcher interface {
	GetRoomMessages(roomID string) ([]qiscus.Message, error)
}

// MessagesHandler serves GET /api/v1/sessions/{room_id}/messages.
//
// It MUST verify that room_id belongs to the authenticated user (via
// SessionsHandler-style lookup) before returning anything — load_comments
// itself does not check ownership, so skipping this check would let any
// authenticated user read any other user's transcript by guessing a
// room_id. See docs/CHAT_HISTORY_PROXY.md §6/§7 (AC4).
type MessagesHandler struct {
	Lister  SessionLister
	Cache   SessionCache
	Fetcher MessageFetcher
}

type messagesResponse struct {
	Data       []qiscus.Message `json:"data"`
	NextCursor *string          `json:"next_cursor"`
}

func (h *MessagesHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	userID, ok := megamiddleware.UserIDFromContext(r.Context())
	if !ok || userID == "" {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	roomID := chi.URLParam(r, "room_id")
	if roomID == "" {
		http.Error(w, "room_id is required", http.StatusBadRequest)
		return
	}

	sessionsHandler := &SessionsHandler{Lister: h.Lister, Cache: h.Cache}
	sessions, err := sessionsHandler.getSessions(userID, false)
	if err != nil {
		writeUpstreamError(w, "failed to verify room ownership", err)
		return
	}

	if !ownsRoom(sessions, roomID) {
		// 404, not 403: don't reveal that a room with this id exists at all.
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	messages, err := h.Fetcher.GetRoomMessages(roomID)
	if err != nil {
		writeUpstreamError(w, "failed to fetch messages", err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(messagesResponse{Data: messages, NextCursor: nil})
}

func ownsRoom(sessions []qiscus.Session, roomID string) bool {
	for _, s := range sessions {
		if s.RoomID == roomID {
			return true
		}
	}
	return false
}
