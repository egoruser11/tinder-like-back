package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"tinder-core/internal/models"
)

var ErrPhotoNotFound = errors.New("photo not found")

type PhotoRepository struct {
	db *pgxpool.Pool
}

func NewPhotoRepository(db *pgxpool.Pool) *PhotoRepository {
	return &PhotoRepository{db: db}
}

func (repo *PhotoRepository) ListByUserID(ctx context.Context, userID int64) ([]models.UserPhoto, error) {
	rows, err := repo.db.Query(ctx, `
		SELECT id, user_id, bucket_name, object_key, content_type, file_size, position, is_main, created_at
		FROM user_photos
		WHERE user_id = $1
		ORDER BY position ASC
	`, userID)
	if err != nil {
		return nil, fmt.Errorf("list user photos: %w", err)
	}
	defer rows.Close()

	photos := make([]models.UserPhoto, 0)
	for rows.Next() {
		var photo models.UserPhoto
		if err := scanPhoto(rows, &photo); err != nil {
			return nil, err
		}
		photos = append(photos, photo)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate user photos: %w", err)
	}
	return photos, nil
}

// Create appends a photo and makes the first photo of a profile its main one.
func (repo *PhotoRepository) Create(ctx context.Context, photo models.UserPhoto) (*models.UserPhoto, error) {
	tx, err := repo.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin create photo: %w", err)
	}
	defer tx.Rollback(ctx)

	var position int16
	if err := tx.QueryRow(ctx, `
		SELECT COALESCE(MAX(position), -1) + 1
		FROM user_photos
		WHERE user_id = $1
	`, photo.UserID).Scan(&position); err != nil {
		return nil, fmt.Errorf("get next photo position: %w", err)
	}
	var hasPhotos bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM user_photos WHERE user_id = $1)`, photo.UserID).Scan(&hasPhotos); err != nil {
		return nil, fmt.Errorf("check existing photos: %w", err)
	}

	var saved models.UserPhoto
	err = tx.QueryRow(ctx, `
		INSERT INTO user_photos (
			user_id, bucket_name, object_key, content_type, file_size, position, is_main
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, user_id, bucket_name, object_key, content_type, file_size, position, is_main, created_at
	`, photo.UserID, photo.BucketName, photo.ObjectKey, photo.ContentType, photo.FileSize, position, !hasPhotos).Scan(
		&saved.ID,
		&saved.UserID,
		&saved.BucketName,
		&saved.ObjectKey,
		&saved.ContentType,
		&saved.FileSize,
		&saved.Position,
		&saved.IsMain,
		&saved.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("create photo metadata: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit create photo: %w", err)
	}
	return &saved, nil
}

func (repo *PhotoRepository) GetByID(ctx context.Context, userID, photoID int64) (*models.UserPhoto, error) {
	var photo models.UserPhoto
	err := repo.db.QueryRow(ctx, `
		SELECT id, user_id, bucket_name, object_key, content_type, file_size, position, is_main, created_at
		FROM user_photos
		WHERE user_id = $1 AND id = $2
	`, userID, photoID).Scan(
		&photo.ID,
		&photo.UserID,
		&photo.BucketName,
		&photo.ObjectKey,
		&photo.ContentType,
		&photo.FileSize,
		&photo.Position,
		&photo.IsMain,
		&photo.CreatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrPhotoNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get photo: %w", err)
	}
	return &photo, nil
}

func (repo *PhotoRepository) Delete(ctx context.Context, userID, photoID int64) error {
	commandTag, err := repo.db.Exec(ctx, `DELETE FROM user_photos WHERE user_id = $1 AND id = $2`, userID, photoID)
	if err != nil {
		return fmt.Errorf("delete photo metadata: %w", err)
	}
	if commandTag.RowsAffected() == 0 {
		return ErrPhotoNotFound
	}
	return nil
}

type photoScanner interface {
	Scan(...any) error
}

func scanPhoto(scanner photoScanner, photo *models.UserPhoto) error {
	if err := scanner.Scan(
		&photo.ID,
		&photo.UserID,
		&photo.BucketName,
		&photo.ObjectKey,
		&photo.ContentType,
		&photo.FileSize,
		&photo.Position,
		&photo.IsMain,
		&photo.CreatedAt,
	); err != nil {
		return fmt.Errorf("scan photo: %w", err)
	}
	return nil
}
