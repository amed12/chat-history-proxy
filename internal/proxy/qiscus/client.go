// Package qiscus is a minimal client for the Qiscus admin REST API, used by
// this proxy to read a user's rooms and a room's messages
// with server credentials. It intentionally never accepts a user token —
// this is what lets it read history after a user's own session has expired.
package qiscus

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"
)

// Client wraps the Qiscus admin REST API.
type Client struct {
	appID     string
	secretKey string
	baseURL   string
	http      *http.Client
}

// NewClient creates a Client using server-side Qiscus credentials.
func NewClient(appID, secretKey, baseURL string) *Client {
	return &Client{
		appID:     appID,
		secretKey: secretKey,
		baseURL:   baseURL,
		http:      &http.Client{Timeout: 15 * time.Second},
	}
}

// Session is one room in a user's chat history, mapped to the shape the
// proxy's own /api/v1/sessions endpoint exposes.
type Session struct {
	RoomID      string `json:"room_id"`
	Name        string `json:"name"`
	StartedAt   string `json:"started_at"`
	IsResolved  bool   `json:"is_resolved"`
	LastMessage string `json:"last_message"`
	// Topic is set by the app (see example/src/Chat.tsx's TopicDialog flow)
	// via qiscus.updateChatRoom(..., extras) right after a new room is
	// created — it lives in the SAME room_options JSON blob as is_resolved,
	// merged client-side to avoid clobbering it. Empty for rooms created
	// before this existed, or created outside the app's own flow.
	Topic string `json:"topic"`
}

// Message is one archived message in a room, mapped to the shape the proxy's
// own /api/v1/sessions/{room_id}/messages endpoint exposes.
type Message struct {
	ID         string      `json:"id"`
	SenderRole string      `json:"sender_role"`
	SenderName string      `json:"sender_name"`
	Type       string      `json:"type"`
	Text       string      `json:"text"`
	Payload    interface{} `json:"payload"`
	CreatedAt  string      `json:"created_at"`
}

// getUserRoomsResponse mirrors the REAL get_user_rooms response, verified
// against a live Qiscus app on 2026-09-15 (see
// docs/CHAT_HISTORY_PROXY.md §9/§12). It differs from the admin
// REST docs' generic shape in two important ways:
//   - the array key is "rooms", not "rooms_info"
//   - there is no last_comment / room_created_at at all; is_resolved lives
//     inside "room_options", which is itself a JSON string, not an object
type getUserRoomsResponse struct {
	Results struct {
		Rooms []struct {
			RoomID      string `json:"room_id"`
			RoomName    string `json:"room_name"`
			RoomOptions string `json:"room_options"`
		} `json:"rooms"`
	} `json:"results"`
}

type roomOptions struct {
	IsResolved bool   `json:"is_resolved"`
	Topic      string `json:"topic"`
}

// GetUserRooms returns every room belonging to userID, in the order Qiscus
// returns them.
//
// get_user_rooms itself does not return a last message or a start timestamp
// for a room (verified against a live app — see comment on
// getUserRoomsResponse), so this makes one extra load_comments call PER ROOM
// to fill Session.StartedAt (first message's timestamp) and
// Session.LastMessage. That's a real cost — N+1 upstream calls for a list of
// N rooms — accepted here for a POC-scale room count; see
// docs/CHAT_HISTORY_PROXY.md §12/§14. A room whose message fetch
// fails is still returned (with those two fields empty) rather than failing
// the whole list.
func (c *Client) GetUserRooms(userID string) ([]Session, error) {
	endpoint := fmt.Sprintf(
		"%s/api/v2.1/rest/get_user_rooms?user_id=%s",
		c.baseURL, url.QueryEscape(userID),
	)

	var body getUserRoomsResponse
	if err := c.getJSON(endpoint, &body); err != nil {
		return nil, fmt.Errorf("qiscus: get_user_rooms: %w", err)
	}

	sessions := make([]Session, 0, len(body.Results.Rooms))
	for _, room := range body.Results.Rooms {
		var opts roomOptions
		// room_options is attacker-uncontrolled (it's our own server
		// calling Qiscus with server credentials), but still just best-effort:
		// an app that omits/changes this field shouldn't break the whole list.
		_ = json.Unmarshal([]byte(room.RoomOptions), &opts)

		session := Session{
			RoomID:     room.RoomID,
			Name:       room.RoomName,
			IsResolved: opts.IsResolved,
			Topic:      opts.Topic,
		}

		if messages, err := c.GetRoomMessages(room.RoomID); err == nil && len(messages) > 0 {
			session.StartedAt = messages[0].CreatedAt
			session.LastMessage = lastDisplayableMessage(messages)
		}

		sessions = append(sessions, session)
	}
	return sessions, nil
}

// lastDisplayableMessage prefers the newest message that isn't a system
// event ("X joined this conversation", "admin marked this conversation as
// resolved", …) — those make for a confusing session-list preview. Falls
// back to the last message of any type if that's genuinely all there is.
func lastDisplayableMessage(messages []Message) string {
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].SenderRole != "system" {
			return messages[i].Text
		}
	}
	return messages[len(messages)-1].Text
}

// loadCommentsResponse mirrors the REAL load_comments response, verified
// against a live Qiscus app on 2026-09-15 (see
// docs/CHAT_HISTORY_PROXY.md §12) — sender username and role live
// inside "user"/"user.extras", not as top-level "username"/"user_type"
// fields.
type loadCommentsResponse struct {
	Results struct {
		Comments []struct {
			ID        int64       `json:"id"`
			Message   string      `json:"message"`
			Type      string      `json:"type"`
			Payload   interface{} `json:"payload"`
			Timestamp string      `json:"timestamp"`
			User      struct {
				Username string `json:"username"`
				Extras   struct {
					IsCustomer bool   `json:"is_customer"`
					Type       string `json:"type"`
				} `json:"extras"`
			} `json:"user"`
		} `json:"comments"`
	} `json:"results"`
}

// GetRoomMessages returns every message in roomID, oldest first.
//
// load_comments itself returns NEWEST first — verified against a live
// Qiscus app on 2026-09-16 (a previous comment here claimed oldest-first,
// which was wrong and never actually re-checked against real multi-message
// data; see docs/CHAT_HISTORY_PROXY.md §15). That bug silently
// swapped GetUserRooms' StartedAt (which read index 0, actually the NEWEST
// message) and LastMessage (which searched backward from the end, actually
// landing on the OLDEST message) — exactly backwards. Reversed here once,
// so every caller can keep assuming ascending order.
func (c *Client) GetRoomMessages(roomID string) ([]Message, error) {
	endpoint := fmt.Sprintf(
		"%s/api/v2.1/rest/load_comments?room_id=%s",
		c.baseURL, url.QueryEscape(roomID),
	)

	var body loadCommentsResponse
	if err := c.getJSON(endpoint, &body); err != nil {
		return nil, fmt.Errorf("qiscus: load_comments: %w", err)
	}

	comments := body.Results.Comments
	messages := make([]Message, len(comments))
	for i, comment := range comments {
		// comments[0] is the newest; write it to the last slot so the
		// returned slice ends up oldest-first.
		messages[len(comments)-1-i] = Message{
			ID:         fmt.Sprintf("%d", comment.ID),
			SenderRole: senderRoleFor(comment.Type, comment.User.Extras.IsCustomer, comment.User.Extras.Type),
			SenderName: comment.User.Username,
			Type:       comment.Type,
			Text:       comment.Message,
			Payload:    comment.Payload,
			CreatedAt:  comment.Timestamp,
		}
	}
	return messages, nil
}

// senderRoleFor maps Qiscus's per-comment sender info to the proxy's own
// sender_role contract ('user' | 'agent' | 'bot' | 'system').
func senderRoleFor(commentType string, isCustomer bool, extrasType string) string {
	if commentType == "system_event" {
		return "system"
	}
	if isCustomer {
		return "user"
	}
	if extrasType == "bot" {
		return "bot"
	}
	return "agent"
}

func (c *Client) getJSON(endpoint string, out interface{}) error {
	req, err := http.NewRequest(http.MethodGet, endpoint, nil)
	if err != nil {
		return err
	}
	req.Header.Set("QISCUS_SDK_APP_ID", c.appID)
	req.Header.Set("QISCUS_SDK_SECRET", c.secretKey)

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status %d", resp.StatusCode)
	}

	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("decode failed: %w", err)
	}
	return nil
}
