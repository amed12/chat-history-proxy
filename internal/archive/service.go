package archive

import (
    "log"

    "github.com/hellogod/chat-history-service/internal/qiscus"
)

// Service orchestrates fetching from Qiscus and persisting to the DB.
type Service struct {
    repo      *Repository
    qiscus    *qiscus.Client
    appID     string
}

// NewService creates an archive Service.
func NewService(repo *Repository, qiscusClient *qiscus.Client, appID string) *Service {
    return &Service{repo: repo, qiscus: qiscusClient, appID: appID}
}

// FetchAndArchive fetches messages for roomID from Qiscus and upserts them
// into the database. Safe to call concurrently for different room IDs.
//
// This is called:
//   - From the webhook handler (async goroutine, source="webhook")
//   - From the history handler when DB has no record (source="on_demand")
func (s *Service) FetchAndArchive(roomID, source string) (*ChatArchive, error) {
    roomName, messages, err := s.qiscus.GetRoomMessages(roomID)
    if err != nil {
        return nil, err
    }

    archived := make([]ArchivedMessage, 0, len(messages))
    for _, m := range messages {
        archived = append(archived, ArchivedMessage{
            ID:          m.ID,
            UniqueID:    m.UniqueID,
            Text:        m.Text,
            SenderName:  m.SenderName,
            SenderEmail: m.SenderEmail,
            Timestamp:   m.Timestamp,
            Type:        m.Type,
            Extras:      m.Extras,
            Payload:     m.Payload,
        })
    }

    record := &ChatArchive{
        RoomID:   roomID,
        RoomName: roomName,
        AppID:    s.appID,
        Messages: archived,
        Source:   source,
    }

    if err := s.repo.Upsert(record); err != nil {
        log.Printf("archive: upsert failed for room %s: %v", roomID, err)
        return nil, err
    }

    log.Printf("archive: saved %d messages for room %s (source=%s)", len(archived), roomID, source)
    return record, nil
}

// GetOrFetch returns the archive for roomID from DB if it exists,
// otherwise fetches from Qiscus first (on-demand).
func (s *Service) GetOrFetch(roomID string) (*ChatArchive, error) {
    record, err := s.repo.FindByRoomID(roomID)
    if err == nil {
        return record, nil // cache hit
    }
    if err != ErrNotFound {
        return nil, err
    }

    // Not in DB yet — fetch on demand.
    return s.FetchAndArchive(roomID, "on_demand")
}
