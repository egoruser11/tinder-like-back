package service

import (
	"bytes"
	"context"
	"errors"
	"io"
	"testing"

	"tinder-core/internal/models"
)

func TestPhotoServiceUpload_SavesObjectThenMetadata(t *testing.T) {
	repository := &fakePhotoStore{}
	storage := &fakeObjectStore{}
	service := NewPhotoService(repository, storage)
	service.newKey = func() (string, error) { return "photo-key", nil }

	photo, err := service.Upload(context.Background(), 7, "image/jpeg", 4, bytes.NewBufferString("jpeg"))
	if err != nil {
		t.Fatalf("Upload returned error: %v", err)
	}
	if storage.putKey != "users/7/photo-key.jpg" || repository.created.ObjectKey != storage.putKey {
		t.Fatalf("unexpected photo key: storage=%q repository=%q", storage.putKey, repository.created.ObjectKey)
	}
	if photo == nil || photo.UserID != 7 {
		t.Fatalf("unexpected saved photo: %#v", photo)
	}
}

func TestPhotoServiceUpload_CleansObjectWhenMetadataFails(t *testing.T) {
	repository := &fakePhotoStore{createErr: errors.New("database unavailable")}
	storage := &fakeObjectStore{}
	service := NewPhotoService(repository, storage)
	service.newKey = func() (string, error) { return "photo-key", nil }

	_, err := service.Upload(context.Background(), 7, "image/png", 4, bytes.NewBufferString("png"))
	if err == nil {
		t.Fatal("Upload error = nil, want error")
	}
	if storage.deletedKey != "users/7/photo-key.png" {
		t.Fatalf("orphaned object was not cleaned up: %q", storage.deletedKey)
	}
}

func TestPhotoServiceUpload_RejectsUnsupportedContentType(t *testing.T) {
	service := NewPhotoService(&fakePhotoStore{}, &fakeObjectStore{})
	_, err := service.Upload(context.Background(), 7, "application/pdf", 4, bytes.NewBufferString("pdf"))
	if !errors.Is(err, ErrInvalidPhoto) {
		t.Fatalf("error = %v, want ErrInvalidPhoto", err)
	}
}

type fakePhotoStore struct {
	created   models.UserPhoto
	createErr error
}

func (s *fakePhotoStore) ListByUserID(context.Context, int64) ([]models.UserPhoto, error) {
	return nil, nil
}

func (s *fakePhotoStore) Create(_ context.Context, photo models.UserPhoto) (*models.UserPhoto, error) {
	s.created = photo
	if s.createErr != nil {
		return nil, s.createErr
	}
	return &photo, nil
}

func (s *fakePhotoStore) GetByID(context.Context, int64, int64) (*models.UserPhoto, error) {
	return nil, nil
}

func (s *fakePhotoStore) Delete(context.Context, int64, int64) error { return nil }

type fakeObjectStore struct {
	putKey     string
	deletedKey string
}

func (s *fakeObjectStore) Bucket() string { return "tinder-photos" }

func (s *fakeObjectStore) Put(_ context.Context, objectKey, _ string, _ int64, _ io.Reader) error {
	s.putKey = objectKey
	return nil
}

func (s *fakeObjectStore) Delete(_ context.Context, objectKey string) error {
	s.deletedKey = objectKey
	return nil
}
