// Package handler implements this proxy's HTTP endpoints:
// GET /api/v1/sessions and GET /api/v1/sessions/{room_id}/messages.
package handler

import (
	"encoding/json"
	"net/http"

	megamiddleware "github.com/qiscus-community/chat-history-proxy/internal/proxy/middleware"
	"github.com/qiscus-community/chat-history-proxy/internal/proxy/qiscus"
)

// SessionLister fetches every session (room) belonging to a user.
// Satisfied by *qiscus.Client and by test doubles.
type SessionLister interface {
	GetUserRooms(userID string) ([]qiscus.Session, error)
}

// SessionCache caches a user's session list to avoid calling Qiscus on
// every request. Satisfied by *cache.TTLCache[[]qiscus.Session].
type SessionCache interface {
	Get(key string) ([]qiscus.Session, bool)
	Set(key string, value []qiscus.Session)
}

// SessionsHandler serves GET /api/v1/sessions.
type SessionsHandler struct {
	Lister SessionLister
	Cache  SessionCache
}

type sessionsResponse struct {
	Data       []qiscus.Session `json:"data"`
	NextCursor *string          `json:"next_cursor"`
}

func (h *SessionsHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	userID, ok := megamiddleware.UserIDFromContext(r.Context())
	if !ok || userID == "" {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	// ?fresh=1 skips the cache read (but still repopulates it) — the app
	// asks for this right after creating a new session, since the cached
	// list from before that room existed would otherwise be served for up
	// to CACHE_TTL_SECONDS, making the just-created room appear missing.
	bypassCache := r.URL.Query().Get("fresh") == "1"

	sessions, err := h.getSessions(userID, bypassCache)
	if err != nil {
		writeUpstreamError(w, "failed to fetch sessions", err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(sessionsResponse{Data: sessions, NextCursor: nil})
}

// getSessions returns userID's sessions from cache if present and
// bypassCache is false, otherwise fetches from Qiscus and (re)populates the
// cache.
func (h *SessionsHandler) getSessions(userID string, bypassCache bool) ([]qiscus.Session, error) {
	if h.Cache != nil && !bypassCache {
		if cached, ok := h.Cache.Get(userID); ok {
			return cached, nil
		}
	}

	sessions, err := h.Lister.GetUserRooms(userID)
	if err != nil {
		return nil, err
	}

	if h.Cache != nil {
		h.Cache.Set(userID, sessions)
	}
	return sessions, nil
}

func writeUpstreamError(w http.ResponseWriter, message string, err error) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusBadGateway)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"error":   message,
		"details": err.Error(),
	})
}
