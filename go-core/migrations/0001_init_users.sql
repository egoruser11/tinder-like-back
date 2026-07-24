CREATE TABLE IF NOT EXISTS users (
    id            BIGSERIAL PRIMARY KEY,
    email         TEXT NOT NULL UNIQUE,
    password_hash TEXT NOT NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);


-- ticket-2: add profiles table (name, birthdate, bio, gender, location)
-- ticket-5: add swipes table (from_user_id, to_user_id, liked, created_at)
--           and matches table (user_a_id, user_b_id, created_at)
