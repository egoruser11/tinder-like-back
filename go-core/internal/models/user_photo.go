package models

import "time"

// UserPhoto соответствует таблице user_photos. Сам файл хранится в MinIO;
// в Postgres остаются его метаданные и object_key.
type UserPhoto struct {
	ID          int64     `json:"id"`
	UserID      int64     `json:"user_id"`
	BucketName  string    `json:"bucket_name"`
	ObjectKey   string    `json:"object_key"`
	ContentType string    `json:"content_type"`
	FileSize    int64     `json:"file_size"`
	Position    int16     `json:"position"`
	IsMain      bool      `json:"is_main"`
	CreatedAt   time.Time `json:"created_at"`
}
