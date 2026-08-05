CREATE TABLE discovery_preferences
(
    user_id         BIGINT PRIMARY KEY
        REFERENCES users (id)
            ON DELETE CASCADE,
    -- Необязательный город поиска. Если NULL, поиск не ограничивается
    -- городом и использует только фильтрацию по координатам.
    city            TEXT,
    min_age         SMALLINT    NOT NULL DEFAULT 18,
    max_age         SMALLINT    NOT NULL DEFAULT 99,
    -- NULL = пол не учитывать
    -- 1 = мужчина
    -- 2 = женщина
    gender          SMALLINT,
    is_verified     BOOLEAN     NOT NULL DEFAULT FALSE,
    max_distance_km INTEGER     NOT NULL DEFAULT 50,
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT discovery_age_valid
        CHECK (
            min_age >= 18
                AND max_age >= min_age
                AND max_age <= 99
            ),

    CONSTRAINT discovery_gender_valid
        CHECK (
            gender IS NULL
                OR gender IN (1, 2)
            ),

    CONSTRAINT discovery_distance_valid
        CHECK (
            max_distance_km > 0
                AND max_distance_km <= 1000
            )
);
