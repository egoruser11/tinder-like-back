package service

import (
	"context"
	"errors"
	"testing"

	"tinder-core/internal/models"
)

func TestChatServiceSend_NormalizesMessage(t *testing.T) {
	store := &fakeChatStore{}
	service := NewChatService(store)

	_, err := service.Send(context.Background(), 1, 10, "  hello  ")
	if err != nil {
		t.Fatalf("Send returned error: %v", err)
	}
	if store.body != "hello" || store.userID != 1 || store.chatID != 10 {
		t.Fatalf("unexpected create message input: %#v", store)
	}
}

func TestChatServiceSend_RejectsEmptyMessage(t *testing.T) {
	service := NewChatService(&fakeChatStore{})
	_, err := service.Send(context.Background(), 1, 10, "  ")
	if !errors.Is(err, ErrInvalidMessageBody) {
		t.Fatalf("error = %v, want ErrInvalidMessageBody", err)
	}
}

func TestChatServiceList_Paginates(t *testing.T) {
	store := &fakeChatStore{chats: []models.ChatPreview{{Chat: models.Chat{ID: 1}}, {Chat: models.Chat{ID: 2}}}}
	service := NewChatService(store)

	page, err := service.List(context.Background(), 1, ChatPageInput{Limit: 1})
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if len(page.Items) != 1 || page.Items[0].ID != 1 || page.NextCursor != "1" {
		t.Fatalf("unexpected page: %#v", page)
	}
}

type fakeChatStore struct {
	chats  []models.ChatPreview
	userID int64
	chatID int64
	body   string
}

func (s *fakeChatStore) ListActive(context.Context, int64, int64, int) ([]models.ChatPreview, error) {
	return s.chats, nil
}

func (s *fakeChatStore) ListMessages(context.Context, int64, int64, int64, int) ([]models.Message, error) {
	return nil, nil
}

func (s *fakeChatStore) CreateMessage(_ context.Context, userID, chatID int64, body string) (*models.Message, error) {
	s.userID, s.chatID, s.body = userID, chatID, body
	return &models.Message{ID: 1, ChatID: chatID, SenderID: userID, Body: body}, nil
}

func (s *fakeChatStore) MarkRead(context.Context, int64, int64) error { return nil }
