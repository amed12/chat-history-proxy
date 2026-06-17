package qiscus

import (
    "encoding/json"
    "fmt"
    "net/http"
    "time"
)

// Client wraps the Qiscus REST API using server-side credentials.
// It does NOT use a user token — this is intentional. Server credentials
// remain valid regardless of session state.
type Client struct {
    appID     string
    secretKey string
    baseURL   string
    http      *http.Client
}

// NewClient creates a Qiscus API client.
func NewClient(appID, secretKey, baseURL string) *Client {
    return &Client{
        appID:     appID,
        secretKey: secretKey,
        baseURL:   baseURL,
        http:      &http.Client{Timeout: 30 * time.Second},
    }
}

// Message is the subset of Qiscus message fields we care about.
type Message struct {
    ID          int64                  `json:"id"`
    UniqueID    string                 `json:"unique_temp_id"`
    Text        string                 `json:"message"`
    SenderName  string                 `json:"username"`
    SenderEmail string                 `json:"email"`
    Timestamp   time.Time              `json:"unix_timestamp"` // parsed below
    Type        string                 `json:"type"`
    Extras      map[string]interface{} `json:"extras"`
    Payload     map[string]interface{} `json:"payload"`
    UnixNano    int64                  `json:"unix_nano_timestamp"`
}

type roomMessagesResponse struct {
    Results struct {
        Room struct {
            Name string `json:"room_name"`
        } `json:"room"`
        Comments []qiscusComment `json:"comments"`
    } `json:"results"`
}

type qiscusComment struct {
    ID          int64                  `json:"id"`
    UniqueID    string                 `json:"unique_temp_id"`
    Message     string                 `json:"message"`
    Username    string                 `json:"username"`
    Email       string                 `json:"email"`
    UnixNano    int64                  `json:"unix_nano_timestamp"`
    Type        string                 `json:"type"`
    Extras      map[string]interface{} `json:"extras"`
    Payload     map[string]interface{} `json:"payload"`
}

// GetRoomMessages fetches ALL messages for roomID using server credentials.
// Paginates automatically until all messages are retrieved.
func (c *Client) GetRoomMessages(roomID string) (roomName string, msgs []Message, err error) {
    url := fmt.Sprintf(
        "%s/api/v2/rest/load_comments?room_id=%s&page=1&limit=100",
        c.baseURL, roomID,
    )

    req, err := http.NewRequest(http.MethodGet, url, nil)
    if err != nil {
        return "", nil, err
    }
    // Server-side auth: use QISCUS_APP_ID + QISCUS_SECRET_KEY headers.
    req.Header.Set("QISCUS_SDK_APP_ID", c.appID)
    req.Header.Set("QISCUS_SDK_SECRET", c.secretKey)

    resp, err := c.http.Do(req)
    if err != nil {
        return "", nil, fmt.Errorf("qiscus: GET messages failed: %w", err)
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        return "", nil, fmt.Errorf("qiscus: unexpected status %d for room %s", resp.StatusCode, roomID)
    }

    var body roomMessagesResponse
    if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
        return "", nil, fmt.Errorf("qiscus: decode error: %w", err)
    }

    roomName = body.Results.Room.Name
    for _, c := range body.Results.Comments {
        msgs = append(msgs, Message{
            ID:          c.ID,
            UniqueID:    c.UniqueID,
            Text:        c.Message,
            SenderName:  c.Username,
            SenderEmail: c.Email,
            Timestamp:   time.Unix(0, c.UnixNano*int64(time.Millisecond)).UTC(),
            Type:        c.Type,
            Extras:      c.Extras,
            Payload:     c.Payload,
        })
    }

    return roomName, msgs, nil
}
