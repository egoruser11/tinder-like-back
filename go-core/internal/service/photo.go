package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"

	"tinder-core/internal/models"
)

const maxPhotoSizeBytes = 10 * 1024 * 1024

var (
	ErrInvalidPhoto          = errors.New("photo must be a non-empty JPEG, PNG, or WebP file no larger than 10 MB")
	ErrInvalidPhotoID        = errors.New("invalid photo id")
	ErrCouldNotGeneratePhoto = errors.New("could not generate photo key")
)

type photoStore interface {
	ListByUserID(context.Context, int64) ([]models.UserPhoto, error)
	Create(context.Context, models.UserPhoto) (*models.UserPhoto, error)
	GetByID(context.Context, int64, int64) (*models.UserPhoto, error)
	Delete(context.Context, int64, int64) error
}

type objectStore interface {
	Bucket() string
	Put(context.Context, string, string, int64, io.Reader) error
	Delete(context.Context, string) error
}

type PhotoService struct {
	repository photoStore
	storage    objectStore
	newKey     func() (string, error)
}

func NewPhotoService(repository photoStore, storage objectStore) *PhotoService {
	return &PhotoService{repository: repository, storage: storage, newKey: randomPhotoKey}
}

func (s *PhotoService) List(ctx context.Context, userID int64) ([]models.UserPhoto, error) {
	return s.repository.ListByUserID(ctx, userID)
}

func (s *PhotoService) Upload(ctx context.Context, userID int64, contentType string, size int64, reader io.Reader) (*models.UserPhoto, error) {
	extension, valid := photoExtension(contentType)
	if !valid || size <= 0 || size > maxPhotoSizeBytes {
		return nil, ErrInvalidPhoto
	}
	key, err := s.newKey()
	if err != nil {
		return nil, fmt.Errorf("generate photo key: %w", err)
	}
	objectKey := fmt.Sprintf("users/%d/%s%s", userID, key, extension)
	if err := s.storage.Put(ctx, objectKey, contentType, size, reader); err != nil {
		return nil, err
	}

	photo, err := s.repository.Create(ctx, models.UserPhoto{
		UserID:      userID,
		BucketName:  s.storage.Bucket(),
		ObjectKey:   objectKey,
		ContentType: contentType,
		FileSize:    size,
	})
	if err != nil {
		if cleanupErr := s.storage.Delete(ctx, objectKey); cleanupErr != nil {
			return nil, fmt.Errorf("save photo metadata: %w (also could not remove orphaned object: %v)", err, cleanupErr)
		}
		return nil, err
	}
	return photo, nil
}

func (s *PhotoService) Delete(ctx context.Context, userID, photoID int64) error {
	if photoID <= 0 {
		return ErrInvalidPhotoID
	}
	photo, err := s.repository.GetByID(ctx, userID, photoID)
	if err != nil {
		return err
	}
	if err := s.storage.Delete(ctx, photo.ObjectKey); err != nil {
		return err
	}
	return s.repository.Delete(ctx, userID, photoID)
}

func photoExtension(contentType string) (string, bool) {
	switch contentType {
	case "image/jpeg":
		return ".jpg", true
	case "image/png":
		return ".png", true
	case "image/webp":
		return ".webp", true
	default:
		return "", false
	}
}

func randomPhotoKey() (string, error) {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return "", ErrCouldNotGeneratePhoto
	}
	return hex.EncodeToString(bytes), nil
}
