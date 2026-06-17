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
