CREATE TABLE user_photos
(
    id           BIGSERIAL PRIMARY KEY,
    user_id      BIGINT      NOT NULL
        REFERENCES users (id)
            ON DELETE CASCADE,
    -- Bucket в MinIO.
    -- Например: user-photos
    bucket_name  TEXT        NOT NULL DEFAULT 'user-photos',
    -- Путь до объекта внутри bucket.
    -- Например:
    -- users/15/550e8400-e29b-41d4-a716-446655440000.jpg
    object_key   TEXT        NOT NULL,
    -- MIME-тип загруженного файла.
    -- image/jpeg, image/png, image/webp
    content_type TEXT        NOT NULL,
    -- Размер файла в байтах.
    file_size    BIGINT      NOT NULL,
    -- Позиция фотографии в профиле.
    position     SMALLINT    NOT NULL DEFAULT 0,
    is_main      BOOLEAN     NOT NULL DEFAULT FALSE,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT user_photos_object_key_not_blank
        CHECK (length(trim(object_key)) > 0),

    CONSTRAINT user_photos_bucket_not_blank
        CHECK (length(trim(bucket_name)) > 0),

    CONSTRAINT user_photos_position_valid
        CHECK (position >= 0),

    CONSTRAINT user_photos_file_size_valid
        CHECK (
            file_size > 0
                AND file_size <= 10485760
            ),

    CONSTRAINT user_photos_content_type_valid
        CHECK (
            content_type IN (
                             'image/jpeg',
                             'image/png',
                             'image/webp'
                )
            ),

    CONSTRAINT user_photos_bucket_object_unique
        UNIQUE (bucket_name, object_key),

    CONSTRAINT user_photos_user_position_unique
        UNIQUE (user_id, position)
);

CREATE INDEX user_photos_user_idx
    ON user_photos (user_id);

CREATE UNIQUE INDEX user_photos_one_main_uq
    ON user_photos (user_id) WHERE is_main = TRUE;