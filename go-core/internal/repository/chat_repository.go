package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"tinder-core/internal/models"
)

var ErrChatNotFound = errors.New("chat not found")

type ChatRepository struct {
	db *pgxpool.Pool
}

func NewChatRepository(db *pgxpool.Pool) *ChatRepository {
	return &ChatRepository{db: db}
}

func (repo *ChatRepository) ListActive(ctx context.Context, userID, afterChatID int64, limit int) ([]models.ChatPreview, error) {
	rows, err := repo.db.Query(ctx, `
		SELECT
			c.id, c.user_1_id, c.user_2_id, c.is_active, c.created_at, c.deactivated_at,
			p.user_id, p.full_name
		FROM chats c
		JOIN profiles p ON p.user_id = CASE WHEN c.user_1_id = $1 THEN c.user_2_id ELSE c.user_1_id END
		WHERE (c.user_1_id = $1 OR c.user_2_id = $1)
		  AND c.is_active = TRUE
		  AND c.id > $2
		ORDER BY c.id ASC
		LIMIT $3
	`, userID, afterChatID, limit)
	if err != nil {
		return nil, fmt.Errorf("list active chats: %w", err)
	}
	defer rows.Close()

	chats := make([]models.ChatPreview, 0, limit)
	for rows.Next() {
		var chat models.ChatPreview
		if err := rows.Scan(
			&chat.ID,
			&chat.User1ID,
			&chat.User2ID,
			&chat.IsActive,
			&chat.CreatedAt,
			&chat.DeactivatedAt,
			&chat.PartnerUserID,
			&chat.PartnerFullName,
		); err != nil {
			return nil, fmt.Errorf("scan chat preview: %w", err)
		}
		chats = append(chats, chat)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate chats: %w", err)
	}
	return chats, nil
}

func (repo *ChatRepository) ListMessages(ctx context.Context, userID, chatID, afterMessageID int64, limit int) ([]models.Message, error) {
	if err := repo.ensureParticipant(ctx, userID, chatID); err != nil {
		return nil, err
	}
	rows, err := repo.db.Query(ctx, `
		SELECT id, chat_id, sender_id, body, is_read, created_at, updated_at, deleted_at
		FROM messages
		WHERE chat_id = $1
		  AND id > $2
		  AND deleted_at IS NULL
		ORDER BY id ASC
		LIMIT $3
	`, chatID, afterMessageID, limit)
	if err != nil {
		return nil, fmt.Errorf("list messages: %w", err)
	}
	defer rows.Close()

	messages := make([]models.Message, 0, limit)
	for rows.Next() {
		var message models.Message
		if err := rows.Scan(
			&message.ID,
			&message.ChatID,
			&message.SenderID,
			&message.Body,
			&message.IsRead,
			&message.CreatedAt,
			&message.UpdatedAt,
			&message.DeletedAt,
		); err != nil {
			return nil, fmt.Errorf("scan message: %w", err)
		}
		messages = append(messages, message)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate messages: %w", err)
	}
	return messages, nil
}

func (repo *ChatRepository) CreateMessage(ctx context.Context, userID, chatID int64, body string) (*models.Message, error) {
	var message models.Message
	err := repo.db.QueryRow(ctx, `
		INSERT INTO messages (chat_id, sender_id, body)
		SELECT $1, $2, $3
		WHERE EXISTS (
			SELECT 1
			FROM chats
			WHERE id = $1
			  AND is_active = TRUE
			  AND (user_1_id = $2 OR user_2_id = $2)
		)
		RETURNING id, chat_id, sender_id, body, is_read, created_at, updated_at, deleted_at
	`, chatID, userID, body).Scan(
		&message.ID,
		&message.ChatID,
		&message.SenderID,
		&message.Body,
		&message.IsRead,
		&message.CreatedAt,
		&message.UpdatedAt,
		&message.DeletedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrChatNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("create message: %w", err)
	}
	return &message, nil
}

func (repo *ChatRepository) MarkRead(ctx context.Context, userID, chatID int64) error {
	if err := repo.ensureParticipant(ctx, userID, chatID); err != nil {
		return err
	}
	_, err := repo.db.Exec(ctx, `
		UPDATE messages
		SET is_read = TRUE, updated_at = now()
		WHERE chat_id = $1
		  AND sender_id <> $2
		  AND is_read = FALSE
		  AND deleted_at IS NULL
	`, chatID, userID)
	if err != nil {
		return fmt.Errorf("mark messages read: %w", err)
	}
	return nil
}

func (repo *ChatRepository) ensureParticipant(ctx context.Context, userID, chatID int64) error {
	var exists bool
	if err := repo.db.QueryRow(ctx, `
		SELECT EXISTS(
			SELECT 1 FROM chats
			WHERE id = $1 AND (user_1_id = $2 OR user_2_id = $2)
		)
	`, chatID, userID).Scan(&exists); err != nil {
		return fmt.Errorf("check chat participant: %w", err)
	}
	if !exists {
		return ErrChatNotFound
	}
	return nil
}
