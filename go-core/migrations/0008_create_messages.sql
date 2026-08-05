CREATE TABLE messages
(
    id         BIGSERIAL PRIMARY KEY,
    chat_id    BIGINT      NOT NULL
        REFERENCES chats (id)
            ON DELETE CASCADE,
    sender_id  BIGINT      NOT NULL
        REFERENCES users (id)
            ON DELETE CASCADE,

    body       TEXT        NOT NULL,
    is_read    BOOLEAN     NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at TIMESTAMPTZ,

    CONSTRAINT messages_body_not_blank
        CHECK (length(trim(body)) > 0),

    CONSTRAINT messages_updated_time_valid
        CHECK (updated_at >= created_at),

    CONSTRAINT messages_deleted_time_valid
        CHECK (
            deleted_at IS NULL
                OR deleted_at >= created_at
            )
);

CREATE INDEX messages_chat_created_idx
    ON messages (chat_id, created_at DESC);

CREATE INDEX messages_unread_idx
    ON messages (chat_id, sender_id) WHERE is_read = FALSE
      AND deleted_at IS NULL;