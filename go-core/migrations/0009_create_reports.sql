CREATE TABLE reports
(
    id             BIGSERIAL PRIMARY KEY,
    -- Кто отправил жалобу
    action_user_id BIGINT      NOT NULL
        REFERENCES users (id)
            ON DELETE CASCADE,
    -- На кого пожаловались
    target_user_id BIGINT      NOT NULL
        REFERENCES users (id)
            ON DELETE CASCADE,
    -- 1 = fake
    -- 2 = spam
    -- 3 = inappropriate_content
    -- 4 = harassment
    -- 5 = other
    reason         SMALLINT    NOT NULL,
    comment        TEXT,
    -- FALSE = жалоба еще не обработана
    -- TRUE  = жалоба обработана
    is_resolved    BOOLEAN     NOT NULL DEFAULT FALSE,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT reports_no_self
        CHECK (action_user_id <> target_user_id),

    CONSTRAINT reports_reason_valid
        CHECK (reason IN (1, 2, 3, 4, 5)),

    CONSTRAINT reports_comment_length
        CHECK (
            comment IS NULL
                OR char_length(comment) <= 2000
            ),

    CONSTRAINT reports_updated_time_valid
        CHECK (updated_at >= created_at)
);

CREATE INDEX reports_target_idx
    ON reports (target_user_id);

CREATE INDEX reports_unresolved_idx
    ON reports (created_at) WHERE is_resolved = FALSE;