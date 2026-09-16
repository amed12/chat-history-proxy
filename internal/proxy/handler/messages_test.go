package handler

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/qiscus-community/chat-history-proxy/internal/proxy/qiscus"
)

type fakeFetcher struct {
	calls    int
	messages []qiscus.Message
	err      error
}

func (f *fakeFetcher) GetRoomMessages(roomID string) ([]qiscus.Message, error) {
	f.calls++
	return f.messages, f.err
}

func newTestRouter(h *MessagesHandler) http.Handler {
	r := chi.NewRouter()
	r.Get("/api/v1/sessions/{room_id}/messages", h.ServeHTTP)
	return r
}

func TestMessagesHandler_RoomNotOwnedByUser(t *testing.T) {
	lister := &fakeLister{rooms: []qiscus.Session{{RoomID: "my-room"}}}
	fetcher := &fakeFetcher{messages: []qiscus.Message{{ID: "1"}}}
	h := &MessagesHandler{Lister: lister, Fetcher: fetcher}
	router := newTestRouter(h)

	req := withUserID(httptest.NewRequest(http.MethodGet, "/api/v1/sessions/someone-elses-room/messages", nil), "user-1")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404 for a room not owned by the caller", rec.Code)
	}
	if fetcher.calls != 0 {
		t.Errorf("GetRoomMessages was called %d times, want 0 — ownership must be checked first", fetcher.calls)
	}
}

func TestMessagesHandler_RoomOwnedByUser(t *testing.T) {
	lister := &fakeLister{rooms: []qiscus.Session{{RoomID: "my-room"}}}
	fetcher := &fakeFetcher{messages: []qiscus.Message{{ID: "1", Text: "hi"}}}
	h := &MessagesHandler{Lister: lister, Fetcher: fetcher}
	router := newTestRouter(h)

	req := withUserID(httptest.NewRequest(http.MethodGet, "/api/v1/sessions/my-room/messages", nil), "user-1")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 for a room owned by the caller", rec.Code)
	}
	if fetcher.calls != 1 {
		t.Errorf("GetRoomMessages was called %d times, want 1", fetcher.calls)
	}
}

func TestMessagesHandler_Unauthorized(t *testing.T) {
	h := &MessagesHandler{Lister: &fakeLister{}, Fetcher: &fakeFetcher{}}
	router := newTestRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/sessions/my-room/messages", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
}

func TestMessagesHandler_UpstreamFailureOnFetch(t *testing.T) {
	lister := &fakeLister{rooms: []qiscus.Session{{RoomID: "my-room"}}}
	fetcher := &fakeFetcher{err: errors.New("qiscus down")}
	h := &MessagesHandler{Lister: lister, Fetcher: fetcher}
	router := newTestRouter(h)

	req := withUserID(httptest.NewRequest(http.MethodGet, "/api/v1/sessions/my-room/messages", nil), "user-1")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code < 500 {
		t.Fatalf("status = %d, want a 5xx on upstream failure", rec.Code)
	}
}
