CREATE TABLE profiles
(
    id               BIGSERIAL PRIMARY KEY,
    user_id          BIGINT      NOT NULL UNIQUE
        REFERENCES users (id)
            ON DELETE CASCADE,
    full_name        TEXT        NOT NULL,
    birthday         DATE        NOT NULL,
    gender           SMALLINT    NOT NULL,
    bio              TEXT        NOT NULL DEFAULT '',
    latitude         DOUBLE PRECISION,
    longitude        DOUBLE PRECISION,
    city             TEXT,
    is_active        BOOLEAN     NOT NULL DEFAULT TRUE,
    last_activity_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT profiles_name_not_blank
        CHECK (length(trim(full_name)) > 0),
    CONSTRAINT profiles_latitude_range
        CHECK (
            latitude IS NULL
                OR latitude BETWEEN -90 AND 90
            ),
    CONSTRAINT profiles_longitude_range
        CHECK (
            longitude IS NULL
                OR longitude BETWEEN -180 AND 180
            )
);

CREATE INDEX profiles_feed_idx
    ON profiles (
                 is_active,
                 gender,
                 birthday
        );

CREATE INDEX profiles_city_idx
    ON profiles (city);

CREATE INDEX profiles_last_activity_idx
    ON profiles (last_activity_at);