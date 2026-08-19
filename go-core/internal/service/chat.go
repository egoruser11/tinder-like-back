package service

import (
	"context"
	"errors"
	"strconv"
	"strings"

	"tinder-core/internal/models"
)

var (
	ErrInvalidChatID      = errors.New("invalid chat id")
	ErrInvalidMessageBody = errors.New("message body must not be empty or exceed 4000 characters")
	ErrInvalidChatCursor  = errors.New("invalid chat cursor")
)

const (
	defaultChatPageLimit = 20
	maximumChatPageLimit = 50
)

type chatStore interface {
	ListActive(context.Context, int64, int64, int) ([]models.ChatPreview, error)
	ListMessages(context.Context, int64, int64, int64, int) ([]models.Message, error)
	CreateMessage(context.Context, int64, int64, string) (*models.Message, error)
	MarkRead(context.Context, int64, int64) error
}

type ChatService struct {
	repository chatStore
}

func NewChatService(repository chatStore) *ChatService {
	return &ChatService{repository: repository}
}

type ChatPageInput struct {
	Limit  int
	Cursor string
}

type ChatPage[T any] struct {
	Items      []T    `json:"items"`
	NextCursor string `json:"next_cursor,omitempty"`
}

func (s *ChatService) List(ctx context.Context, userID int64, input ChatPageInput) (ChatPage[models.ChatPreview], error) {
	limit, cursor, err := normalizeChatPage(input)
	if err != nil {
		return ChatPage[models.ChatPreview]{}, err
	}
	chats, err := s.repository.ListActive(ctx, userID, cursor, limit+1)
	if err != nil {
		return ChatPage[models.ChatPreview]{}, err
	}
	return makeChatPage(chats, limit, func(chat models.ChatPreview) int64 { return chat.ID }), nil
}

func (s *ChatService) Messages(ctx context.Context, userID, chatID int64, input ChatPageInput) (ChatPage[models.Message], error) {
	if chatID <= 0 {
		return ChatPage[models.Message]{}, ErrInvalidChatID
	}
	limit, cursor, err := normalizeChatPage(input)
	if err != nil {
		return ChatPage[models.Message]{}, err
	}
	messages, err := s.repository.ListMessages(ctx, userID, chatID, cursor, limit+1)
	if err != nil {
		return ChatPage[models.Message]{}, err
	}
	return makeChatPage(messages, limit, func(message models.Message) int64 { return message.ID }), nil
}

func (s *ChatService) Send(ctx context.Context, userID, chatID int64, body string) (*models.Message, error) {
	if chatID <= 0 {
		return nil, ErrInvalidChatID
	}
	body = strings.TrimSpace(body)
	if body == "" || len(body) > 4000 {
		return nil, ErrInvalidMessageBody
	}
	return s.repository.CreateMessage(ctx, userID, chatID, body)
}

func (s *ChatService) MarkRead(ctx context.Context, userID, chatID int64) error {
	if chatID <= 0 {
		return ErrInvalidChatID
	}
	return s.repository.MarkRead(ctx, userID, chatID)
}

func normalizeChatPage(input ChatPageInput) (int, int64, error) {
	limit := input.Limit
	if limit == 0 {
		limit = defaultChatPageLimit
	}
	if limit < 1 || limit > maximumChatPageLimit {
		return 0, 0, ErrInvalidChatCursor
	}
	if strings.TrimSpace(input.Cursor) == "" {
		return limit, 0, nil
	}
	cursor, err := strconv.ParseInt(input.Cursor, 10, 64)
	if err != nil || cursor < 0 {
		return 0, 0, ErrInvalidChatCursor
	}
	return limit, cursor, nil
}

func makeChatPage[T any](values []T, limit int, getID func(T) int64) ChatPage[T] {
	page := ChatPage[T]{Items: make([]T, 0, min(limit, len(values)))}
	for index, value := range values {
		if index == limit {
			page.NextCursor = strconv.FormatInt(getID(page.Items[len(page.Items)-1]), 10)
			break
		}
		page.Items = append(page.Items, value)
	}
	return page
}
