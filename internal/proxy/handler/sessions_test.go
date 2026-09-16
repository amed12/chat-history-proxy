package handler

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	megamiddleware "github.com/amed12/chat-history-proxy/internal/proxy/middleware"
	"github.com/amed12/chat-history-proxy/internal/proxy/qiscus"
)

type fakeLister struct {
	calls   int
	rooms   []qiscus.Session
	err     error
	lastArg string
}

func (f *fakeLister) GetUserRooms(userID string) ([]qiscus.Session, error) {
	f.calls++
	f.lastArg = userID
	return f.rooms, f.err
}

type fakeCache struct {
	store map[string][]qiscus.Session
}

func newFakeCache() *fakeCache {
	return &fakeCache{store: make(map[string][]qiscus.Session)}
}

func (f *fakeCache) Get(key string) ([]qiscus.Session, bool) {
	v, ok := f.store[key]
	return v, ok
}

func (f *fakeCache) Set(key string, value []qiscus.Session) {
	f.store[key] = value
}

func withUserID(r *http.Request, userID string) *http.Request {
	ctx := context.WithValue(r.Context(), megamiddleware.UserIDKey, userID)
	return r.WithContext(ctx)
}

func TestSessionsHandler_Unauthorized(t *testing.T) {
	h := &SessionsHandler{Lister: &fakeLister{}}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/sessions", nil)
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
}

func TestSessionsHandler_UsesSubFromContext(t *testing.T) {
	lister := &fakeLister{rooms: []qiscus.Session{{RoomID: "r1"}}}
	h := &SessionsHandler{Lister: lister}
	req := withUserID(httptest.NewRequest(http.MethodGet, "/api/v1/sessions?user_id=someone-else", nil), "user-1")
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if lister.lastArg != "user-1" {
		t.Errorf("GetUserRooms called with %q, want %q (from JWT, not query string)", lister.lastArg, "user-1")
	}
}

func TestSessionsHandler_CacheHitAvoidsSecondUpstreamCall(t *testing.T) {
	lister := &fakeLister{rooms: []qiscus.Session{{RoomID: "r1"}}}
	cache := newFakeCache()
	h := &SessionsHandler{Lister: lister, Cache: cache}

	for i := 0; i < 2; i++ {
		req := withUserID(httptest.NewRequest(http.MethodGet, "/api/v1/sessions", nil), "user-1")
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("request %d: status = %d, want 200", i, rec.Code)
		}
	}

	if lister.calls != 1 {
		t.Errorf("GetUserRooms called %d times, want 1 (second request should hit cache)", lister.calls)
	}
}

func TestSessionsHandler_FreshQueryParamBypassesCache(t *testing.T) {
	lister := &fakeLister{rooms: []qiscus.Session{{RoomID: "r1"}}}
	cache := newFakeCache()
	h := &SessionsHandler{Lister: lister, Cache: cache}

	req1 := withUserID(httptest.NewRequest(http.MethodGet, "/api/v1/sessions", nil), "user-1")
	h.ServeHTTP(httptest.NewRecorder(), req1)

	// Without ?fresh=1 this second request would hit the cache (see
	// TestSessionsHandler_CacheHitAvoidsSecondUpstreamCall) — with it, it
	// must reach Qiscus again, e.g. right after the app just created a new
	// room and needs the list to reflect that immediately.
	req2 := withUserID(httptest.NewRequest(http.MethodGet, "/api/v1/sessions?fresh=1", nil), "user-1")
	h.ServeHTTP(httptest.NewRecorder(), req2)

	if lister.calls != 2 {
		t.Errorf("GetUserRooms called %d times, want 2 (fresh=1 must bypass the cache)", lister.calls)
	}
}

func TestSessionsHandler_UpstreamFailure(t *testing.T) {
	lister := &fakeLister{err: errors.New("qiscus down")}
	h := &SessionsHandler{Lister: lister}
	req := withUserID(httptest.NewRequest(http.MethodGet, "/api/v1/sessions", nil), "user-1")
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code < 500 {
		t.Fatalf("status = %d, want a 5xx on upstream failure, not a silent empty list", rec.Code)
	}
}
