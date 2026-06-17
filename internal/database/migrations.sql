-- Chat archive table
-- One row per resolved Qiscus room.
-- Messages stored as JSONB for flexibility (schema can evolve without migration).
CREATE TABLE IF NOT EXISTS chat_archives (
    id          BIGSERIAL PRIMARY KEY,
    room_id     VARCHAR(50)  UNIQUE NOT NULL,
    room_name   VARCHAR(255) NOT NULL DEFAULT '',
    app_id      VARCHAR(100) NOT NULL,
    messages    JSONB        NOT NULL DEFAULT '[]',
    archived_at TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    source      VARCHAR(20)  NOT NULL DEFAULT 'webhook', -- 'webhook' | 'on_demand'
    updated_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_chat_archives_room_id ON chat_archives (room_id);
CREATE INDEX IF NOT EXISTS idx_chat_archives_app_id  ON chat_archives (app_id);
