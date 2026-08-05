CREATE TABLE user_exclusions
(
    id               BIGSERIAL PRIMARY KEY,
    -- Пользователь, который сделал dislike / block
    action_user_id   BIGINT      NOT NULL
        REFERENCES users (id)
            ON DELETE CASCADE,
    -- Пользователь, которого исключили
    target_user_id   BIGINT      NOT NULL
        REFERENCES users (id)
            ON DELETE CASCADE,
    -- FALSE = обычный dislike, временное исключение
    -- TRUE  = постоянная блокировка
    is_permanent_ban BOOLEAN     NOT NULL DEFAULT FALSE,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT user_exclusions_no_self
        CHECK (action_user_id <> target_user_id),
    CONSTRAINT user_exclusions_unique_pair
        UNIQUE (action_user_id, target_user_id)
);

CREATE INDEX user_exclusions_target_idx
    ON user_exclusions (target_user_id);

CREATE INDEX user_exclusions_temporary_idx
    ON user_exclusions (created_at) WHERE is_permanent_ban = FALSE;