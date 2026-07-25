CREATE TABLE IF NOT EXISTS profiles (
    id                 BIGSERIAL PRIMARY KEY,
    user_id            BIGINT NOT NULL UNIQUE REFERENCES users(id) ON DELETE CASCADE,
    full_name          TEXT NOT NULL,
    birthday           DATE NOT NULL,
    bio                TEXT,
    gender             SMALLINT NOT NULL,
    status             TEXT,
    main_photo_id      BIGINT,
    photo_storage_slot BIGINT,
    photo_ids          JSONB NOT NULL DEFAULT '[]',
    latitude           DOUBLE PRECISION,
    longitude          DOUBLE PRECISION,
    is_active          BOOLEAN NOT NULL DEFAULT TRUE,
    created_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at         TIMESTAMPTZ NOT NULL DEFAULT now()
);
