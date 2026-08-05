CREATE TABLE chats
(
    id             BIGSERIAL PRIMARY KEY,

    user_1_id      BIGINT      NOT NULL
        REFERENCES users (id)
            ON DELETE CASCADE,

    user_2_id      BIGINT      NOT NULL
        REFERENCES users (id)
            ON DELETE CASCADE,

    is_active      BOOLEAN     NOT NULL DEFAULT TRUE,

    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),

    deactivated_at TIMESTAMPTZ,

    CONSTRAINT chats_no_self
        CHECK (user_1_id <> user_2_id),

    CONSTRAINT chats_active_state_valid
        CHECK (
            (is_active = TRUE AND deactivated_at IS NULL)
                OR
            (is_active = FALSE AND deactivated_at IS NOT NULL)
            )
);

CREATE UNIQUE INDEX chats_unique_pair_uq
    ON chats (
              LEAST(user_1_id, user_2_id),
              GREATEST(user_1_id, user_2_id)
        );

CREATE INDEX chats_user_1_idx
    ON chats (user_1_id);

CREATE INDEX chats_user_2_idx
    ON chats (user_2_id);

CREATE INDEX chats_inactive_idx
    ON chats (deactivated_at) WHERE is_active = FALSE;