package qiscus

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// roomsAndCommentsServer routes get_user_rooms and load_comments on one test
// server (GetUserRooms now calls both) based on the query string, so each
// endpoint's fixture can be swapped independently per test.
func roomsAndCommentsServer(t *testing.T, roomsBody string, commentsByRoom map[string]string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.Contains(r.URL.Path, "get_user_rooms"):
			_, _ = w.Write([]byte(roomsBody))
		case strings.Contains(r.URL.Path, "load_comments"):
			roomID := r.URL.Query().Get("room_id")
			body, ok := commentsByRoom[roomID]
			if !ok {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			_, _ = w.Write([]byte(body))
		default:
			t.Fatalf("unexpected request path: %s", r.URL.Path)
		}
	}))
}

func TestGetUserRooms(t *testing.T) {
	// Fixture mirrors REAL get_user_rooms/load_comments responses captured
	// against a live Qiscus app on 2026-09-15 (see
	// docs/CHAT_HISTORY_PROXY.md §12/§14) — array key "rooms" (not
	// "rooms_info"), is_resolved nested inside the room_options JSON
	// *string*, no last_comment/room_created_at (that's why GetUserRooms
	// makes a second load_comments call per room instead).
	server := roomsAndCommentsServer(t, `{
		"results": {
			"meta": {"current_page": 1, "total_room": 1},
			"rooms": [
				{
					"room_id": "room-1",
					"room_name": "Session 1",
					"room_options": "{\"is_resolved\": true, \"channel\": \"qiscus\"}"
				}
			]
		},
		"status": 200
	}`, map[string]string{
		// Newest first, matching the real load_comments order (see
		// TestGetRoomMessages) — GetUserRooms relies on GetRoomMessages
		// reversing this to derive StartedAt (oldest) and LastMessage
		// correctly.
		"room-1": `{
			"results": {
				"comments": [
					{"id": 2, "message": "admin marked this conversation as resolved", "type": "system_event", "timestamp": "2022-03-15T08:05:00Z", "user": {"username": "System", "extras": {"is_customer": false}}},
					{"id": 1, "message": "halo, saya mau tanya soal pengiriman", "type": "text", "timestamp": "2022-03-15T08:00:00Z", "user": {"username": "Customer", "extras": {"is_customer": true}}}
				]
			}
		}`,
	})
	defer server.Close()

	client := NewClient("app-1", "secret-1", server.URL)
	sessions, err := client.GetUserRooms("user-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(sessions) != 1 {
		t.Fatalf("expected 1 session, got %d", len(sessions))
	}
	got := sessions[0]
	want := Session{
		RoomID:      "room-1",
		Name:        "Session 1",
		IsResolved:  true,
		StartedAt:   "2022-03-15T08:00:00Z",
		LastMessage: "halo, saya mau tanya soal pengiriman", // system_event skipped, not the literal last comment
	}
	if got != want {
		t.Errorf("session = %+v, want %+v", got, want)
	}
}

func TestGetUserRooms_DecodesTopicFromRoomOptions(t *testing.T) {
	// Topic is written by the app via qiscus.updateChatRoom(..., extras)
	// merged with the room's existing extras (see example/src/Chat.tsx) —
	// it lands in the same room_options JSON blob as is_resolved, alongside
	// it, not replacing it.
	server := roomsAndCommentsServer(t, `{
		"results": {
			"rooms": [
				{
					"room_id": "room-1",
					"room_name": "Session 1",
					"room_options": "{\"is_resolved\": false, \"topic\": \"Tanya status pengiriman\"}"
				}
			]
		}
	}`, map[string]string{
		"room-1": `{"results": {"comments": []}}`,
	})
	defer server.Close()

	client := NewClient("app-1", "secret-1", server.URL)
	sessions, err := client.GetUserRooms("user-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(sessions) != 1 || sessions[0].Topic != "Tanya status pengiriman" {
		t.Fatalf("expected topic decoded from room_options, got %+v", sessions)
	}
	if sessions[0].IsResolved {
		t.Fatalf("expected is_resolved=false to survive alongside topic, got %+v", sessions[0])
	}
}

func TestGetUserRooms_MalformedRoomOptionsDoesNotBreakTheList(t *testing.T) {
	server := roomsAndCommentsServer(t, `{
		"results": {
			"rooms": [
				{"room_id": "room-1", "room_name": "Session 1", "room_options": "not-json"}
			]
		}
	}`, map[string]string{
		"room-1": `{"results": {"comments": []}}`,
	})
	defer server.Close()

	client := NewClient("app-1", "secret-1", server.URL)
	sessions, err := client.GetUserRooms("user-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(sessions) != 1 || sessions[0].RoomID != "room-1" {
		t.Fatalf("expected the room to still be listed despite bad room_options, got %+v", sessions)
	}
}

func TestGetUserRooms_UpstreamError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	client := NewClient("app-1", "secret-1", server.URL)
	_, err := client.GetUserRooms("user-1")
	if err == nil {
		t.Fatal("expected an error when upstream returns 500")
	}
}

func TestGetUserRooms_LoadCommentsFailureLeavesRoomListedWithoutThoseFields(t *testing.T) {
	// The load_comments call for this room 500s (no fixture registered for
	// it) — GetUserRooms must still return the room, just without
	// StartedAt/LastMessage, rather than dropping it or failing the whole list.
	server := roomsAndCommentsServer(t, `{
		"results": {
			"rooms": [
				{"room_id": "room-1", "room_name": "Session 1", "room_options": "{\"is_resolved\": false}"}
			]
		}
	}`, map[string]string{})
	defer server.Close()

	client := NewClient("app-1", "secret-1", server.URL)
	sessions, err := client.GetUserRooms("user-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(sessions) != 1 || sessions[0].StartedAt != "" || sessions[0].LastMessage != "" {
		t.Fatalf("expected room listed with empty StartedAt/LastMessage, got %+v", sessions)
	}
}

func TestGetRoomMessages(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("room_id") != "room-1" {
			t.Errorf("expected room_id=room-1, got %s", r.URL.RawQuery)
		}
		w.Header().Set("Content-Type", "application/json")
		// Fixture mirrors a REAL load_comments response captured against a
		// live Qiscus app on 2026-09-16 — username/role live inside "user"
		// (not top-level fields), AND comments come back NEWEST first (a
		// previous version of this fixture had them oldest-first, which was
		// simply wrong — GetRoomMessages is what reverses this to
		// ascending, so the assertions below check its *output* order).
		_, _ = w.Write([]byte(`{
			"results": {
				"comments": [
					{
						"id": 3,
						"message": "X joined this conversation",
						"type": "system_event",
						"timestamp": "2022-03-15T08:02:00Z",
						"user": {"username": "System", "extras": {"is_customer": false}}
					},
					{
						"id": 2,
						"message": "hai juga",
						"type": "text",
						"timestamp": "2022-03-15T08:01:00Z",
						"user": {"username": "Agent A", "extras": {"is_customer": false, "type": "agent"}}
					},
					{
						"id": 1,
						"message": "halo",
						"type": "text",
						"timestamp": "2022-03-15T08:00:00Z",
						"user": {"username": "Customer", "extras": {"is_customer": true}}
					}
				]
			}
		}`))
	}))
	defer server.Close()

	client := NewClient("app-1", "secret-1", server.URL)
	messages, err := client.GetRoomMessages("room-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(messages) != 3 {
		t.Fatalf("expected 3 messages, got %d", len(messages))
	}
	if messages[0].SenderRole != "user" || messages[0].SenderName != "Customer" {
		t.Errorf("message 0: got role=%q name=%q, want role=user name=Customer", messages[0].SenderRole, messages[0].SenderName)
	}
	if messages[1].SenderRole != "agent" || messages[1].SenderName != "Agent A" {
		t.Errorf("message 1: got role=%q name=%q, want role=agent name=Agent A", messages[1].SenderRole, messages[1].SenderName)
	}
	if messages[2].SenderRole != "system" {
		t.Errorf("message 2: got role=%q, want system (from type=system_event)", messages[2].SenderRole)
	}
	if messages[0].CreatedAt >= messages[1].CreatedAt {
		t.Errorf("expected messages in ascending time order, got %+v", messages)
	}
	// Regression: comments came in newest-first (id 3, 2, 1 in the fixture
	// above) — GetRoomMessages must reverse that, not pass it through.
	if messages[0].ID != "1" || messages[2].ID != "3" {
		t.Errorf("expected reversed to oldest-first (ids 1,2,3), got ids %s,%s,%s", messages[0].ID, messages[1].ID, messages[2].ID)
	}
}
