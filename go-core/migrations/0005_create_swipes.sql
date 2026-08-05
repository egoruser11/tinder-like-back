CREATE TABLE swipes
(
    id             BIGSERIAL PRIMARY KEY,
    actor_user_id  BIGINT      NOT NULL
        REFERENCES users (id)
            ON DELETE CASCADE,
    target_user_id BIGINT      NOT NULL
        REFERENCES users (id)
            ON DELETE CASCADE,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT swipes_no_self
        CHECK (actor_user_id <> target_user_id),

    CONSTRAINT swipes_unique_pair
        UNIQUE (actor_user_id, target_user_id)
);

CREATE INDEX swipes_target_actor_idx
    ON swipes (target_user_id, actor_user_id);