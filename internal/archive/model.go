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
